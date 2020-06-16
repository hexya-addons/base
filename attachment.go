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

	"github.com/google/uuid"
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/tools/strutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	"github.com/spf13/viper"
)

var fields_Attachment = map[string]models.FieldDefinition{
	"Name":        fields.Char{String: "Attachment Name", Required: true},
	"Description": fields.Text{},
	"ResName": fields.Char{String: "Resource Name",
		Compute: h.Attachment().Methods().ComputeResName(), Stored: true, Depends: []string{"ResModel", "ResID"}},
	"ResModel": fields.Char{String: "Resource Model", Help: "The database object this attachment will be attached to",
		Index: true, ReadOnly: true},
	"ResField": fields.Char{String: "Resource Field", Index: true, ReadOnly: true},
	"ResID": fields.Integer{String: "Resource ID", Help: "The record id this is attached to", Index: true,
		ReadOnly: true},
	"Company": fields.Many2One{RelationModel: h.Company(), Default: func(env models.Environment) interface{} {
		return h.User().NewSet(env).CurrentUser().Company()
	}},
	"Type": fields.Selection{Selection: types.Selection{"binary": "File", "url": "URL"},
		Help: "You can either upload a file from your computer or copy/paste an internet link to your file."},
	"URL":    fields.Char{Index: true, Size: 1024},
	"Public": fields.Boolean{String: "Is a public document"},

	"AccessToken": fields.Char{},

	"Datas": fields.Binary{String: "File Content", Compute: h.Attachment().Methods().ComputeDatas(),
		Inverse: h.Attachment().Methods().InverseDatas(), Depends: []string{"StoreFname", "DBDatas"}},
	"DBDatas":      fields.Char{String: "Database Data"},
	"StoreFname":   fields.Char{String: "Stored Filename"},
	"FileSize":     fields.Integer{GoType: new(int)},
	"CheckSum":     fields.Char{String: "Checksum/SHA1", Size: 40, Index: true, ReadOnly: true},
	"MimeType":     fields.Char{ReadOnly: true},
	"IndexContent": fields.Text{String: "Indexed Content", ReadOnly: true},
}

// ComputeResName computes the display name of the ressource this document is attached to.
func attachment_ComputeResName(rs m.AttachmentSet) m.AttachmentData {
	res := h.Attachment().NewData().SetResName("")
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
	rSet := h.Attachment().NewSet(rs.Env())

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
	rs.Env().Cr().Select(&whitelistSlice, "SELECT DISTINCT store_fname FROM ir_attachment WHERE store_fname IN ?", checklist)
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
	vals := rs.GetDatasRelatedValues(val, rs.MimeType())
	// take current location in filestore to possibly garbage-collect it
	fName := rs.StoreFname()
	// write as superuser, as user probably does not have write access
	rs.Sudo().WithContext("attachment_set_datas", true).Write(vals)
	if fName != "" {
		rs.FileDelete(fName)
	}
}

