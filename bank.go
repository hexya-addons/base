// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func sanitizeAccountNumber(accNumber string) string {
	if accNumber == "" {
		return ""
	}
	rg, _ := regexp.Compile(`\W+`)
	san := rg.ReplaceAllString(accNumber, "")
	san = strings.ToUpper(san)
	return san
}

func init() {
	h.Bank().DeclareModel()
	h.Bank().AddFields(map[string]models.FieldDefinition{
		"Name":    models.CharField{Required: true},
		"Street":  models.CharField{},
		"Street2": models.CharField{},
		"Zip":     models.CharField{},
		"City":    models.CharField{},
		"State": models.Many2OneField{RelationModel: h.CountryState(), String: "Fed. State",
			Filter: q.CountryState().Country().EqualsFunc(func(rs models.RecordSet) models.RecordSet {
				bank := rs.(m.BankSet)
				return bank.Country()
			})},
		"Country": models.Many2OneField{RelationModel: h.Country()},
		"Email":   models.CharField{},
		"Phone":   models.CharField{},
		"Fax":     models.CharField{},
		"Active":  models.BooleanField{Default: models.DefaultValue(true)},
		"BIC":     models.CharField{String: "Bank Identifier Cord", Index: true, Help: "Sometimes called BIC or Swift."},
	})
	h.Bank().Methods().NameGet().Extend("",
		func(rs m.BankSet) string {
			res := rs.Name()
			if rs.BIC() != "" {
				res = fmt.Sprintf("%s - %s", res, rs.BIC())
			}
			return res
		})

	h.Bank().Methods().SearchByName().Extend("",
		func(rs m.BankSet, name string, op operator.Operator, additionalCond q.BankCondition, limit int) m.BankSet {
			if name == "" {
				return rs.Super().SearchByName(name, op, additionalCond, limit)
			}
			cond := q.Bank().BIC().ILike(name+"%").Or().Name().AddOperator(op, name)
			if !additionalCond.Underlying().IsEmpty() {
				cond = cond.AndCond(additionalCond)
			}
			return h.Bank().Search(rs.Env(), cond).Limit(limit)
		})

	h.BankAccount().DeclareModel()
	h.BankAccount().AddFields(map[string]models.FieldDefinition{
		"AccountType": models.CharField{
			Compute: h.BankAccount().Methods().ComputeAccountType(),
			Help:    "Bank account type, inferred from account number"},
		"Name": models.CharField{
			String:   "Account Number",
			Required: true},
		"SanitizedAccountNumber": models.CharField{
			Compute:  h.BankAccount().Methods().ComputeSanitizedAccountNumber(),
			Stored:   true,
			ReadOnly: true,
			Depends:  []string{"Name"}},
		"Partner": models.Many2OneField{
			RelationModel: h.Partner(),
			String:        "Account Holder",
			OnDelete:      models.Cascade,
			Index:         true,
			Filter:        q.Partner().IsCompany().Equals(true).Or().Parent().IsNull()},
		"Bank": models.Many2OneField{
			RelationModel: h.Bank()},
		"BankName": models.CharField{
			Related: "Bank.Name"},
		"BankBIC": models.CharField{
			Related: "Bank.BIC"},
		"Sequence": models.IntegerField{},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency()},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company()
			},
			OnDelete: models.Cascade},
	})
	h.BankAccount().AddSQLConstraint("unique_number", "unique(sanitized_account_number, company_id)", "Account Number must be unique")

	h.BankAccount().Methods().ComputeAccountType().DeclareMethod(
		`ComputeAccountType computes the type of account from the account number`,
		func(rs m.BankAccountSet) m.BankAccountData {
			return h.BankAccount().NewData().SetAccountType("bank")
		})

	h.BankAccount().Methods().ComputeSanitizedAccountNumber().DeclareMethod(
		`ComputeSanitizedAccountNumber removes all spaces and invalid characters from account number`,
		func(rs m.BankAccountSet) m.BankAccountData {
			number := sanitizeAccountNumber(rs.Name())
			println(number)
			return h.BankAccount().NewData().SetSanitizedAccountNumber(number)
		})

	h.BankAccount().Methods().Search().Extend("",
		func(rs m.BankAccountSet, cond q.BankAccountCondition) m.BankAccountSet {
			predicates := cond.PredicatesWithField(h.BankAccount().Fields().Name())
			for i, pred := range predicates {
				switch arg := pred.Argument().(type) {
				case []string:
					newArg := make([]string, len(arg))
					for j, a := range arg {
						newArg[j] = sanitizeAccountNumber(a)
					}
					predicates[i].AlterArgument(newArg)
				case string:
					predicates[i].AlterArgument(sanitizeAccountNumber(arg))
				}
				predicates[i].AlterField(h.BankAccount().Fields().SanitizedAccountNumber())
			}
			return rs.Super().Search(cond)
		})

}
