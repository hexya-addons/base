// Copyright 2016 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/tools/strutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	"github.com/spf13/viper"
)

var fields_Attachment = map[string]models.FieldDefinition{
	"Name":        fields.Char{String: "Attachment Name", Required: true},
	"DatasFname":  fields.Char{String: "File Name"},
	"Description": fields.Text{},
	"ResName": fields.Char{String: "Resource Name",
		Compute: h.Attachment().Methods().ComputeResName(), Stored: true, Depends: []string{"ResModel", "ResID"}},
	"ResModel": fields.Char{String: "Resource Model", Help: "The database object this attachment will be attached to",
		Index: true},
	"ResField": fields.Char{String: "Resource Field", Index: true},
	"ResID":    fields.Integer{String: "Resource ID", Help: "The record id this is attached to"},
	"Company": fields.Many2One{RelationModel: h.Company(), Default: func(env models.Environment) interface{} {
		return h.User().NewSet(env).CurrentUser().Company()
	}},
	"Type": fields.Selection{Selection: types.Selection{"binary": "Binary", "url": "URL"},
		Help: "You can either upload a file from your computer or copy/paste an internet link to your file."},
	"URL":    fields.Char{Index: true, Size: 1024},
	"Public": fields.Boolean{String: "Is a public document"},

	"Datas": fields.Binary{String: "File Content", Compute: h.Attachment().Methods().ComputeDatas(),
		Inverse: h.Attachment().Methods().InverseDatas()},
	"DBDatas":      fields.Char{String: "Database Data"},
	"StoreFname":   fields.Char{String: "Stored Filename"},
	"FileSize":     fields.Integer{GoType: new(int)},
	"CheckSum":     fields.Char{String: "Checksum/SHA1", Size: 40, Index: true},
	"MimeType":     fields.Char{},
	"IndexContent": fields.Text{String: "Indexed Content"},
}

// ComputeResName computes the display name of the ressource this document is attached to.
func attachment_ComputeResName(rs m.AttachmentSet) m.AttachmentData {
	res := h.Attachment().NewData()
	if rs.ResModel() != "" && rs.ResID() != 0 {
		record := rs.Env().Pool(rs.ResModel()).Search(models.Registry.MustGet(rs.ResModel()).Field(models.ID).Equals(rs.ResID()))
		res.SetResName(record.Get(record.Model().FieldName("DisplayName")).(string))
	}
	return res
}

// Storage returns the configured storage mechanism for attachments (e.g. database, file, etc.)
func attachment_Storage(rs m.AttachmentSet) string {
	return h.ConfigParameter().NewSet(rs.Env()).GetParam("attachment.location", "file")
}

// FileStore returns the directory in which the attachment files are saved.
func attachment_FileStore(_ m.AttachmentSet) string {
	return filepath.Join(viper.GetString("DataDir"), "filestore")
}

// ForceStorage forces all attachments to be stored in the currently configured storage
func attachment_ForceStorage(rs m.AttachmentSet) bool {
	if !h.User().NewSet(rs.Env()).CurrentUser().IsAdmin() {
		log.Panic(rs.T("Only administrators can execute this action."))
	}
	var cond q.AttachmentCondition
	switch rs.Storage() {
	case "db":
		cond = q.Attachment().StoreFname().IsNotNull()
	case "file":
		cond = q.Attachment().DBDatas().IsNotNull()
	}
	for _, attach := range h.Attachment().Search(rs.Env(), cond).Records() {
		attach.SetDatas(attach.Datas())
	}
	return true
}

// FullPath returns the given relative path as a full sanitized path
func attachment_FullPath(rs m.AttachmentSet, path string) string {
	return filepath.Join(rs.FileStore(), path)
}

// GetPath returns the relative and full paths of the file with the given sha.
// This methods creates the directory if it does not exist.`,
func attachment_GetPath(rs m.AttachmentSet, sha string) (string, string) {
	fName := filepath.Join(sha[:2], sha)
	fullPath := rs.FullPath(fName)
	if os.MkdirAll(filepath.Dir(fullPath), 0755) != nil {
		log.Panic("Unable to create directory for file storage")
	}
	return fName, fullPath
}

