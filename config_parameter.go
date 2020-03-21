// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"fmt"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	"github.com/spf13/viper"
)

var defaultParameters = map[string]func(env models.Environment) (string, m.GroupSet){
	"web.base.url": func(env models.Environment) (string, m.GroupSet) {
		prefix := "http"
		if viper.GetString("Server.Certificate") != "" || viper.GetString("Server.Domain") != "" {
			prefix = "https"
		}
		return fmt.Sprintf("%s://localhost:%s", prefix, viper.GetString("Server.Port")), h.Group().NewSet(env)
	},
}

var fields_ConfigParameter = map[string]models.FieldDefinition{
	"Key":    fields.Char{Index: true, Required: true, Unique: true},
	"Value":  fields.Text{Required: true},
	"Groups": fields.Many2Many{RelationModel: h.Group()},
}

// Init Initializes the parameters listed in defaultParameters.
// It overrides existing parameters if force is 'true'.
func configParameter_Init(rs m.ConfigParameterSet, force ...bool) {
	var forceInit bool
	if len(force) > 0 && force[0] {
		forceInit = true
	}
	for key, fnct := range defaultParameters {
		params := rs.Env().Pool(rs.ModelName()).Sudo().Search(q.ConfigParameter().Key().Equals(key).Condition)
		if forceInit || params.IsEmpty() {
			value, groups := fnct(rs.Env())
			h.ConfigParameter().NewSet(rs.Env()).SetParam(key, value).LimitToGroups(groups)
		}
	}
}

// GetParam retrieves the value for a given key. It returns defaultValue if the parameter is missing.
func configParameter_GetParam(rs m.ConfigParameterSet, key string, defaultValue string) string {
	param := h.ConfigParameter().Search(rs.Env(), q.ConfigParameter().Key().Equals(key)).Limit(1).Load(h.ConfigParameter().Fields().Value())
	if param.Value() == "" {
		return defaultValue
	}
	return param.Value()
}

// SetParam sets the value of a parameter. It returns the parameter
func configParameter_SetParam(rs m.ConfigParameterSet, key, value string) m.ConfigParameterSet {
	var res m.ConfigParameterSet
	param := rs.Env().Pool(rs.ModelName()).Search(q.ConfigParameter().Key().Equals(key).Condition).Wrap("ConfigParameter").(m.ConfigParameterSet)
	if param.IsEmpty() {
		if value != "" {
			res = rs.Create(h.ConfigParameter().NewData().
				SetKey(key).
				SetValue(value))
		}
		return res
	}
	if value == "" {
		param.Unlink()
		return rs.Env().Pool(rs.ModelName()).Wrap("ConfigParameter").(m.ConfigParameterSet)
	}
	param.SetValue(value)
	return param
}

// LimitToGroups limits the access to this key to the given list of groups
func configParameter_LimitToGroups(rs m.ConfigParameterSet, groups m.GroupSet) {
	if rs.IsEmpty() {
		return
	}
	rs.SetGroups(groups)
}

func init() {
	models.NewModel("ConfigParameter")
	h.ConfigParameter().AddFields(fields_ConfigParameter)

	h.ConfigParameter().NewMethod("Init", configParameter_Init)
	h.ConfigParameter().NewMethod("GetParam", configParameter_GetParam)
	h.ConfigParameter().NewMethod("SetParam", configParameter_SetParam)
	h.ConfigParameter().NewMethod("LimitToGroups", configParameter_LimitToGroups)
}
