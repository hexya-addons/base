// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/pool/h"
)

var fields_CountryGroup = map[string]models.FieldDefinition{
	"Name":      fields.Char{Required: true},
	"Countries": fields.Many2Many{RelationModel: h.Country()},
}

var fields_Country = map[string]models.FieldDefinition{
	"Name": fields.Char{String: "State Name", Required: true,
		Help: "Administrative divisions of a country. E.g. Fed. State, Departement, Canton"},
	"Country": fields.Many2One{RelationModel: h.Country(), Required: true},
	"Code": fields.Char{String: "State Code", Size: 3,
		Help: "The state code in max. three chars.", Required: true},
}

var fields_CountryState = map[string]models.FieldDefinition{
	"Name": fields.Char{String: "Country Name", Help: "The full name of the country.", Translate: true, Required: true, Unique: true},
	"Code": fields.Char{String: "Country Code", Size: 2, Unique: true, Help: "The ISO country code in two chars.\nYou can use this field for quick search."},
	"AddressFormat": fields.Text{Default: func(env models.Environment) interface{} {
		return "%(Street)s\n%(Street2)s\n%(City)s %(StateCode)s %(Zip)s\n%(CountryName)s"
	}, Help: "You can state here the usual format to use for the addresses belonging to this country."},
	"Currency":      fields.Many2One{RelationModel: h.Currency()},
	"Image":         fields.Binary{},
	"PhoneCode":     fields.Integer{String: "Country Calling Code"},
	"CountryGroups": fields.Many2Many{RelationModel: h.CountryGroup()},
	"States":        fields.One2Many{RelationModel: h.CountryState(), ReverseFK: "Country"},
}

func init() {
	models.NewModel("CountryGroup")
	h.CountryGroup().AddFields(fields_CountryGroup)

	models.NewModel("CountryState")
	h.CountryState().AddFields(fields_Country)
	h.CountryState().AddSQLConstraint("name_code_uniq", "unique(country_id, code)", "The code of the state must be unique by country !")

	models.NewModel("Country")
	h.Country().AddFields(fields_CountryState)
}
