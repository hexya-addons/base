// Copyright 2018 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

func init() {
	h.ConfigSettings().DeclareTransientModel()

	h.ConfigSettings().Methods().Copy().Extend("", func(rs m.ConfigSettingsSet, data m.ConfigSettingsData) m.ConfigSettingsSet {
		panic(rs.T("Cannot duplicate configuration"))
	})

	h.ConfigSettings().Methods().DefaultGet().Extend("",
		func(rs m.ConfigSettingsSet) models.FieldMap {
			res := rs.Super().DefaultGet()
			for fName := range rs.DefaultGet() {
				if gm, ok := rs.Collection().Model().Methods().Get("GetDefault" + fName); ok {
					res[fName] = gm.Call(rs.Collection())
				}
			}
			return res
		})

	h.ConfigSettings().Methods().Execute().DeclareMethod(
		`Execute this config settings wizard`,
		func(rs m.ConfigSettingsSet) *actions.Action {
			rs.EnsureOne()
			if rs.Env().Uid() != security.SuperUserID && h.User().NewSet(rs.Env()).CurrentUser().HasGroup("base_group_systeme") {
				panic(rs.T("Only administrators can change the settings"))
			}
			for fName := range rs.DefaultGet() {
				if sm, ok := rs.Collection().Model().Methods().Get("SetValue" + fName); ok {
					sm.Call(rs.Collection())
				}
			}
			return &actions.Action{
				Type: actions.ActionClient,
				Tag:  "reload",
			}
		})

	h.ConfigSettings().Methods().Cancel().DeclareMethod(
		`Cancel ignores the current record, and send the action to reopen the view`,
		func(rs m.ConfigSettingsSet) *actions.Action {
			var action *actions.Action
			for _, act := range actions.Registry.GetAll() {
				if act.Type == actions.ActionActWindow && act.Model == rs.ModelName() {
					action = act
					break
				}
			}
			return action
		})
}
