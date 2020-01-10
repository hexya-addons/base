// Copyright 2016 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"fmt"
	"math"
	"regexp"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

// CurrencyDisplayPattern is the regexp pattern for displaying a currency
const CurrencyDisplayPattern = `(\w+)\s*(?:\((.*)\))?`

var fields_CurrencyRate = map[string]models.FieldDefinition{
	"Name": fields.DateTime{String: "Date", Required: true, Index: true},
	"Rate": fields.Float{Digits: nbutils.Digits{Precision: 16, Scale: 6},
		Help: "The rate of the currency to the currency of rate 1"},
	"Currency": fields.Many2One{RelationModel: h.Currency()},
	"Company":  fields.Many2One{RelationModel: h.Company()},
}
var fields_Currency = map[string]models.FieldDefinition{
	"Name": fields.Char{String: "Currency", Help: "Currency Code [ISO 4217]", Size: 3,
		Unique: true},
	"Symbol": fields.Char{Help: "Currency sign, to be used when printing amounts", Size: 4},
	"Rate": fields.Float{String: "Current Rate",
		Help: "The rate of the currency to the currency of rate 1", Digits: nbutils.Digits{Precision: 16, Scale: 6},
		Compute: h.Currency().Methods().ComputeCurrentRate(), Depends: []string{"Rates", "Rates.Rate"}},
	"Rates": fields.One2Many{RelationModel: h.CurrencyRate(), ReverseFK: "Currency"},
	"Rounding": fields.Float{String: "Rounding Factor", Digits: nbutils.Digits{Precision: 12,
		Scale: 6}},
	"DecimalPlaces": fields.Integer{GoType: new(int),
		Compute: h.Currency().Methods().ComputeDecimalPlaces(), Depends: []string{"Rounding"}},
	"Active": fields.Boolean{},
	"Position": fields.Selection{Selection: types.Selection{"after": "After Amount", "before": "Before Amount"},
		String: "Symbol Position", Help: "Determines where the currency symbol should be placed after or before the amount."},
	"Date": fields.Date{Compute: h.Currency().Methods().ComputeDate(), Depends: []string{"Rates", "Rates.Name"}},
}

// ComputeCurrentRate returns the current rate of this currency.
// If a 'date' key is given in the context, then it is used to compute the rate,
// otherwise now is used.
func currency_ComputeCurrentRate(rs m.CurrencySet) m.CurrencyData {
	date := dates.Now()
	if rs.Env().Context().HasKey("date") {
		date = rs.Env().Context().GetDate("date").ToDateTime()
	}
	company := h.User().NewSet(rs.Env()).GetCompany()
	if rs.Env().Context().HasKey("company_id") {
		company = h.Company().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("company_id")})
	}
	rate := h.CurrencyRate().Search(rs.Env(),
		q.CurrencyRate().Currency().Equals(rs).
			And().Name().LowerOrEqual(date).
			AndCond(
				q.CurrencyRate().Company().IsNull().
					Or().Company().Equals(company))).
		OrderBy("Company", "Name desc").
		Limit(1)
	res := rate.Rate()
	if res == 0 {
		res = 1.0
	}
	return h.Currency().NewData().SetRate(res)
}

// ComputeDecimalPlaces returns the decimal place from the currency's rounding
func currency_ComputeDecimalPlaces(rs m.CurrencySet) m.CurrencyData {
	var dp int
	if rs.Rounding() > 0 && rs.Rounding() < 1 {
		dp = int(math.Ceil(math.Log10(1 / rs.Rounding())))
	}
	return h.Currency().NewData().SetDecimalPlaces(dp)
}

// ComputeDate returns the date of the last rate of this currency
func currency_ComputeDate(rs m.CurrencySet) m.CurrencyData {
	var lastDate dates.Date
	if rateLength := len(rs.Rates().Records()); rateLength > 0 {
		lastDate = rs.Rates().Records()[rateLength-1].Name().ToDate()
	}
	return h.Currency().NewData().SetDate(lastDate)
}

// Round returns the given amount rounded according to this currency rounding rules
func currency_Round(rs m.CurrencySet, amount float64) float64 {
	return nbutils.Round(amount, math.Pow10(-rs.DecimalPlaces()))
}

// CompareAmounts compares 'amount1' and 'amount2' after rounding them according
// to the given currency's precision. The returned values are per the following table:
//
//     value1 > value2 : 1
//     value1 == value2: 0
//     value1 < value2 : -1
//
// An amount is considered lower/greater than another amount if their rounded
// value is different. This is not the same as having a non-zero difference!
//
// For example 1.432 and 1.431 are equal at 2 digits precision,
// so this method would return 0.
// However 0.006 and 0.002 are considered different (returns 1) because
// they respectively round to 0.01 and 0.0, even though 0.006-0.002 = 0.004
// which would be considered zero at 2 digits precision.
func currency_CompareAmounts(rs m.CurrencySet, amount1, amount2 float64) int8 {
	return nbutils.Compare(amount1, amount2, math.Pow10(-rs.DecimalPlaces()))
}

