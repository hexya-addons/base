// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"fmt"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	"github.com/spf13/viper"
)

var defaultParameters = map[string]func(env models.Environment) (string, h.GroupSet){
	"web.base.url": func(env models.Environment) (string, h.GroupSet) {
		prefix := "http"
		if viper.GetString("Server.Certificate") != "" || viper.GetString("Server.Domain") != "" {
			prefix = "https"
		}
		return fmt.Sprintf("%s://localhost:%s", prefix, viper.GetString("Server.Port")), h.Group().NewSet(env)
	},
}

func init() {
	h.ConfigParameter().DeclareModel()
	h.ConfigParameter().AddFields(map[string]models.FieldDefinition{
		"Key":    models.CharField{Index: true, Required: true, Unique: true},
		"Value":  models.TextField{Required: true},
		"Groups": models.Many2ManyField{RelationModel: h.Group()},
	})

	h.ConfigParameter().Methods().Init().DeclareMethod(
		`Init Initializes the parameters listed in defaultParameters.
        It overrides existing parameters if force is 'true'.`,
		func(rs h.ConfigParameterSet, force ...bool) {
			var forceInit bool
			if len(force) > 0 && force[0] {
				forceInit = true
			}
			for key, fnct := range defaultParameters {
				params := rs.Model().NewSet(rs.Env()).Sudo().Search(q.ConfigParameter().Key().Equals(key))
				if forceInit || params.IsEmpty() {
					value, groups := fnct(rs.Env())
					h.ConfigParameter().NewSet(rs.Env()).SetParam(key, value).LimitToGroups(groups)
				}
			}
		})

	h.ConfigParameter().Methods().GetParam().DeclareMethod(
		`GetParam retrieves the value for a given key. It returns defaultValue if the parameter is missing.`,
		func(rs h.ConfigParameterSet, key string, defaultValue string) string {
			param := rs.Model().Search(rs.Env(), q.ConfigParameter().Key().Equals(key)).Limit(1).Load("Value")
			if param.Value() == "" {
				return defaultValue
			}
			return param.Value()
		})

	h.ConfigParameter().Methods().SetParam().DeclareMethod(
		`SetParam sets the value of a parameter. It returns the parameter`,
		func(rs h.ConfigParameterSet, key, value string) h.ConfigParameterSet {
			var res h.ConfigParameterSet
			param := rs.Model().Search(rs.Env(), q.ConfigParameter().Key().Equals(key))
			if param.IsEmpty() {
				if value != "" {
					res = rs.Create(&h.ConfigParameterData{
						Key:   key,
						Value: value,
					})
				}
				return res
			}
			if value == "" {
				param.Unlink()
				return rs.Model().NewSet(rs.Env())
			}
			param.SetValue(value)
			return param
		})

	h.ConfigParameter().Methods().LimitToGroups().DeclareMethod(
		`LimitToGroups limits the access to this key to the given list of groups`,
		func(rs h.ConfigParameterSet, groups h.GroupSet) {
			if rs.IsEmpty() {
				return
			}
			rs.SetGroups(groups)
		})

}