// GetDatasRelatedValues compute the fields that depend on data
func attachment_GetDatasRelatedValues(rs m.AttachmentSet, data string, mimeType string) m.AttachmentData {
	var binData string
	if data != "" {
		binBytes, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			log.Panic("Unable to decode attachment content", "error", err)
		}
		binData = string(binBytes)
	}
	values := h.Attachment().NewData().
		SetFileSize(len(binData)).
		SetCheckSum(rs.ComputeCheckSum(binData)).
		SetIndexContent(rs.Index(binData, mimeType)).
		SetDBDatas(data)
	if data != "" && rs.Storage() != "db" {
		// Save the file to the filestore
		values.SetStoreFname(rs.FileWrite(data, values.CheckSum()))
		values.SetDBDatas("")
	}
	return values
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
	user := h.User().NewSet(rs.Env()).CurrentUser()
	if rs.Env().Context().HasKey("binary_field_real_user") {
		user = h.User().BrowseOne(rs.Env(), rs.Env().Context().GetInteger("binary_field_real_user"))
	}
	xmlLike := strings.Contains(res.MimeType(), "ht") || strings.Contains(res.MimeType(), "xml")
	if xmlLike && (!user.IsSystem() || rs.Env().Context().GetBool("attachments_mime_plainxml")) {
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

// GetServingGroups returns groups allowed tp create and write serving attachments.
//
// An attachment record may be used as a fallback in the
// http dispatch if its type field is set to "binary" and its url
// field is set as the request's url. Only the groups returned by
// this method are allowed to create and write on such records.
func attachment_GetServingGroups(rs m.AttachmentSet) m.GroupSet {
	return h.Group().Search(rs.Env(), q.Group().GroupID().Equals(GroupSystem.ID()))
}

// CheckServingAttachment limits creation and modification of served attachments
// to the members of the serving groups.
func attachment_CheckServingAttachments(rs m.AttachmentSet) {
	currentUser := h.User().NewSet(rs.Env()).CurrentUser()
	if currentUser.IsAdmin() {
		return
	}
attachmentLoop:
	for _, attachment := range rs.Records() {
		// restrict writing on attachments that could be served by the
		// ir.http's dispatch exception handling.
		if attachment.Type() != "binary" || attachment.URL() == "" {
			continue
		}
		for _, group := range attachment.GetServingGroups().Records() {
			if currentUser.HasGroup(group.GroupID()) {
				continue attachmentLoop
			}
		}
		panic(rs.T("Sorry, you are not allowed to write on this document"))
	}
}

// Check restricts the access to an ir.attachment, according to referred model
// In the 'document' module, it is overridden to relax this hard rule, since
// more complex ones apply there.
//
// This method panics if the user does not have the access rights.
func attachment_Check(rs m.AttachmentSet, mode string, values m.AttachmentData) {
	currentUser := h.User().NewSet(rs.Env()).CurrentUser()
	if currentUser.IsSuperUser() {
		return
	}
	// collect the records to check (by model)
	var requireEmployee bool
	modelIds := make(map[string][]int64)
	if rs.IsNotEmpty() {
		var attachs []struct {
			ResModel  sql.NullString `db:"res_model"`
			ResID     sql.NullInt64  `db:"res_id"`
			CreateUID sql.NullInt64  `db:"create_uid"`
			Public    sql.NullBool
			ResField  sql.NullString `db:"res_field"`
		}
		rs.Env().Cr().Select(&attachs, "SELECT res_model, res_id, create_uid, public, res_field FROM attachment WHERE id IN (?)", rs.Ids())
		for _, attach := range attachs {
			if attach.ResField.Valid && !currentUser.IsSystem() {
				panic(rs.T("Sorry, you are not allowed to access this document."))
			}
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
		if !currentUser.IsAdmin() && !currentUser.HasGroup(GroupUser.ID()) {
			log.Panic(rs.T("Sorry, you are not allowed to access this document."))
		}
	}
}

// ReadGroupAllowedFields returns the fields by which a non-admin user is allowed to group by
func attachment_ReadGroupAllowedFields(_ m.AttachmentSet) models.FieldNames {
	return models.FieldNames{
		h.Attachment().Fields().Type(),
		h.Attachment().Fields().Company(),
		h.Attachment().Fields().ResID(),
		h.Attachment().Fields().CreateDate(),
		h.Attachment().Fields().CreateUID(),
		h.Attachment().Fields().Name(),
		h.Attachment().Fields().MimeType(),
		h.Attachment().Fields().ID(),
		h.Attachment().Fields().URL(),
		h.Attachment().Fields().ResField(),
		h.Attachment().Fields().ResModel(),
	}
}

func attachment_Aggregates(rs m.AttachmentSet, fields ...models.FieldName) []m.AttachmentGroupAggregateRow {
	if len(fields) == 0 {
		panic(rs.T("Sorry, you must provide fields to read on attachments"))
	}
	for _, f := range fields {
		if strings.Contains(f.Name(), "(") || strings.Contains(f.JSON(), "(") {
			panic(rs.T("Sorry, the syntax 'name:agg(field)' is not available for attachments"))
		}
		var inAlloaedFields bool
		for _, af := range rs.ReadGroupAllowedFields() {
			if af.Name() == f.Name() {
				inAlloaedFields = true
				break
			}
		}
		if !inAlloaedFields && !h.User().NewSet(rs.Env()).CurrentUser().IsSystem() {
			panic(rs.T("Sorry, you are not allowed to access these fields on attachments."))
		}
	}
	return rs.Super().Aggregates(fields...)
}

func attachment_Search(rs m.AttachmentSet, cond q.AttachmentCondition) m.AttachmentSet {
	// add res_field=False in domain if not present
	hasResField := cond.HasField(h.Attachment().Fields().ResField())
	if !hasResField {
		cond = cond.And().ResField().IsNull()
	}
	if h.User().NewSet(rs.Env()).CurrentUser().IsSystem() {
		// rules do not apply for the superuser
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
	if !rs.Env().Context().GetBool("hexya_force_compute_write") {
		vals.UnsetFileSize()
		vals.UnsetCheckSum()
	}
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
	if rs.IsEmpty() {
		return 0
	}
	rs.Check("unlink", nil)
	// First delete in the database, *then* in the filesystem if the
	// database allowed it. Helps avoid errors when concurrent transactions
	// are deleting the same file, and some of the transactions are
	// rolled back by PostgreSQL (due to concurrent updates detection).
	toDelete := make(map[string]bool)
	for _, attach := range rs.Records() {
		toDelete[attach.StoreFname()] = true
	}
	res := rs.Super().Unlink()
	for filePath := range toDelete {
		rs.FileDelete(filePath)
	}
	return res
}

func attachment_Create(rs m.AttachmentSet, vals m.AttachmentData) m.AttachmentSet {
	vals = rs.CheckContents(vals)
	if !rs.Env().Context().GetBool("hexya_force_compute_write") {
		vals.UnsetFileSize()
		vals.UnsetCheckSum()
	}
	rs.Check("write", vals)
	return rs.Super().Create(vals)
}

// PostAddCreate is called after an attachment is uploaded.
// It can be overridden to implement specific behaviour after attachment creation.
func attachment_PostAddCreate(_ m.AttachmentSet) {
}

// GenerateAccessToken generates and store a random access token for these attachments
func attachment_GenerateAccessToken(rs m.AttachmentSet) []string {
	var tokens []string
	for _, attachment := range rs.Records() {
		if attachment.AccessToken() != "" {
			tokens = append(tokens, attachment.AccessToken())
			continue
		}
		accessToken := rs.GenerateToken()
		attachment.SetAccessToken(accessToken)
		tokens = append(tokens, accessToken)
	}
	return tokens
}

// GenerateToken generates and return a single random accessToken.
// Base implementation returns a UUID.
func attachment_GenerateToken(_ m.AttachmentSet) string {
	return uuid.New().String()
}

// ActionGet returns the action for displaying attachments
func attachment_ActionGet(_ m.AttachmentSet) *actions.Action {
	return actions.Registry.MustGetByXMLID("base_action_attachment")
}

// GetServeAttachment returns the serve attachments
func attachment_GetServeAttachment(rs m.AttachmentSet, url string, extraCond q.AttachmentCondition, extraFields models.FieldNames, orders []string) []models.RecordData {
	cond := q.Attachment().Type().Equals("binary").And().URL().Equals(url).AndCond(extraCond)
	fieldNames := append(extraFields, h.Attachment().Fields().LastUpdate(), h.Attachment().Fields().Datas(), h.Attachment().Fields().MimeType())
	attachments := h.Attachment().Search(rs.Env(), cond).OrderBy(orders...)
	return attachments.Read(fieldNames)
}

// GetAttachmentByKey returns the attachment with the given key
func attachment_GetAttachmentByKey(rs m.AttachmentSet, key string, extraCond q.AttachmentCondition, orders []string) m.AttachmentSet {
	cond := q.Attachment().HexyaExternalID().Equals(key).AndCond(extraCond)
	return h.Attachment().Search(rs.Env(), cond).OrderBy(orders...)
}

func init() {
	models.NewModel("Attachment")
	h.Attachment().SetDefaultOrder("ID desc")
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
	h.Attachment().NewMethod("GetDatasRelatedValues", attachment_GetDatasRelatedValues)
	h.Attachment().NewMethod("ComputeCheckSum", attachment_ComputeCheckSum)
	h.Attachment().NewMethod("ComputeMimeType", attachment_ComputeMimeType)
	h.Attachment().NewMethod("CheckContents", attachment_CheckContents)
	h.Attachment().NewMethod("Index", attachment_Index)
	h.Attachment().NewMethod("GetServingGroups", attachment_GetServingGroups)
	h.Attachment().NewMethod("CheckServingAttachments", attachment_CheckServingAttachments)
	h.Attachment().NewMethod("Check", attachment_Check)
	h.Attachment().NewMethod("ReadGroupAllowedFields", attachment_ReadGroupAllowedFields)
	h.Attachment().Methods().Aggregates().Extend(attachment_Aggregates)
	h.Attachment().Methods().Search().Extend(attachment_Search)
	h.Attachment().Methods().Load().Extend(attachment_Load)
	h.Attachment().Methods().Write().Extend(attachment_Write)
	h.Attachment().Methods().Copy().Extend(attachment_Copy)
	h.Attachment().Methods().Unlink().Extend(attachment_Unlink)
	h.Attachment().Methods().Create().Extend(attachment_Create)
	h.Attachment().NewMethod("PostAddCreate", attachment_PostAddCreate)
	h.Attachment().NewMethod("GenerateAccessToken", attachment_GenerateAccessToken)
	h.Attachment().NewMethod("GenerateToken", attachment_GenerateToken)
	h.Attachment().NewMethod("ActionGet", attachment_ActionGet)
	h.Attachment().NewMethod("GetServeAttachment", attachment_GetServeAttachment)
	h.Attachment().NewMethod("GetAttachmentByKey", attachment_GetAttachmentByKey)
}