// IsZero returns true if 'amount' is small enough to be treated as
// zero according to current currency's rounding rules.
//
// Warning: IsZero(amount1-amount2) is not always equivalent to
// CompareAmomuts(amount1,amount2) == _, true, as the former will
// round after computing the difference, while the latter will round
// before, giving different results for e.g. 0.006 and 0.002 at 2
// digits precision.
func currency_IsZero(rs m.CurrencySet, amount float64) bool {
	return nbutils.IsZero(amount, math.Pow10(-rs.DecimalPlaces()))
}

// GetConversionRateTo returns the conversion rate from this currency to 'target' currency
func currency_GetConversionRateTo(rs m.CurrencySet, target m.CurrencySet) float64 {
	return target.WithNewContext(rs.Env().Context()).Rate() / rs.Rate()
}

// Compute converts 'amount' from this currency to 'targetCurrency'.
// The result is rounded to the 'target' currency if 'round' is true.
func currency_Compute(rs m.CurrencySet, amount float64, target m.CurrencySet, round bool) float64 {
	if rs.Equals(target) {
		if round {
			return rs.Round(amount)
		}
		return amount
	}
	res := amount * rs.GetConversionRateTo(target)
	if round {
		return target.Round(res)
	}
	return res
}

// GetFormatCurrenciesJsFunction returns a string that can be used to instanciate a javascript
// 		function that formats numbers as currencies.
//
// 		That function expects the number as first parameter	and the currency id as second parameter.
// 		If the currency id parameter is false or undefined, the	company currency is used.
func currency_GetFormatCurrenciesJsFunction(rs m.CurrencySet) string {
	companyCurrency := h.User().Browse(rs.Env(), []int64{rs.Env().Uid()}).Company().Currency()
	var function string
	for _, currency := range h.Currency().NewSet(rs.Env()).SearchAll().Records() {
		symbol := currency.Symbol()
		if symbol == "" {
			symbol = currency.Name()
		}
		formatNumberStr := fmt.Sprintf("hexyaerp.web.format_value(arguments[0], {type: 'float', digits: [69,%d]}, 0.00)", currency.DecimalPlaces())
		returnStr := fmt.Sprintf("return %s + '\\xA0' + %s;", formatNumberStr, symbol)
		if currency.Position() == "before" {
			returnStr = fmt.Sprintf("return %s + '\\xA0' + %s;", symbol, formatNumberStr)
		}
		function += fmt.Sprintf("if (arguments[1] === %v) { %s }", currency.ID(), returnStr)
		if currency.Equals(companyCurrency) {
			companyCurrentFormat := returnStr
			function = fmt.Sprintf("if (arguments[1] === false || arguments[1] === undefined) { %s }%s", companyCurrentFormat, function)
		}
	}
	return function
}

// SelectCompaniesRates returns an SQL query to get the currency rates per companies.
func currency_SelectCompaniesRates(_ m.CurrencySet) string {
	return `
SELECT r.currency_id,
       COALESCE(r.company_id, c.id) as company_id,
       r.rate,
       r.name                       AS date_start,
       (SELECT name
        FROM currency_rate r2
        WHERE r2.name > r.name
          AND r2.currency_id = r.currency_id
          AND (r2.company_id is null or r2.company_id = c.id)
        ORDER BY r2.name ASC
        LIMIT 1)                    AS date_end
FROM currency_rate r
         JOIN company c ON (r.company_id is null or r.company_id = c.id)`
}

func currency_SearchByName(rs m.CurrencySet, name string, op operator.Operator, additionalCond q.CurrencyCondition, limit int) m.CurrencySet {
	res := rs.Super().SearchByName(name, op, additionalCond, limit)
	if res.IsEmpty() {
		re, _ := regexp.Compile(CurrencyDisplayPattern)
		if x := re.FindString(name); x != "" {
			res = rs.Super().SearchByName(x, op, additionalCond, limit)
		}
	}
	return res
}

func init() {
	models.NewModel("CurrencyRate")
	h.CurrencyRate().AddFields(fields_CurrencyRate)

	models.NewModel("Currency")
	h.Currency().AddFields(fields_Currency)

	h.Currency().NewMethod("ComputeCurrentRate", currency_ComputeCurrentRate)
	h.Currency().NewMethod("ComputeDecimalPlaces", currency_ComputeDecimalPlaces)
	h.Currency().NewMethod("ComputeDate", currency_ComputeDate)
	h.Currency().NewMethod("Round", currency_Round)
	h.Currency().NewMethod("CompareAmounts", currency_CompareAmounts)
	h.Currency().NewMethod("IsZero", currency_IsZero)
	h.Currency().NewMethod("GetConversionRateTo", currency_GetConversionRateTo)
	h.Currency().NewMethod("Compute", currency_Compute)
	h.Currency().NewMethod("GetFormatCurrenciesJsFunction", currency_GetFormatCurrenciesJsFunction)
	h.Currency().NewMethod("SelectCompaniesRates", currency_SelectCompaniesRates)
	h.Currency().Methods().SearchByName().Extend(currency_SearchByName)
}
