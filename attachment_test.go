// Copyright 2018 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/spf13/viper"
)

const HashSplit = 1

func TestAttachment(t *testing.T) {
	Convey("Testing Attachments", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			viper.Set("DataDir", os.TempDir())
			// Blob 1
			blob1 := "blob1"
			blob1B64 := base64.StdEncoding.EncodeToString([]byte(blob1))
			blob1Hash := sha1.Sum([]byte(blob1))
			blob1FName := fmt.Sprintf("%x/%x", blob1Hash[:HashSplit], blob1Hash)
			// Blob 2
			blob2 := "blob2"
			blob2B64 := base64.StdEncoding.EncodeToString([]byte(blob2))

			Convey("Storing in DB", func() {
				h.ConfigParameter().NewSet(env).SetParam("attachment.location", "db")
				a1 := h.Attachment().Create(env, h.Attachment().NewData().
					SetName("a1").
					SetDatas(blob1B64))
				So(a1.Datas(), ShouldEqual, blob1B64)
				So(a1.DBDatas(), ShouldEqual, blob1B64)
			})
			Convey("Storing on disk", func() {
				a2 := h.Attachment().Create(env, h.Attachment().NewData().
					SetName("a2").
					SetDatas(blob1B64))
				So(a2.StoreFname(), ShouldEqual, blob1FName)
				_, err := os.Stat(filepath.Join(a2.FileStore(), a2.StoreFname()))
				So(err, ShouldBeNil)
			})
			Convey("No Duplication", func() {
				a2 := h.Attachment().Create(env, h.Attachment().NewData().
					SetName("a2").
					SetDatas(blob1B64))
				a3 := h.Attachment().Create(env, h.Attachment().NewData().
					SetName("a2").
					SetDatas(blob1B64))
				So(a2.StoreFname(), ShouldEqual, a3.StoreFname())
			})
			Convey("Keep file", func() {
				a2 := h.Attachment().Create(env, h.Attachment().NewData().
					SetName("a2").
					SetDatas(blob1B64))
				a3 := h.Attachment().Create(env, h.Attachment().NewData().
					SetName("a2").
					SetDatas(blob1B64))
				a2FN := filepath.Join(a2.FileStore(), a2.StoreFname())
				a3.Unlink()
				_, err := os.Stat(a2FN)
				So(err, ShouldBeNil)
			})
			Convey("Change data change file", func() {
				a2 := h.Attachment().Create(env, h.Attachment().NewData().
					SetName("a2").
					SetDatas(blob1B64))
				a2StoreFName1 := a2.StoreFname()
				a2FN := filepath.Join(a2.FileStore(), a2StoreFName1)
				_, err := os.Stat(a2FN)
				So(err, ShouldBeNil)
				a2.SetDatas(blob2B64)
				a2StoreFName2 := a2.StoreFname()
				So(a2StoreFName1, ShouldNotEqual, a2StoreFName2)
				a2FN = filepath.Join(a2.FileStore(), a2StoreFName2)
				_, err = os.Stat(a2FN)
				So(err, ShouldBeNil)
			})
		}), ShouldBeNil)
	})
}
