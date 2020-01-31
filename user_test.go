// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	. "github.com/smartystreets/goconvey/convey"
)

func TestUserAuthentication(t *testing.T) {
	Convey("Testing User Authentication", t, func() {
		models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			userJohn := h.User().Create(env, h.User().NewData().
				SetName("John Smith").
				SetLogin("jsmith").
				SetPassword("secret"))
			Convey("Correct user authentication", func() {
				uid := h.User().NewSet(env).Authenticate("jsmith", "secret")
				So(uid, ShouldEqual, userJohn.ID())
			})
			Convey("Invalid credentials authentication", func() {
				So(func() { h.User().NewSet(env).Authenticate("jsmith", "wrong-secret") }, ShouldPanicWith, security.InvalidCredentialsError("jsmith"))
			})
			Convey("Unknown user authentication", func() {
				So(func() { h.User().NewSet(env).Authenticate("jsmith2", "wrong-secret") }, ShouldPanicWith, security.UserNotFoundError("jsmith2"))
			})
			Convey("Empty passwords should fail too", func() {
				So(func() { h.User().NewSet(env).Authenticate("jsmith2", "") }, ShouldPanic)
			})
		})
	})
}
