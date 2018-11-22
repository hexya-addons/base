// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/pool/h"
)

func init() {
	countryGroup := h.CountryGroup().DeclareModel()
	countryGroup.AddFields(map[string]models.FieldDefinition{
		"Name":      models.CharField{Required: true},
		"Countries": models.Many2ManyField{RelationModel: h.Country()},
	})

	countryState := h.CountryState().DeclareModel()
	countryState.AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{String: "State Name", Required: true,
			Help: "Administrative divisions of a country. E.g. Fed. State, Departement, Canton"},
		"Country": models.Many2OneField{RelationModel: h.Country(), Required: true},
		"Code": models.CharField{String: "State Code", Size: 3,
			Help: "The state code in max. three chars.", Required: true},
	})
	countryState.AddSQLConstraint("name_code_uniq", "unique(country_id, code)", "The code of the state must be unique by country !")

	country := h.Country().DeclareModel()
	country.AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{String: "Country Name", Help: "The full name of the country.", Translate: true, Required: true, Unique: true},
		"Code": models.CharField{String: "Country Code", Size: 2, Unique: true, Help: "The ISO country code in two chars.\nYou can use this field for quick search."},
		"AddressFormat": models.TextField{Default: func(env models.Environment) interface{} {
			return "%(Street)s\n%(Street2)s\n%(City)s %(StateCode)s %(Zip)s\n%(CountryName)s"
		}, Help: "You can state here the usual format to use for the addresses belonging to this country."},
		"Currency":      models.Many2OneField{RelationModel: h.Currency()},
		"Image":         models.BinaryField{},
		"PhoneCode":     models.IntegerField{String: "Country Calling Code"},
		"CountryGroups": models.Many2ManyField{RelationModel: h.CountryGroup()},
		"States":        models.One2ManyField{RelationModel: h.CountryState(), ReverseFK: "Country"},
	})
}
