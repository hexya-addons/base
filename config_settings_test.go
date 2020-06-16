// Copyright 2020 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"testing"

	"github.com/hexya-addons/base/basetypes"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	. "github.com/smartystreets/goconvey/convey"
)

func TestConfigSettings(t *testing.T) {
	Convey("Testing ConfigSettings", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			configSettings := h.ConfigSettings().NewSet(env).Create(h.ConfigSettings().NewData())
			Convey("New empty config settings should have default values", func() {
				So(configSettings.TestSettingsSimple(), ShouldEqual, "")
				So(configSettings.TestSettingsConfigChar(), ShouldEqual, "")
				So(configSettings.TestSettingsConfigInteger(), ShouldEqual, 0)
				So(configSettings.TestSettingsConfigFloat(), ShouldEqual, 0)
				So(configSettings.TestSettingsConfigBoolean(), ShouldBeFalse)
				So(configSettings.TestSettingsConfigSelection(), ShouldEqual, "yes")
				So(configSettings.TestSettingsConfigM2O().IsEmpty(), ShouldBeTrue)
			})
			Convey("Setting config values should persist as config parameters", func() {
				configSettings.SetTestSettingsSimple("value will be lost")
				configSettings.SetTestSettingsConfigChar("char value")
				configSettings.SetTestSettingsConfigInteger(13)
				configSettings.SetTestSettingsConfigFloat(2.6)
				configSettings.SetTestSettingsConfigBoolean(true)
				configSettings.SetTestSettingsConfigSelection("yes")
				configSettings.SetTestSettingsConfigM2O(h.User().BrowseOne(env, security.SuperUserID))
				configSettings.Execute()
				So(h.ConfigParameter().NewSet(env).GetParam("base.char", ""), ShouldEqual, "char value")
				So(h.ConfigParameter().NewSet(env).GetParam("base.integer", ""), ShouldEqual, "13")
				So(h.ConfigParameter().NewSet(env).GetParam("base.float", ""), ShouldEqual, "2.6")
				So(h.ConfigParameter().NewSet(env).GetParam("base.boolean", ""), ShouldEqual, "true")
				So(h.ConfigParameter().NewSet(env).GetParam("base.selection", ""), ShouldEqual, "yes")
				So(h.ConfigParameter().NewSet(env).GetParam("base.m2o", ""), ShouldEqual, "1")
				newCS := h.ConfigSettings().NewSet(env).Create(h.ConfigSettings().NewData())
				So(newCS.TestSettingsSimple(), ShouldEqual, "")
				So(newCS.TestSettingsConfigChar(), ShouldEqual, "char value")
				So(newCS.TestSettingsConfigInteger(), ShouldEqual, 13)
				So(newCS.TestSettingsConfigFloat(), ShouldEqual, 2.6)
				So(newCS.TestSettingsConfigBoolean(), ShouldBeTrue)
				So(newCS.TestSettingsConfigSelection(), ShouldEqual, "yes")
				So(newCS.TestSettingsConfigM2O().Equals(h.User().BrowseOne(env, security.SuperUserID)), ShouldBeTrue)
			})
		}), ShouldBeNil)
	})
}

func testConfigSettings_ConfigFields(rs m.ConfigSettingsSet) basetypes.ConfigFieldsMap {
	res := rs.Super().ConfigFields()
	res[h.ConfigSettings().Fields().TestSettingsConfigChar()] = "base.char"
	res[h.ConfigSettings().Fields().TestSettingsConfigInteger()] = "base.integer"
	res[h.ConfigSettings().Fields().TestSettingsConfigFloat()] = "base.float"
	res[h.ConfigSettings().Fields().TestSettingsConfigBoolean()] = "base.boolean"
	res[h.ConfigSettings().Fields().TestSettingsConfigSelection()] = "base.selection"
	res[h.ConfigSettings().Fields().TestSettingsConfigM2O()] = "base.m2o"
	return res
}

func init() {
	h.ConfigSettings().AddFields(map[string]models.FieldDefinition{
		"TestSettingsSimple":        fields.Char{},
		"TestSettingsConfigChar":    fields.Char{},
		"TestSettingsConfigInteger": fields.Integer{},
		"TestSettingsConfigFloat":   fields.Float{},
		"TestSettingsConfigM2O":     fields.Many2One{RelationModel: h.User()},
		"TestSettingsConfigBoolean": fields.Boolean{},
		"TestSettingsConfigSelection": fields.Selection{
			Selection: types.Selection{"yes": "YES", "no": "NO"},
			Default:   models.DefaultValue("yes"),
		},
	})
	h.ConfigSettings().Methods().ConfigFields().Extend(testConfigSettings_ConfigFields)
}
