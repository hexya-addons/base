// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package basetypes

import (
	"github.com/hexya-erp/hexya/src/models"
)

// An AddressData holds address data for formating an address
type AddressData struct {
	Street      string
	Street2     string
	City        string
	Zip         string
	StateCode   string
	StateName   string
	CountryName string
	CountryCode string
	CompanyName string
}

// A ConfigFieldsMap is a map between fields of ConfigSettings and a ConfigParameter key.
type ConfigFieldsMap map[*models.Field]string
