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

func configSettings_Copy(rs m.ConfigSettingsSet, data m.ConfigSettingsData) m.ConfigSettingsSet {
	panic(rs.T("Cannot duplicate configuration"))
}

func configSettings_DefaultGet(rs m.ConfigSettingsSet) m.ConfigSettingsData {
	res := rs.Super().DefaultGet()
	for _, fName := range res.FieldNames() {
		if gm, ok := h.ConfigSettings().Methods().Get("GetDefault" + fName.Name()); ok {
			res.Set(fName, gm.Call(rs.Collection()))
		}
	}
	return res
}

// Execute this config settings wizard
func configSettings_Execute(rs m.ConfigSettingsSet) *actions.Action {
	rs.EnsureOne()
	if rs.Env().Uid() != security.SuperUserID && h.User().NewSet(rs.Env()).CurrentUser().HasGroup("base_group_systeme") {
		panic(rs.T("Only administrators can change the settings"))
	}
	for fName := range rs.DefaultGet().FieldNames() {
		if sm, ok := rs.Collection().Model().Methods().Get("SetValue" + string(fName)); ok {
			sm.Call(rs.Collection())
		}
	}
	return &actions.Action{
		Type: actions.ActionClient,
		Tag:  "reload",
	}
}

// Cancel ignores the current record, and send the action to reopen the view
func configSettings_Cancel(rs m.ConfigSettingsSet) *actions.Action {
	var action *actions.Action
	for _, act := range actions.Registry.GetAll() {
		if act.Type == actions.ActionActWindow && act.Model == rs.ModelName() {
			action = act
			break
		}
	}
	return action
}

func init() {
	models.NewTransientModel("ConfigSettings")
	h.ConfigSettings().Methods().Copy().Extend(configSettings_Copy)
	h.ConfigSettings().Methods().DefaultGet().Extend(configSettings_DefaultGet)
	h.ConfigSettings().NewMethod("Execute", configSettings_Execute)
	h.ConfigSettings().NewMethod("Cancel", configSettings_Cancel)
}
