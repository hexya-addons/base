// Copyright 2018 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"strconv"
	"strings"

	"github.com/hexya-addons/base/basetypes"
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fieldtype"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

func configSettings_Copy(rs m.ConfigSettingsSet, _ m.ConfigSettingsData) m.ConfigSettingsSet {
	panic(rs.T("Cannot duplicate configuration"))
}

func configSettings_DefaultGet(rs m.ConfigSettingsSet) m.ConfigSettingsData {
	res := rs.Super().DefaultGet()

	// config: get & convert stored ConfigParameter (or default)
	for field, key := range rs.ConfigFields() {
		fi := rs.FieldGet(field)
		value := h.ConfigParameter().NewSet(rs.Env()).GetParam(key, "")
		if value == "" {
			if fi.DefaultFunc != nil {
				res.Set(field, fi.DefaultFunc(rs.Env()))
			}
			continue
		}
		switch fi.Type {
		case fieldtype.Many2One:
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				log.Warn("Error when converting value", "value", value, "field", fi.Name, "key", key)
				res.Set(field, rs.Env().Pool(fi.Relation).Wrap())
				continue
			}
			rrs := rs.Env().Pool(fi.Relation)
			rrs = rrs.Search(rrs.Model().Field(models.ID).Equals(id))
			res.Set(field, rrs.Wrap())
		case fieldtype.Integer:
			val, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				log.Warn("Error when converting value", "value", value, "field", fi.Name, "key", key)
				res.Set(field, int64(0))
				continue
			}
			res.Set(field, val)
		case fieldtype.Float:
			val, err := strconv.ParseFloat(value, 64)
			if err != nil {
				log.Warn("Error when converting value", "value", value, "field", fi.Name, "key", key)
				res.Set(field, float64(0))
				continue
			}
			res.Set(field, val)
		case fieldtype.Boolean:
			val, err := strconv.ParseBool(value)
			if err != nil {
				log.Warn("Error when converting value", "value", value, "field", fi.Name, "key", key)
				res.Set(field, false)
				continue
			}
			res.Set(field, val)
		case fieldtype.Char, fieldtype.Text, fieldtype.Selection:
			res.Set(field, value)
		}
	}
	res.MergeWith(rs.GetValues())
	return res
}

// ConfigFields returns a map between fields of ConfigSettings and a ConfigParameter key.
//
// If your ConfigSettings field adds a configuration to be stored as a ConfigParameter,
// then override this function to add an entry to the returned map with your field and
// mapping to the ConfigParameter key.
func configSettings_ConfigFields(_ m.ConfigSettingsSet) basetypes.ConfigFieldsMap {
	res := make(map[*models.Field]string)
	return res
}

// GetValues returns values for the fields other than `group` and `config`.
//
// You should override this method to set up your own logic for your config settings fields.
func configSettings_GetValues(_ m.ConfigSettingsSet) m.ConfigSettingsData {
	return h.ConfigSettings().NewData()
}

// SetValues set values for config fields.
//
// You should override this method to set up your own logic for your config settings fields.
func configSettings_SetValues(rs m.ConfigSettingsSet) {
	rs = rs.WithContext("active_test", false)

	// config fields: store ConfigParameters
	for field, key := range rs.ConfigFields() {
		fi := rs.FieldGet(field)
		var value string
		switch fi.Type {
		case fieldtype.Char, fieldtype.Text, fieldtype.Selection:
			value = strings.TrimSpace(rs.Get(field).(string))
		case fieldtype.Integer:
			value = strconv.FormatInt(rs.Get(field).(int64), 10)
		case fieldtype.Float:
			value = strconv.FormatFloat(rs.Get(field).(float64), 'f', -1, 64)
		case fieldtype.Many2One:
			rrs := rs.Get(field).(models.RecordSet)
			switch {
			case rrs.IsEmpty():
				value = "0"
			default:
				value = strconv.FormatInt(rrs.Ids()[0], 10)
			}
		case fieldtype.Boolean:
			value = strconv.FormatBool(rs.Get(field).(bool))
		}
		h.ConfigParameter().NewSet(rs.Env()).SetParam(key, value)
	}
}

// Execute this config settings wizard
func configSettings_Execute(rs m.ConfigSettingsSet) *actions.Action {
	rs.EnsureOne()
	if rs.Env().Uid() != security.SuperUserID && h.User().NewSet(rs.Env()).CurrentUser().HasGroup("base_group_systeme") {
		panic(rs.T("Only administrators can change the settings"))
	}
	rs.SetValues()
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
	h.ConfigSettings().NewMethod("GetValues", configSettings_GetValues)
	h.ConfigSettings().NewMethod("SetValues", configSettings_SetValues)
	h.ConfigSettings().NewMethod("ConfigFields", configSettings_ConfigFields)
	h.ConfigSettings().NewMethod("Execute", configSettings_Execute)
	h.ConfigSettings().NewMethod("Cancel", configSettings_Cancel)
}