// FileRead returns the base64 encoded content of the given fileName (relative path).
// If binSize is true, it returns the file size instead as a human readable string`,
func attachment_FileRead(rs m.AttachmentSet, fileName string, binSize bool) string {
	fullPath := rs.FullPath(fileName)
	if binSize {
		fInfo, err := os.Stat(fullPath)
		if err != nil {
			log.Warn("Error while stating file", "file", fullPath, "error", err)
			return ""
		}
		return strutils.HumanSize(fInfo.Size())
	}
	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		log.Warn("Unable to read file", "file", fullPath, "error", err)
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

// FileWrite writes value into the file given by sha. If the file already exists, nothing is done.
//
// It returns the filename of the written file.`,
func attachment_FileWrite(rs m.AttachmentSet, value, sha string) string {
	fName, fullPath := rs.GetPath(sha)
	_, err := os.Stat(fullPath)
	if err == nil {
		// File already exists
		return fName
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		log.Warn("Unable to decode file content", "file", sha, "error", err)
	}
	ioutil.WriteFile(fullPath, data, 0644)
	// add fname to checklist, in case the transaction aborts
	rs.MarkForGC(fName)
	return fName
}

// FileDelete adds the given file name to the checklist for the garbage collector
func attachment_FileDelete(rs m.AttachmentSet, fName string) {
	rs.MarkForGC(fName)
}

// MarkForGC adds fName in a checklist for filestore garbage collection.
func attachment_MarkForGC(rs m.AttachmentSet, fName string) {
	// we use a spooldir: add an empty file in the subdirectory 'checklist'
	fullPath := filepath.Join(rs.FullPath("checklist"), fName)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	ioutil.WriteFile(fullPath, []byte{}, 0644)
}

// FileGC performs the garbage collection of the filestore.`,
func attachment_FileGC(rs m.AttachmentSet) {
	if rs.Storage() != "file" {
		return
	}
	// Continue in a new transaction. The LOCK statement below must be the
	// first one in the current transaction, otherwise the database snapshot
	// used by it may not contain the most recent changes made to the table
	// ir_attachment! Indeed, if concurrent transactions create attachments,
	// the LOCK statement will wait until those concurrent transactions end.
	// But this transaction will not see the new attachements if it has done
	// other requests before the LOCK (like the method _storage() above).
	models.ExecuteInNewEnvironment(rs.Env().Uid(), func(env models.Environment) {
		env.Cr().Execute("LOCK ir_attachment IN SHARE MODE")

		rSet := h.Attachment().NewSet(env)

		// retrieve the file names from the checklist
		var checklist []string
		err := filepath.Walk(rSet.FullPath("checklist"), func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			fName := filepath.Join(filepath.Base(filepath.Dir(path)), info.Name())
			checklist = append(checklist, fName)
			return nil
		})
		if err != nil {
			log.Panic("Error while walking the checklist directory", "error", err)
		}

		// determine which files to keep among the checklist
		var whitelistSlice []string
		env.Cr().Select(&whitelistSlice, "SELECT DISTINCT store_fname FROM ir_attachment WHERE store_fname IN ?", checklist)
		whitelist := make(map[string]bool)
		for _, wl := range whitelistSlice {
			whitelist[wl] = true
		}

		// remove garbage files, and clean up checklist
		var removed int
		for _, fName := range checklist {
			if !whitelist[fName] {
				err = os.Remove(rSet.FullPath(fName))
				if err != nil {
					log.Warn("Unable to FileGC", "file", rSet.FullPath(fName), "error", err)
					continue
				}
				removed++
			}
			err = os.Remove(filepath.Join(rSet.FullPath("checklist"), fName))
			if err != nil {
				log.Warn("Unable to clean checklist dir", "file", fName, "error", err)
			}
		}

		log.Info("Filestore garbage collected", "checked", len(checklist), "removed", removed)
	})

}

// ComputeDatas returns the data of the attachment, reading either from file or database
func attachment_ComputeDatas(rs m.AttachmentSet) m.AttachmentData {
	var datas string
	binSize := rs.Env().Context().GetBool("bin_size")
	if rs.StoreFname() != "" {
		datas = rs.FileRead(rs.StoreFname(), binSize)
	} else {
		datas = rs.DBDatas()
	}
	return h.Attachment().NewData().SetDatas(datas)
}

// InverseDatas stores the given data either in database or in file.
func attachment_InverseDatas(rs m.AttachmentSet, val string) {
	var binData string
	if val != "" {
		binBytes, err := base64.StdEncoding.DecodeString(val)
		if err != nil {
			log.Panic("Unable to decode attachment content", "error", err)
		}
		binData = string(binBytes)
	}
	vals := h.Attachment().NewData().
		SetFileSize(len(binData)).
		SetCheckSum(rs.ComputeCheckSum(binData)).
		SetIndexContent(rs.Index(binData, rs.MimeType())).
		SetDBDatas(val)
	if val != "" && rs.Storage() != "db" {
		// Save the file to the filestore
		vals.SetStoreFname(rs.FileWrite(val, vals.CheckSum()))
		vals.SetDBDatas("")
	}
	// take current location in filestore to possibly garbage-collect it
	fName := rs.StoreFname()
	// write as superuser, as user probably does not have write access
	rs.Sudo().WithContext("attachment_set_datas", true).Write(vals)
	if fName != "" {
		rs.FileDelete(fName)
	}
}

