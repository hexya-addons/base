// Copyright 2016 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/server"
	"github.com/hexya-erp/hexya/src/tools/b64image"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

// CompanyDependent is a context to add to make a field depend on the user's current company.
// If a company ID is passed in the context under the key "force_company", then this company
// is used instead.
var CompanyDependent = models.FieldContexts{
	"company": func(rs models.RecordSet) string {
		companyID := rs.Env().Context().GetInteger("force_company")
		if companyID == 0 {
			companyID = rs.Env().Context().GetInteger("company_id")
		}
		if companyID == 0 {
			return ""
		}
		return fmt.Sprintf("%d", companyID)
	},
}

// CompanyGetUserCurrency returns the currency of the current user's company if it exists
// or the default currency otherwise
func CompanyGetUserCurrency(env models.Environment) interface{} {
	currency := h.User().NewSet(env).GetCompany().Currency()
	if currency.IsEmpty() {
		return h.Company().NewSet(env).GetEuro()
	}
	return currency
}

var fields_Company = map[string]models.FieldDefinition{
	"Name": fields.Char{String: "Company Name", Size: 128, Required: true,
		Related: "Partner.Name", Unique: true},
	"Sequence": fields.Integer{Default: models.DefaultValue(10),
		Help: "Used to order Companies in the company switcher"},
	"Parent": fields.Many2One{RelationModel: h.Company(),
		String: "Parent Company", Index: true, Constraint: h.Company().Methods().CheckParent()},
	"Children": fields.One2Many{RelationModel: h.Company(),
		ReverseFK: "Parent", String: "Child Companies"},
	"Partner": fields.Many2One{RelationModel: h.Partner(),
		Required: true, Index: true},
	"Logo": fields.Binary{Related: "Partner.Image"},
	"LogoWeb": fields.Binary{Compute: h.Company().Methods().ComputeLogoWeb(),
		Stored: true, Depends: []string{"Partner", "Partner.Image"}},
	"Currency": fields.Many2One{RelationModel: h.Currency(),
		Required: true, Default: CompanyGetUserCurrency},
	"Users":   fields.Many2Many{RelationModel: h.User(), String: "Accepted Users"},
	"Street":  fields.Char{Related: "Partner.Street"},
	"Street2": fields.Char{Related: "Partner.Street2"},
	"Zip":     fields.Char{Related: "Partner.Zip"},
	"City":    fields.Char{Related: "Partner.City"},
	"State": fields.Many2One{RelationModel: h.CountryState(),
		Related: "Partner.State", OnChange: h.Company().Methods().OnChangeState()},
	"Country": fields.Many2One{RelationModel: h.Country(),
		Related: "Partner.Country", OnChange: h.Company().Methods().OnChangeCountry()},
	"Email":           fields.Char{Related: "Partner.Email"},
	"Phone":           fields.Char{Related: "Partner.Phone"},
	"Website":         fields.Char{Related: "Partner.Website"},
	"VAT":             fields.Char{Related: "Partner.VAT"},
	"CompanyRegistry": fields.Char{Size: 64},
	"Favicon": fields.Binary{String: "Company Favicon", Default: func(env models.Environment) interface{} {
		fileName := filepath.Join(server.ResourceDir, "static", "web", "src", "img", "favicon.ico")
		imgData, _ := ioutil.ReadFile(fileName)
		return base64.StdEncoding.EncodeToString(imgData)
	}, Help: `This field holds the image used to display a favicon for a given company.`},
}

func company_Copy(rs m.CompanySet, overrides m.CompanyData) m.CompanySet {
	rs.EnsureOne()
	if !overrides.HasName() && !overrides.HasPartner() {
		copyPartner := rs.Partner().Copy(nil)
		overrides.SetPartner(copyPartner)
		overrides.SetName(copyPartner.Name())
	}
	return rs.Super().Copy(overrides)
}

// ComputeLogoWeb returns a resized version of the company logo
func company_ComputeLogoWeb(rs m.CompanySet) m.CompanyData {
	res := h.Company().NewData().SetLogoWeb(b64image.Resize(rs.Logo(), 160, 0, true))
	return res
}

// OnchangeState sets the country to the country of the state when you select one.
func company_OnchangeState(rs m.CompanySet) m.CompanyData {
	return h.Company().NewData().SetCountry(rs.State().Country())
}

// GetEuro returns the currency with rate 1 (euro by default, unless changed by the user)
func company_GetEuro(rs m.CompanySet) m.CurrencySet {
	return h.CurrencyRate().Search(rs.Env(), q.CurrencyRate().Rate().Equals(1)).Limit(1).Currency()
}

// OnChangeCountry updates the currency of this company on a country change
func company_OnChangeCountry(rs m.CompanySet) m.CompanyData {
	if rs.Country().IsEmpty() {
		userCurrency := CompanyGetUserCurrency(rs.Env()).(m.CurrencySet)
		return h.Company().NewData().SetCurrency(userCurrency)
	}
	return h.Company().NewData().SetCurrency(rs.Country().Currency())
}

// CompanyDefaultGet returns the default company (usually the user's company).`,
func company_CompanyDefaultGet(rs m.CompanySet) m.CompanySet {
	return h.User().NewSet(rs.Env()).GetCompany()
}

func company_Create(rs m.CompanySet, data m.CompanyData) m.CompanySet {
	if !data.Partner().IsEmpty() {
		return rs.Super().Create(data)
	}
	partner := h.Partner().Create(rs.Env(), h.Partner().NewData().
		SetName(data.Name()).
		SetCompanyType("company").
		SetImage(data.Logo()).
		SetEmail(data.Email()).
		SetPhone(data.Phone()).
		SetWebsite(data.Website()).
		SetVAT(data.VAT()))
	data.SetPartner(partner)
	company := rs.Super().Create(data)
	partner.SetCompany(company)
	return company
}

// CheckParent checks that there is no recursion in the company tree`,
func company_CheckParent(rs m.CompanySet) {
	rs.CheckRecursion()
}

func company_SearchByName(rs m.CompanySet, name string, op operator.Operator, additionalCond q.CompanyCondition, limit int) m.CompanySet {
	// We browse as superuser. Otherwise, the user would be able to
	// select only the currently visible companies (according to rules,
	// which are probably to allow to see the child companies) even if
	// she belongs to some other companies.
	rSet := rs
	companies := h.Company().NewSet(rs.Env())
	if rs.Env().Context().HasKey("user_preference") {
		currentUser := h.User().NewSet(rs.Env()).CurrentUser().Sudo()
		companies = currentUser.Companies().Union(currentUser.Company())
		rSet = rSet.Sudo()
	}
	return rSet.Super().SearchByName(name, op, additionalCond, limit).Union(companies)
}

func init() {
	models.NewModel("Company")
	h.Company().AddFields(fields_Company)

	h.Company().Methods().Copy().Extend(company_Copy)
	h.Company().NewMethod("ComputeLogoWeb", company_ComputeLogoWeb)
	h.Company().NewMethod("OnChangeState", company_OnchangeState)
	h.Company().NewMethod("GetEuro", company_GetEuro)
	h.Company().NewMethod("OnChangeCountry", company_OnChangeCountry)
	h.Company().NewMethod("CompanyDefaultGet", company_CompanyDefaultGet)
	h.Company().Methods().Create().Extend(company_Create)
	h.Company().NewMethod("CheckParent", company_CheckParent)
	h.Company().Methods().SearchByName().Extend(company_SearchByName)
}
