// Copyright 2016 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"fmt"
	"math"
	"regexp"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

const CurrencyDisplayPattern = `(\w+)\s*(?:\((.*)\))?`

func init() {
	currencyRateModel := h.CurrencyRate().DeclareModel()
	currencyRateModel.AddFields(map[string]models.FieldDefinition{
		"Name": models.DateTimeField{String: "Date", Required: true, Index: true},
		"Rate": models.FloatField{Digits: nbutils.Digits{Precision: 12, Scale: 6},
			Help: "The rate of the currency to the currency of rate 1"},
		"Currency": models.Many2OneField{RelationModel: h.Currency()},
		"Company":  models.Many2OneField{RelationModel: h.Company()},
	})

	currencyModel := h.Currency().DeclareModel()
	currencyModel.AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{String: "Currency", Help: "Currency Code [ISO 4217]", Size: 3,
			Unique: true},
		"Symbol": models.CharField{Help: "Currency sign, to be used when printing amounts", Size: 4},
		"Rate": models.FloatField{String: "Current Rate",
			Help: "The rate of the currency to the currency of rate 1", Digits: nbutils.Digits{Precision: 12, Scale: 6},
			Compute: h.Currency().Methods().ComputeCurrentRate(), Depends: []string{"Rates", "Rates.Rate"}},
		"Rates": models.One2ManyField{RelationModel: h.CurrencyRate(), ReverseFK: "Currency"},
		"Rounding": models.FloatField{String: "Rounding Factor", Digits: nbutils.Digits{Precision: 12,
			Scale: 6}},
		"DecimalPlaces": models.IntegerField{GoType: new(int),
			Compute: h.Currency().Methods().ComputeDecimalPlaces(), Depends: []string{"Rounding"}},
		"Active": models.BooleanField{},
		"Position": models.SelectionField{Selection: types.Selection{"after": "After Amount", "before": "Before Amount"},
			String: "Symbol Position", Help: "Determines where the currency symbol should be placed after or before the amount."},
		"Date": models.DateField{Compute: h.Currency().Methods().ComputeDate(), Depends: []string{"Rates", "Rates.Name"}},
	})

	currencyModel.Methods().ComputeCurrentRate().DeclareMethod(
		`ComputeCurrentRate returns the current rate of this currency.
		 If a 'date' key (type DateTime) is given in the context, then it is used to compute the rate,
		 otherwise now is used.`,
		func(rs m.CurrencySet) m.CurrencyData {
			date := dates.Now()
			if rs.Env().Context().HasKey("date") {
				date = rs.Env().Context().GetDateTime("date")
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
		})

	currencyModel.Methods().ComputeDecimalPlaces().DeclareMethod(
		`ComputeDecimalPlaces returns the decimal place from the currency's rounding`,
		func(rs m.CurrencySet) m.CurrencyData {
			var dp int
			if rs.Rounding() > 0 && rs.Rounding() < 1 {
				dp = int(math.Ceil(math.Log10(1 / rs.Rounding())))
			}
			return h.Currency().NewData().SetDecimalPlaces(dp)
		})

	currencyModel.Methods().ComputeDate().DeclareMethod(
		`ComputeDate returns the date of the last rate of this currency`,
		func(rs m.CurrencySet) m.CurrencyData {
			var lastDate dates.Date
			if rateLength := len(rs.Rates().Records()); rateLength > 0 {
				lastDate = rs.Rates().Records()[rateLength-1].Name().ToDate()
			}
			return h.Currency().NewData().SetDate(lastDate)
		})

	currencyModel.Methods().Round().DeclareMethod(
		`Round returns the given amount rounded according to this currency rounding rules`,
		func(rs m.CurrencySet, amount float64) float64 {
			return nbutils.Round(amount, math.Pow10(-rs.DecimalPlaces()))
		})

	currencyModel.Methods().CompareAmounts().DeclareMethod(
		`CompareAmounts compares 'amount1' and 'amount2' after rounding them according
		 to the given currency's precision. The returned values are per the following table:

		     value1 > value2 : 1
    	     value1 == value2: 0
    	     value1 < value2 : -1

		 An amount is considered lower/greater than another amount if their rounded
         value is different. This is not the same as having a non-zero difference!

         For example 1.432 and 1.431 are equal at 2 digits precision,
         so this method would return 0.
         However 0.006 and 0.002 are considered different (returns 1) because
         they respectively round to 0.01 and 0.0, even though 0.006-0.002 = 0.004
         which would be considered zero at 2 digits precision.`,
		func(rs m.CurrencySet, amount1, amount2 float64) int8 {
			return nbutils.Compare(amount1, amount2, math.Pow10(-rs.DecimalPlaces()))
		})

	currencyModel.Methods().IsZero().DeclareMethod(
		`IsZero returns true if 'amount' is small enough to be treated as
		zero according to current currency's rounding rules.

		Warning: IsZero(amount1-amount2) is not always equivalent to
		CompareAmomuts(amount1,amount2) == _, true, as the former will
		round after computing the difference, while the latter will round
		before, giving different results for e.g. 0.006 and 0.002 at 2
		digits precision.`,
		func(rs m.CurrencySet, amount float64) bool {
			return nbutils.IsZero(amount, math.Pow10(-rs.DecimalPlaces()))
		})

	currencyModel.Methods().GetConversionRateTo().DeclareMethod(
		`GetConversionRateTo returns the conversion rate from this currency to 'target' currency`,
		func(rs m.CurrencySet, target m.CurrencySet) float64 {
			return target.WithEnv(rs.Env()).Rate() / rs.Rate()
		})

	currencyModel.Methods().Compute().DeclareMethod(
		`Compute converts 'amount' from this currency to 'targetCurrency'.
		 The result is rounded to the 'target' currency if 'round' is true.`,
		func(rs m.CurrencySet, amount float64, target m.CurrencySet, round bool) float64 {
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
		})

	currencyModel.Methods().GetFormatCurrenciesJsFunction().DeclareMethod(
		`GetFormatCurrenciesJsFunction returns a string that can be used to instanciate a javascript
		function that formats numbers as currencies.

		That function expects the number as first parameter	and the currency id as second parameter.
		If the currency id parameter is false or undefined, the	company currency is used.`,
		func(rs m.CurrencySet) string {
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
		})

	currencyModel.Methods().SelectCompaniesRates().DeclareMethod(`
		SelectCompaniesRates returns an SQL query to get the currency rates per companies.`,
		func(rs m.CurrencySet) string {
			return `
			SELECT
                r.currency_id,
                COALESCE(r.company_id, c.id) as company_id,
                r.rate,
                r.name AS date_start,
                (SELECT name FROM currency_rate r2
                 WHERE r2.name > r.name AND
                       r2.currency_id = r.currency_id AND
                       (r2.company_id is null or r2.company_id = c.id)
                 ORDER BY r2.name ASC
                 LIMIT 1) AS date_end
            FROM currency_rate r
            JOIN company c ON (r.company_id is null or r.company_id = c.id)`
		})

	currencyModel.Methods().SearchByName().Extend("",
		func(rs m.CurrencySet, name string, op operator.Operator, additionalCond q.CurrencyCondition, limit int) m.CurrencySet {
			res := rs.Super().SearchByName(name, op, additionalCond, limit)
			if res.IsEmpty() {
				re, _ := regexp.Compile(CurrencyDisplayPattern)
				if m := re.FindString(name); m != "" {
					res = rs.Super().SearchByName(m, op, additionalCond, limit)
				}
			}
			return res
		})
}
