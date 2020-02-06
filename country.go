// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/pool/h"
)

var fields_CountryGroup = map[string]models.FieldDefinition{
	"Name":      fields.Char{Required: true},
	"Countries": fields.Many2Many{RelationModel: h.Country()},
}

var fields_CountryState = map[string]models.FieldDefinition{
	"Name": fields.Char{String: "State Name", Required: true,
		Help: "Administrative divisions of a country. E.g. Fed. State, Departement, Canton"},
	"Country": fields.Many2One{RelationModel: h.Country(), Required: true},
	"Code": fields.Char{String: "State Code", Size: 3,
		Help: "The state code in max. three chars.", Required: true},
}

var fields_Country = map[string]models.FieldDefinition{
	"Name": fields.Char{String: "Country Name", Help: "The full name of the country.", Translate: true, Required: true, Unique: true},
	"Code": fields.Char{String: "Country Code", Size: 2, Unique: true, Help: "The ISO country code in two chars.\nYou can use this field for quick search."},
	"AddressFormat": fields.Text{Default: func(env models.Environment) interface{} {
		return "{{ .Street }}\n{{ .Street2 }}\n{{ .City }} {{ .StateCode }} {{ .Zip }}\n{{ .CountryName }}"
	}, Help: `You can state here the usual format to use for the addresses belonging to this country.
You can use Go-style string pattern with all the fields of the address 
(for example, use '{{ .Street }}' to display the field 'Street') plus
{{ .StateName }}: the name of the state
{{ .StateCode }}: the code of the state
{{ .CountryName }}: the name of the country
{{ .CountryCode }}: the code of the country
`},
	"AddressViewID": fields.Char{String: "Input View", Help: `Use this field if you want to replace the usual way to encode a complete address.
Note that the address_format field is used to modify the way to display addresses
(in reports for example), while this field is used to modify the input form for
addresses.`},
	"Currency":      fields.Many2One{RelationModel: h.Currency()},
	"Image":         fields.Binary{},
	"PhoneCode":     fields.Integer{String: "Country Calling Code"},
	"CountryGroups": fields.Many2Many{RelationModel: h.CountryGroup()},
	"States":        fields.One2Many{RelationModel: h.CountryState(), ReverseFK: "Country"},
	"NamePosition": fields.Selection{Selection: types.Selection{
		"before": "Before Address",
		"after":  "After Address",
	}, String: "Customer Name Position", Default: models.DefaultValue("before"),
		Help: "Determines where the customer/company name should be placed, i.e. after or before the address."},
	"VATLabel": fields.Char{Translate: true, Help: "Use this field if you want to change vat label."},
}

func init() {
	models.NewModel("CountryGroup")
	h.CountryGroup().AddFields(fields_CountryGroup)

	models.NewModel("CountryState")
	h.CountryState().AddFields(fields_CountryState)
	h.CountryState().AddSQLConstraint("name_code_uniq", "unique(country_id, code)", "The code of the state must be unique by country !")

	models.NewModel("Country")
	h.Country().AddFields(fields_Country)
}
