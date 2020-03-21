// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/pool/h"
)

var fields_Lang = map[string]models.FieldDefinition{
	"Name": fields.Char{Required: true, Unique: true},
	"Code": fields.Char{String: "Locale Code", Required: true,
		Help: "This field is used to set/get locales for user", Unique: true},
	"ISOCode":      fields.Char{Help: "This ISO code is the name of PO files to use for translations"},
	"Translatable": fields.Boolean{},
	"Active":       fields.Boolean{},
	"Direction": fields.Selection{Selection: types.Selection{"ltr": "Left-to-Right", "rtl": "Right-to-left"},
		Required: true, Default: models.DefaultValue("ltr")},
	"DateFormat": fields.Char{Required: true, Default: models.DefaultValue("2006-01-02")},
	"TimeFormat": fields.Char{Required: true, Default: models.DefaultValue("15:04:05")},
	"Grouping": fields.Char{String: "Separator Format", Required: true,
		Default: models.DefaultValue("[]"), Help: `The Separator Format should be like [,n] where 0 < n :starting from Unit digit."
-1 will end the separation. e.g. [3,2,-1] will represent 106500 to be 1,06,500"
[1,2,-1] will represent it to be 106,50,0;[3] will represent it as 106,500."
Provided ',' as the thousand separator in each case.`},
	"DecimalPoint": fields.Char{String: "Decimal Separator", Required: true, Default: models.DefaultValue(".")},
	"ThousandsSep": fields.Char{String: "Thousands Separator", Default: models.DefaultValue(",")},
}

func init() {
	models.NewModel("Lang")
	h.Lang().AddFields(fields_Lang)
}
