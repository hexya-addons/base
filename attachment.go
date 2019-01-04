// Copyright 2016 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"crypto/sha1"
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
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/tools/strutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	"github.com/spf13/viper"
)

func init() {
	attachmentModel := h.Attachment().DeclareModel()
	attachmentModel.AddFields(map[string]models.FieldDefinition{
		"Name":        models.CharField{String: "Attachment Name", Required: true},
		"DatasFname":  models.CharField{String: "File Name"},
		"Description": models.TextField{},
		"ResName": models.CharField{String: "Resource Name",
			Compute: h.Attachment().Methods().ComputeResName(), Stored: true, Depends: []string{"ResModel", "ResID"}},
		"ResModel": models.CharField{String: "Resource Model", Help: "The database object this attachment will be attached to",
			Index: true},
		"ResField": models.CharField{String: "Resource Field", Index: true},
		"ResID":    models.IntegerField{String: "Resource ID", Help: "The record id this is attached to"},
		"Company": models.Many2OneField{RelationModel: h.Company(), Default: func(env models.Environment) interface{} {
			return h.User().NewSet(env).CurrentUser().Company()
		}},
		"Type": models.SelectionField{Selection: types.Selection{"binary": "Binary", "url": "URL"},
			Help: "You can either upload a file from your computer or copy/paste an internet link to your file."},
		"URL":    models.CharField{Index: true, Size: 1024},
		"Public": models.BooleanField{String: "Is a public document"},

		"Datas": models.BinaryField{String: "File Content", Compute: h.Attachment().Methods().ComputeDatas(),
			Inverse: h.Attachment().Methods().InverseDatas()},
		"DBDatas":      models.CharField{String: "Database Data"},
		"StoreFname":   models.CharField{String: "Stored Filename"},
		"FileSize":     models.IntegerField{GoType: new(int)},
		"CheckSum":     models.CharField{String: "Checksum/SHA1", Size: 40, Index: true},
		"MimeType":     models.CharField{},
		"IndexContent": models.TextField{String: "Indexed Content"},
	})

	attachmentModel.Methods().ComputeResName().DeclareMethod(
		`ComputeResName computes the display name of the ressource this document is attached to.`,
		func(rs h.AttachmentSet) *h.AttachmentData {
			res := h.Attachment().NewData()
			if rs.ResModel() != "" && rs.ResID() != 0 {
				record := rs.Env().Pool(rs.ResModel()).Search(models.Registry.MustGet(rs.ResModel()).Field("ID").Equals(rs.ResID()))
				res.SetResName(record.Get("DisplayName").(string))
			}
			return res
		})

	attachmentModel.Methods().Storage().DeclareMethod(
		`Storage returns the configured storage mechanism for attachments (e.g. database, file, etc.)`,
		func(rs h.AttachmentSet) string {
			return h.ConfigParameter().NewSet(rs.Env()).GetParam("attachment.location", "file")
		})

	attachmentModel.Methods().FileStore().DeclareMethod(
		`FileStore returns the directory in which the attachment files are saved.`,
		func(rs h.AttachmentSet) string {
			return filepath.Join(viper.GetString("DataDir"), "filestore")
		})

	attachmentModel.Methods().ForceStorage().DeclareMethod(
		`ForceStorage forces all attachments to be stored in the currently configured storage`,
		func(rs h.AttachmentSet) bool {
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
		})

	attachmentModel.Methods().FullPath().DeclareMethod(
		`FullPath returns the given relative path as a full sanitized path`,
		func(rs h.AttachmentSet, path string) string {
			return filepath.Join(rs.FileStore(), path)
		})

	attachmentModel.Methods().GetPath().DeclareMethod(
		`GetPath returns the relative and full paths of the file with the given sha.
		This methods creates the directory if it does not exist.`,
		func(rs h.AttachmentSet, sha string) (string, string) {
			fName := filepath.Join(sha[:2], sha)
			fullPath := rs.FullPath(fName)
			if os.MkdirAll(filepath.Dir(fullPath), 0755) != nil {
				log.Panic("Unable to create directory for file storage")
			}
			return fName, fullPath
		})

	attachmentModel.Methods().FileRead().DeclareMethod(
		`FileRead returns the base64 encoded content of the given fileName (relative path).
		If binSize is true, it returns the file size instead as a human readable string`,
		func(rs h.AttachmentSet, fileName string, binSize bool) string {
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
		})

	attachmentModel.Methods().FileWrite().DeclareMethod(
		`FileWrite writes value into the file given by sha. If the file already exists, nothing is done.

		It returns the filename of the written file.`,
		func(rs h.AttachmentSet, value, sha string) string {
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
		})

	attachmentModel.Methods().FileDelete().DeclareMethod(
		`FileDelete adds the given file name to the checklist for the garbage collector`,
		func(rs h.AttachmentSet, fName string) {
			rs.MarkForGC(fName)
		})

	attachmentModel.Methods().MarkForGC().DeclareMethod(
		`MarkForGC adds fName in a checklist for filestore garbage collection.`,
		func(rs h.AttachmentSet, fName string) {
			// we use a spooldir: add an empty file in the subdirectory 'checklist'
			fullPath := filepath.Join(rs.FullPath("checklist"), fName)
			os.MkdirAll(filepath.Dir(fullPath), 0755)
			ioutil.WriteFile(fullPath, []byte{}, 0644)
		})

	attachmentModel.Methods().FileGC().DeclareMethod(
		`FileGC performs the garbage collection of the filestore.`,
		func(rs h.AttachmentSet) {
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

				//retrieve the file names from the checklist
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

		})

	attachmentModel.Methods().ComputeDatas().DeclareMethod(
		`ComputeDatas returns the data of the attachment, reading either from file or database`,
		func(rs h.AttachmentSet) *h.AttachmentData {
			var datas string
			binSize := rs.Env().Context().GetBool("bin_size")
			if rs.StoreFname() != "" {
				datas = rs.FileRead(rs.StoreFname(), binSize)
			} else {
				datas = rs.DBDatas()
			}
			return h.Attachment().NewData().SetDatas(datas)
		})

	attachmentModel.Methods().InverseDatas().DeclareMethod(
		`InverseDatas stores the given data either in database or in file.`,
		func(rs h.AttachmentSet, val string) {
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
		})

	attachmentModel.Methods().ComputeCheckSum().DeclareMethod(
		`ComputeCheckSum computes the SHA1 checksum of the given data`,
		func(rs h.AttachmentSet, data string) string {
			return fmt.Sprintf("%x", sha1.Sum([]byte(data)))
		})

	attachmentModel.Methods().ComputeMimeType().DeclareMethod(
		`ComputeMimeType of the given values`,
		func(rs h.AttachmentSet, values *h.AttachmentData) string {
			mimeType := values.MimeType()
			if mimeType == "" && values.Datas() != "" {
				mimeType = http.DetectContentType([]byte(values.Datas()))
			}
			if mimeType == "" {
				mimeType = "application/octet-stream"
			}
			return mimeType
		})

	attachmentModel.Methods().CheckContents().DeclareMethod(
		`CheckContents updates the given values`,
		func(rs h.AttachmentSet, values *h.AttachmentData) *h.AttachmentData {
			res := *values
			res.SetMimeType(rs.ComputeMimeType(values))
			if strings.Contains(res.MimeType(), "ht") || strings.Contains(res.MimeType(), "xml") &&
				(!h.User().NewSet(rs.Env()).CurrentUser().IsAdmin() ||
					rs.Env().Context().GetBool("attachments_mime_plainxml")) {
				res.SetMimeType("text/plain")
			}
			return &res
		})

	attachmentModel.Methods().Index().DeclareMethod(
		`Index computes the index content of the given filename, or binary data.`,
		func(rs h.AttachmentSet, binData, fileType string) string {
			if fileType == "" {
				return ""
			}
			if strings.Split(fileType, "/")[0] != "text" {
				return ""
			}
			re := regexp.MustCompile(`[^\x00-\x1F\x7F-\xFF]{4,}`)
			words := re.FindAllString(binData, -1)
			return strings.Join(words, "\n")
		})

	attachmentModel.Methods().Check().DeclareMethod(
		`Check restricts the access to an ir.attachment, according to referred model
        In the 'document' module, it is overridden to relax this hard rule, since
        more complex ones apply there.

		This method panics if the user does not have the access rights.`,
		func(rs h.AttachmentSet, mode string, values *h.AttachmentData) {
			// collect the records to check (by model)
			var requireEmployee bool
			modelIds := make(map[string][]int64)
			if !rs.IsEmpty() {
				var attachs []struct {
					ResModel  string
					ResID     int64
					CreateUID int64
					Public    bool
				}
				rs.Env().Cr().Select(&attachs, "SELECT res_model, res_id, create_uid, public FROM attachment WHERE id IN (?)", rs.Ids())
				for _, attach := range attachs {
					if attach.Public && mode == "read" {
						continue
					}
					if attach.ResModel == "" || attach.ResID == 0 {
						if attach.CreateUID != rs.Env().Uid() {
							requireEmployee = true
						}
						continue
					}
					modelIds[attach.ResModel] = append(modelIds[attach.ResModel], attach.ResID)
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
				records := rs.Env().Pool(resModel).Search(rModel.Field("ID").In(resIds))
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
		})

	attachmentModel.Methods().Search().Extend("",
		func(rs h.AttachmentSet, cond q.AttachmentCondition) h.AttachmentSet {
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
				h.Attachment().Fields().ID().String(),
				h.Attachment().Fields().ResModel().String(),
				h.Attachment().Fields().ResID().String(),
				h.Attachment().Fields().Public().String())
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
				allowed := rs.Env().Pool(rRef.ModelName).Search(rModel.Field("ID").In(targets))
				allowedIds = append(allowedIds, allowed.Ids()...)
			}
			return h.Attachment().Browse(rs.Env(), allowedIds)
		})

	attachmentModel.Methods().Load().Extend("",
		func(rs h.AttachmentSet, fields ...string) h.AttachmentSet {
			rs.Check("read", nil)
			return rs.Super().Load(fields...)
		})

	attachmentModel.Methods().Write().Extend("",
		func(rs h.AttachmentSet, vals *h.AttachmentData) bool {
			if rs.Env().Context().GetBool("attachment_set_datas") {
				return rs.Super().Write(vals)
			}
			rs.Check("write", vals)
			if vals.HasMimeType() || vals.HasDatas() {
				vals = rs.CheckContents(vals)
			}
			return rs.Super().Write(vals)
		})

	attachmentModel.Methods().Copy().Extend("",
		func(rs h.AttachmentSet, overrides *h.AttachmentData) h.AttachmentSet {
			rs.Check("write", nil)
			return rs.Super().Copy(overrides)
		})

	attachmentModel.Methods().Unlink().Extend("",
		func(rs h.AttachmentSet) int64 {
			rs.Check("unlink", nil)
			return rs.Super().Unlink()
		})

	attachmentModel.Methods().Create().Extend("",
		func(rs h.AttachmentSet, vals *h.AttachmentData) h.AttachmentSet {
			vals = rs.CheckContents(vals)
			rs.Check("write", vals)
			return rs.Super().Create(vals)
		})

	attachmentModel.Methods().ActionGet().DeclareMethod(
		`ActionGet returns the action for displaying attachments`,
		func(rs h.AttachmentSet) *actions.Action {
			return actions.Registry.GetById("base_action_attachment")
		})
}
