// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func sanitizeAccountNumber(accNumber string) string {
	if accNumber == "" {
		return ""
	}
	rg, _ := regexp.Compile("\\W+")
	san := rg.ReplaceAllString(accNumber, "")
	san = strings.ToUpper(san)
	return san
}

var fields_Bank = map[string]models.FieldDefinition{
	"Name":    fields.Char{Required: true},
	"Street":  fields.Char{},
	"Street2": fields.Char{},
	"Zip":     fields.Char{},
	"City":    fields.Char{},
	"State": fields.Many2One{RelationModel: h.CountryState(), String: "Fed. State",
		Filter: q.CountryState().Country().EqualsFunc(func(rs models.RecordSet) models.RecordSet {
			bank := rs.(m.BankSet)
			return bank.Country()
		}),
		OnChange: h.Bank().Methods().OnchangeState()},
	"Country": fields.Many2One{RelationModel: h.Country(), OnChange: h.Bank().Methods().OnchangeCountry()},
	"Email":   fields.Char{},
	"Phone":   fields.Char{},
	"Fax":     fields.Char{},
	"Active":  fields.Boolean{Default: models.DefaultValue(true)},
	"BIC":     fields.Char{String: "Bank Identifier Cord", Index: true, Help: "Sometimes called BIC or Swift."},
}

func bank_NameGet(rs m.BankSet) string {
	res := rs.Name()
	if rs.BIC() != "" {
		res = fmt.Sprintf("%s - %s", res, rs.BIC())
	}
	return res
}

func bank_SearchByName(rs m.BankSet, name string, op operator.Operator, additionalCond q.BankCondition, limit int) m.BankSet {
	if name == "" {
		return rs.Super().SearchByName(name, op, additionalCond, limit)
	}
	cond := q.Bank().BIC().ILike(name+"%").Or().Name().AddOperator(op, name)
	if !additionalCond.Underlying().IsEmpty() {
		cond = cond.AndCond(additionalCond)
	}
	return h.Bank().Search(rs.Env(), cond).Limit(limit)
}

// OnchangeCountry updates the state field when country is changed
func bank_OnchangeCountry(rs m.BankSet) m.BankData {
	res := h.Bank().NewData()
	if rs.Country().IsNotEmpty() && !rs.Country().Equals(rs.State().Country()) {
		res.SetState(nil)
	}
	return res
}

// OnchangeState updates the country field when the state is changed
func bank_OnchangeState(rs m.BankSet) m.BankData {
	res := h.Bank().NewData()
	if rs.State().Country().IsNotEmpty() {
		res.SetCountry(rs.State().Country())
	}
	return res
}

var fields_BankAccount = map[string]models.FieldDefinition{
	"AccountType": fields.Char{Compute: h.BankAccount().Methods().ComputeAccountType(), Depends: []string{""}},
	"Name":        fields.Char{String: "Account Number", Required: true},
	"SanitizedAccountNumber": fields.Char{Compute: h.BankAccount().Methods().ComputeSanitizedAccountNumber(),
		Stored: true, Depends: []string{"Name"}},
	"Partner": fields.Many2One{RelationModel: h.Partner(),
		String: "Account Holder", OnDelete: models.Cascade, Index: true,
		Filter: q.Partner().IsCompany().Equals(true).Or().Parent().IsNull()},
	"Bank":     fields.Many2One{RelationModel: h.Bank()},
	"BankName": fields.Char{Related: "Bank.Name"},
	"BankBIC":  fields.Char{Related: "Bank.BIC"},
	"Sequence": fields.Integer{},
	"Currency": fields.Many2One{RelationModel: h.Currency()},
	"Company": fields.Many2One{RelationModel: h.Company(), Required: true, Default: func(env models.Environment) interface{} {
		return h.User().NewSet(env).CurrentUser().Company()
	}},
}

// ComputeAccountType computes the type of account from the account number
func bankAccount_ComputeAccountType(rs m.BankAccountSet) m.BankAccountData {
	return h.BankAccount().NewData().SetAccountType("bank")
}

// ComputeSanitizedAccountNumber removes all spaces and invalid characters from account number
func bankAccount_ComputeSanitizedAccountNumber(rs m.BankAccountSet) m.BankAccountData {
	return h.BankAccount().NewData().SetSanitizedAccountNumber(sanitizeAccountNumber(rs.Name()))
}

func bankAccount_Search(rs m.BankAccountSet, cond q.BankAccountCondition) m.BankAccountSet {
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
}

func init() {
	models.NewModel("Bank")
	h.Bank().AddFields(fields_Bank)

	h.Bank().Methods().NameGet().Extend(bank_NameGet)
	h.Bank().Methods().SearchByName().Extend(bank_SearchByName)
	h.Bank().NewMethod("OnchangeCountry", bank_OnchangeCountry)
	h.Bank().NewMethod("OnchangeState", bank_OnchangeState)

	models.NewModel("BankAccount")
	h.BankAccount().AddFields(fields_BankAccount)
	h.BankAccount().AddSQLConstraint("unique_number", "unique(sanitized_account_number, company_id)", "Account Number must be unique")

	h.BankAccount().NewMethod("ComputeAccountType", bankAccount_ComputeAccountType)
	h.BankAccount().NewMethod("ComputeSanitizedAccountNumber", bankAccount_ComputeSanitizedAccountNumber)
	h.BankAccount().Methods().Search().Extend(bankAccount_Search)
}
