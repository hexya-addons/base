// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package basetypes

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