// ComputeCheckSum computes the SHA1 checksum of the given data
func attachment_ComputeCheckSum(_ m.AttachmentSet, data string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(data)))
}

// ComputeMimeType of the given values
func attachment_ComputeMimeType(_ m.AttachmentSet, values m.AttachmentData) string {
	mimeType := values.MimeType()
	if mimeType == "" && values.Datas() != "" {
		mimeType = http.DetectContentType([]byte(values.Datas()))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return mimeType
}

// CheckContents updates the given values
func attachment_CheckContents(rs m.AttachmentSet, values m.AttachmentData) m.AttachmentData {
	res := values
	res.SetMimeType(rs.ComputeMimeType(values))
	if strings.Contains(res.MimeType(), "ht") || strings.Contains(res.MimeType(), "xml") &&
		(!h.User().NewSet(rs.Env()).CurrentUser().IsAdmin() ||
			rs.Env().Context().GetBool("attachments_mime_plainxml")) {
		res.SetMimeType("text/plain")
	}
	return res
}

// Index computes the index content of the given filename, or binary data.
func attachment_Index(_ m.AttachmentSet, binData, fileType string) string {
	if fileType == "" {
		return ""
	}
	if strings.Split(fileType, "/")[0] != "text" {
		return ""
	}
	re := regexp.MustCompile(`[^\x00-\x1F\x7F-\xFF]{4,}`)
	words := re.FindAllString(binData, -1)
	return strings.Join(words, "\n")
}

// Check restricts the access to an ir.attachment, according to referred model
// In the 'document' module, it is overridden to relax this hard rule, since
// more complex ones apply there.
//
// This method panics if the user does not have the access rights.
func attachment_Check(rs m.AttachmentSet, mode string, values m.AttachmentData) {
	// collect the records to check (by model)
	var requireEmployee bool
	modelIds := make(map[string][]int64)
	if !rs.IsEmpty() {
		var attachs []struct {
			ResModel  sql.NullString `db:"res_model"`
			ResID     sql.NullInt64  `db:"res_id"`
			CreateUID sql.NullInt64  `db:"create_uid"`
			Public    sql.NullBool
		}
		rs.Env().Cr().Select(&attachs, "SELECT res_model, res_id, create_uid, public FROM attachment WHERE id IN (?)", rs.Ids())
		for _, attach := range attachs {
			if attach.Public.Bool && mode == "read" {
				continue
			}
			if attach.ResModel.String == "" || attach.ResID.Int64 == 0 {
				if attach.CreateUID.Int64 != rs.Env().Uid() {
					requireEmployee = true
				}
				continue
			}
			modelIds[attach.ResModel.String] = append(modelIds[attach.ResModel.String], attach.ResID.Int64)
		}
	}
	if values != nil && values.ResModel() != "" && values.ResID() != 0 {
		modelIds[values.ResModel()] = append(modelIds[values.ResModel()], values.ResID())
	}

	// check access rights on the records
	for resModel, resIds := range modelIds {
		// ignore attachments that are not attached to a resource anymore
		// when checking access rights (resource was deleted but attachment
		// was not)
		if _, exists := models.Registry.Get(resModel); !exists {
			requireEmployee = true
			continue
		}
		rModel := models.Registry.MustGet(resModel)
		records := rs.Env().Pool(resModel).Search(rModel.Field(models.ID).In(resIds))
		if records.Len() < len(resIds) {
			requireEmployee = true
		}
		// For related models, check if we can write to the model, as unlinking
		// and creating attachments can be seen as an update to the model
		switch mode {
		case "create", "write", "unlink":
			records.CheckExecutionPermission(rModel.Methods().MustGet("Write").Underlying())
		case "read":
			records.CheckExecutionPermission(rModel.Methods().MustGet("Load").Underlying())
		}
	}
	if requireEmployee {
		currentUser := h.User().NewSet(rs.Env()).CurrentUser()
		if !currentUser.IsAdmin() && !currentUser.HasGroup(GroupUser.ID) {
			log.Panic(rs.T("Sorry, you are not allowed to access this document."))
		}
	}
}

func attachment_Search(rs m.AttachmentSet, cond q.AttachmentCondition) m.AttachmentSet {
	// add res_field=False in domain if not present
	hasResField := cond.HasField(h.Attachment().Fields().ResField())
	if !hasResField {
		cond = cond.And().ResField().IsNull()
	}
	if rs.Env().Uid() == security.SuperUserID {
		return rs.Super().Search(cond)
	}
	// For attachments, the permissions of the document they are attached to
	// apply, so we must remove attachments for which the user cannot access
	// the linked document.
	modelAttachments := make(map[models.RecordRef][]int64)
	rs.Load(
		h.Attachment().Fields().ID(),
		h.Attachment().Fields().ResModel(),
		h.Attachment().Fields().ResID(),
		h.Attachment().Fields().Public())
	for _, attach := range rs.Records() {
		if attach.ResModel() == "" || attach.Public() {
			continue
		}
		rRef := models.RecordRef{
			ModelName: attach.ResModel(),
			ID:        attach.ResID(),
		}
		modelAttachments[rRef] = append(modelAttachments[rRef], attach.ID())
	}
	// To avoid multiple queries for each attachment found, checks are
	// performed in batch as much as possible.
	var allowedIds []int64
	for rRef, targets := range modelAttachments {
		if _, exists := models.Registry.Get(rRef.ModelName); !exists {
			continue
		}
		rModel := models.Registry.MustGet(rRef.ModelName)
		if !rs.Env().Pool(rRef.ModelName).CheckExecutionPermission(rModel.Methods().MustGet("Load").Underlying(), true) {
			continue
		}
		allowed := rs.Env().Pool(rRef.ModelName).Search(rModel.Field(models.ID).In(targets))
		allowedIds = append(allowedIds, allowed.Ids()...)
	}
	return h.Attachment().Browse(rs.Env(), allowedIds)
}

func attachment_Load(rs m.AttachmentSet, fields ...models.FieldName) m.AttachmentSet {
	rs.Check("read", nil)
	return rs.Super().Load(fields...)
}

func attachment_Write(rs m.AttachmentSet, vals m.AttachmentData) bool {
	if rs.Env().Context().GetBool("attachment_set_datas") {
		return rs.Super().Write(vals)
	}
	rs.Check("write", vals)
	if vals.HasMimeType() || vals.HasDatas() {
		vals = rs.CheckContents(vals)
	}
	return rs.Super().Write(vals)
}

func attachment_Copy(rs m.AttachmentSet, overrides m.AttachmentData) m.AttachmentSet {
	rs.Check("write", nil)
	return rs.Super().Copy(overrides)
}

func attachment_Unlink(rs m.AttachmentSet) int64 {
	rs.Check("unlink", nil)
	return rs.Super().Unlink()
}

func attachment_Create(rs m.AttachmentSet, vals m.AttachmentData) m.AttachmentSet {
	vals = rs.CheckContents(vals)
	rs.Check("write", vals)
	return rs.Super().Create(vals)
}

// ActionGet returns the action for displaying attachments
func attachment_ActionGet(_ m.AttachmentSet) *actions.Action {
	return actions.Registry.GetByXMLId("base_action_attachment")
}

func init() {
	models.NewModel("Attachment")
	h.Attachment().AddFields(fields_Attachment)

	h.Attachment().NewMethod("ComputeResName", attachment_ComputeResName)
	h.Attachment().NewMethod("Storage", attachment_Storage)
	h.Attachment().NewMethod("FileStore", attachment_FileStore)
	h.Attachment().NewMethod("ForceStorage", attachment_ForceStorage)
	h.Attachment().NewMethod("FullPath", attachment_FullPath)
	h.Attachment().NewMethod("GetPath", attachment_GetPath)
	h.Attachment().NewMethod("FileRead", attachment_FileRead)
	h.Attachment().NewMethod("FileWrite", attachment_FileWrite)
	h.Attachment().NewMethod("FileDelete", attachment_FileDelete)
	h.Attachment().NewMethod("MarkForGC", attachment_MarkForGC)
	h.Attachment().NewMethod("FileGC", attachment_FileGC)
	h.Attachment().NewMethod("ComputeDatas", attachment_ComputeDatas)
	h.Attachment().NewMethod("InverseDatas", attachment_InverseDatas)
	h.Attachment().NewMethod("ComputeCheckSum", attachment_ComputeCheckSum)
	h.Attachment().NewMethod("ComputeMimeType", attachment_ComputeMimeType)
	h.Attachment().NewMethod("CheckContents", attachment_CheckContents)
	h.Attachment().NewMethod("Index", attachment_Index)
	h.Attachment().NewMethod("Check", attachment_Check)
	h.Attachment().Methods().Search().Extend(attachment_Search)
	h.Attachment().Methods().Load().Extend(attachment_Load)
	h.Attachment().Methods().Write().Extend(attachment_Write)
	h.Attachment().Methods().Copy().Extend(attachment_Copy)
	h.Attachment().Methods().Unlink().Extend(attachment_Unlink)
	h.Attachment().Methods().Create().Extend(attachment_Create)
	h.Attachment().NewMethod("ActionGet", attachment_ActionGet)
}
