// Copyright 2018 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"strings"
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSanitizedAccountNumber(t *testing.T) {
	Convey("Test account numbers", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Sanitize account number", func() {
				accNumber := " BE-001 2518823 03 "
				partner2 := h.Partner().Search(env, q.Partner().HexyaExternalID().Equals("base_res_partner_2"))
				So(partner2.Len(), ShouldEqual, 1)
				vals := h.BankAccount().Search(env, q.BankAccount().Name().Equals(accNumber))
				So(vals.IsEmpty(), ShouldBeTrue)
				bankAccount := h.BankAccount().Create(env, &h.BankAccountData{
					Name:    accNumber,
					Partner: partner2,
				})
				vals = h.BankAccount().Search(env, q.BankAccount().Name().Equals(accNumber))
				So(vals.Len(), ShouldEqual, 1)
				So(vals.Equals(bankAccount), ShouldBeTrue)
				vals = h.BankAccount().Search(env, q.BankAccount().Name().In([]string{accNumber}))
				So(vals.Len(), ShouldEqual, 1)
				So(vals.Equals(bankAccount), ShouldBeTrue)

				So(bankAccount.Name(), ShouldEqual, accNumber)

				sanitizedAccountNumber := "BE001251882303"
				vals = h.BankAccount().Search(env, q.BankAccount().Name().Equals(sanitizedAccountNumber))
				So(vals.Len(), ShouldEqual, 1)
				So(vals.Equals(bankAccount), ShouldBeTrue)
				vals = h.BankAccount().Search(env, q.BankAccount().Name().In([]string{sanitizedAccountNumber}))
				So(vals.Len(), ShouldEqual, 1)
				So(vals.Equals(bankAccount), ShouldBeTrue)

				So(bankAccount.SanitizedAccountNumber(), ShouldEqual, sanitizedAccountNumber)

				vals = h.BankAccount().Search(env, q.BankAccount().Name().Equals(strings.ToLower(sanitizedAccountNumber)))
				So(vals.Len(), ShouldEqual, 1)
				vals = h.BankAccount().Search(env, q.BankAccount().Name().Equals(strings.ToLower(accNumber)))
				So(vals.Len(), ShouldEqual, 1)
			})
		}), ShouldBeNil)
	})
}
