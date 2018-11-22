// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/pool/h"
)

func init() {
	h.Lang().DeclareModel()
	h.Lang().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{Required: true, Unique: true},
		"Code": models.CharField{String: "Locale Code", Required: true,
			Help: "This field is used to set/get locales for user", Unique: true},
		"ISOCode":      models.CharField{Help: "This ISO code is the name of PO files to use for translations"},
		"Translatable": models.BooleanField{},
		"Active":       models.BooleanField{},
		"Direction": models.SelectionField{Selection: types.Selection{"ltr": "Left-to-Right", "rtl": "Right-to-left"},
			Required: true, Default: models.DefaultValue("ltr")},
		"DateFormat": models.CharField{Required: true, Default: models.DefaultValue("2006-01-02")},
		"TimeFormat": models.CharField{Required: true, Default: models.DefaultValue("15:04:05")},
		"Grouping": models.CharField{String: "Separator Format", Required: true,
			Default: models.DefaultValue("[]"), Help: `The Separator Format should be like [,n] where 0 < n :starting from Unit digit."
-1 will end the separation. e.g. [3,2,-1] will represent 106500 to be 1,06,500"
[1,2,-1] will represent it to be 106,50,0;[3] will represent it as 106,500."
Provided ',' as the thousand separator in each case.`},
		"DecimalPoint": models.CharField{String: "Decimal Separator", Required: true, Default: models.DefaultValue(".")},
		"ThousandsSep": models.CharField{String: "Thousands Separator", Default: models.DefaultValue(",")},
	})
}
