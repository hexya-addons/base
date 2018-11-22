// Copyright 2016 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"fmt"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/tools/b64image"
	"github.com/hexya-erp/pool/h"
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

func init() {
	companyModel := h.Company().DeclareModel()
	companyModel.AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{String: "Company Name", Size: 128, Required: true,
			Related: "Partner.Name", Unique: true},
		"Parent": models.Many2OneField{RelationModel: h.Company(),
			String: "Parent Company", Index: true, Constraint: h.Company().Methods().CheckParent()},
		"Children": models.One2ManyField{RelationModel: h.Company(),
			ReverseFK: "Parent", String: "Child Companies"},
		"Partner": models.Many2OneField{RelationModel: h.Partner(),
			Required: true, Index: true},
		"Tagline": models.CharField{},
		"Logo":    models.BinaryField{Related: "Partner.Image"},
		"LogoWeb": models.BinaryField{Compute: h.Company().Methods().ComputeLogoWeb(),
			Stored: true, Depends: []string{"Partner", "Partner.Image"}},
		"Currency": models.Many2OneField{RelationModel: h.Currency(),
			Required: true, Default: CompanyGetUserCurrency},
		"Users":   models.Many2ManyField{RelationModel: h.User(), String: "Accepted Users"},
		"Street":  models.CharField{Related: "Partner.Street"},
		"Street2": models.CharField{Related: "Partner.Street2"},
		"Zip":     models.CharField{Related: "Partner.Zip"},
		"City":    models.CharField{Related: "Partner.City"},
		"State": models.Many2OneField{RelationModel: h.CountryState(),
			Related: "Partner.State", OnChange: h.Company().Methods().OnChangeState()},
		"Country": models.Many2OneField{RelationModel: h.Country(),
			Related: "Partner.Country", OnChange: h.Company().Methods().OnChangeCountry()},
		"Email":           models.CharField{Related: "Partner.Email"},
		"Phone":           models.CharField{Related: "Partner.Phone"},
		"Fax":             models.CharField{Related: "Partner.Fax"},
		"Website":         models.CharField{Related: "Partner.Website"},
		"VAT":             models.CharField{Related: "Partner.VAT"},
		"CompanyRegistry": models.CharField{Size: 64},
	})

	companyModel.Methods().Copy().Extend("",
		func(rs h.CompanySet, overrides *h.CompanyData, fieldsToReset ...models.FieldNamer) h.CompanySet {
			rs.EnsureOne()
			_, eName := overrides.Get(h.Company().Name(), fieldsToReset...)
			_, ePartner := overrides.Get(h.Company().Partner(), fieldsToReset...)
			if !eName && !ePartner {
				copyPartner := rs.Partner().Copy(new(h.PartnerData))
				overrides.Partner = copyPartner
				overrides.Name = copyPartner.Name()
				fieldsToReset = append(fieldsToReset, h.Company().Partner(), h.Company().Name())
			}
			return rs.Super().Copy(overrides, fieldsToReset...)
		})

	companyModel.Methods().ComputeLogoWeb().DeclareMethod(
		`ComputeLogoWeb returns a resized version of the company logo`,
		func(rs h.CompanySet) *h.CompanyData {
			res := h.CompanyData{
				LogoWeb: b64image.Resize(rs.Logo(), 160, 0, true),
			}
			return &res
		})

	companyModel.Methods().OnChangeState().DeclareMethod(
		`OnchangeState sets the country to the country of the state when you select one.`,
		func(rs h.CompanySet) (*h.CompanyData, []models.FieldNamer) {
			return &h.CompanyData{
				Country: rs.State().Country(),
			}, []models.FieldNamer{h.Company().Country()}
		})

	companyModel.Methods().GetEuro().DeclareMethod(
		`GetEuro returns the currency with rate 1 (euro by default, unless changed by the user)`,
		func(rs h.CompanySet) h.CurrencySet {
			return h.CurrencyRate().Search(rs.Env(), q.CurrencyRate().Rate().Equals(1)).Limit(1).Currency()
		})

	companyModel.Methods().OnChangeCountry().DeclareMethod(
		`OnChangeCountry updates the currency of this company on a country change`,
		func(rs h.CompanySet) (*h.CompanyData, []models.FieldNamer) {
			if rs.Country().IsEmpty() {
				userCurrency := CompanyGetUserCurrency(rs.Env()).(h.CurrencySet)
				return &h.CompanyData{
					Currency: userCurrency,
				}, []models.FieldNamer{h.Company().Currency()}
			}
			return &h.CompanyData{
				Currency: rs.Country().Currency(),
			}, []models.FieldNamer{h.Company().Currency()}
		})

	companyModel.Methods().CompanyDefaultGet().DeclareMethod(
		`CompanyDefaultGet returns the default company (usually the user's company).`,
		func(rs h.CompanySet) h.CompanySet {
			return h.User().NewSet(rs.Env()).GetCompany()
		})

	companyModel.Methods().Create().Extend("",
		func(rs h.CompanySet, data *h.CompanyData, fieldsToReset ...models.FieldNamer) h.CompanySet {
			if !data.Partner.IsEmpty() {
				return rs.Super().Create(data)
			}
			partner := h.Partner().Create(rs.Env(), &h.PartnerData{
				Name:        data.Name,
				CompanyType: "company",
				Image:       data.Logo,
				Customer:    false,
				Email:       data.Email,
				Phone:       data.Phone,
				Website:     data.Website,
				VAT:         data.VAT,
			})
			data.Partner = partner
			company := rs.Super().Create(data)
			partner.SetCompany(company)
			return company
		})

	companyModel.Methods().CheckParent().DeclareMethod(
		`CheckParent checks that there is no recursion in the company tree`,
		func(rs h.CompanySet) {
			rs.CheckRecursion()
		})

	companyModel.Methods().SearchByName().Extend("",
		func(rs h.CompanySet, name string, op operator.Operator, additionalCond q.CompanyCondition, limit int) h.CompanySet {
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
		})
}
