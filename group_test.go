// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGroupLoading(t *testing.T) {
	models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		var (
			adminUser, user                    h.UserSet
			adminGrp, someGroup, everyoneGroup h.GroupSet
		)
		Convey("Testing Group Loading", t, func() {
			h.Group().NewSet(env).ReloadGroups()
			groups := h.Group().NewSet(env).SearchAll()
			So(groups.Len(), ShouldEqual, len(security.Registry.AllGroups()))
			adminUser = h.User().Search(env, q.User().ID().Equals(security.SuperUserID))
			adminGrp = h.Group().Search(env, q.Group().GroupID().Equals(security.GroupAdminID))
			everyoneGroup = h.Group().Search(env, q.Group().GroupID().Equals(security.GroupEveryoneID))
			So(adminUser.Groups().Ids(), ShouldHaveLength, 2)
			So(adminUser.Groups().Ids(), ShouldContain, adminGrp.ID())
			So(adminUser.Groups().Ids(), ShouldContain, everyoneGroup.ID())
		})
		Convey("Testing Group ReLoading with a new group", t, func() {
			security.Registry.NewGroup("some_group", "Some Group")
			h.Group().NewSet(env).ReloadGroups()
			groups := h.Group().NewSet(env).SearchAll()
			So(groups.Len(), ShouldEqual, len(security.Registry.AllGroups()))
		})
		Convey("Creating a new user with a new group", t, func() {
			adminUser = h.User().Search(env, q.User().ID().Equals(security.SuperUserID))
			adminGrp = h.Group().Search(env, q.Group().GroupID().Equals(security.GroupAdminID))
			everyoneGroup = h.Group().Search(env, q.Group().GroupID().Equals(security.GroupEveryoneID))
			someGroup = h.Group().Search(env, q.Group().GroupID().Equals("some_group"))
			user = h.User().Create(env, &h.UserData{
				Name:   "Test User",
				Login:  "test_user",
				Groups: someGroup,
			})
			So(user.Groups().Ids(), ShouldHaveLength, 1)
			So(user.Groups().Ids(), ShouldContain, someGroup.ID())
			So(adminUser.Groups().Ids(), ShouldHaveLength, 2)
			So(adminUser.Groups().Ids(), ShouldContain, adminGrp.ID())
			So(adminUser.Groups().Ids(), ShouldContain, everyoneGroup.ID())
		})
		Convey("Giving the new user admin rights", t, func() {
			groups := someGroup.Union(adminGrp)
			user.SetGroups(groups)
			h.Group().NewSet(env).ReloadGroups()
			adminGrp = h.Group().Search(env, q.Group().GroupID().Equals(security.GroupAdminID))
			everyoneGroup = h.Group().Search(env, q.Group().GroupID().Equals(security.GroupEveryoneID))
			someGroup = h.Group().Search(env, q.Group().GroupID().Equals("some_group"))
			So(user.Groups().Ids(), ShouldHaveLength, 3)
			So(user.Groups().Ids(), ShouldContain, someGroup.ID())
			So(user.Groups().Ids(), ShouldContain, adminGrp.ID())
			So(user.Groups().Ids(), ShouldContain, everyoneGroup.ID())
			So(adminUser.Groups().Ids(), ShouldHaveLength, 2)
			So(adminUser.Groups().Ids(), ShouldContain, adminGrp.ID())
			So(adminUser.Groups().Ids(), ShouldContain, everyoneGroup.ID())
		})
		Convey("Removing rights and checking that after reload, we get admin right", t, func() {
			user = h.User().Search(env, q.User().Login().Equals("test_user"))
			user.SetGroups(h.Group().NewSet(env))
			adminUser = h.User().Search(env, q.User().ID().Equals(security.SuperUserID))
			adminUser.SetGroups(h.Group().NewSet(env))
			So(user.Groups().Ids(), ShouldBeEmpty)
			So(adminUser.Groups().Ids(), ShouldBeEmpty)
			h.Group().NewSet(env).ReloadGroups()
			// We need to reload admin and everyone groups because they changed id
			adminGrp = h.Group().Search(env, q.Group().GroupID().Equals(security.GroupAdminID))
			everyoneGroup = h.Group().Search(env, q.Group().GroupID().Equals(security.GroupEveryoneID))
			So(user.Groups().Ids(), ShouldHaveLength, 1)
			So(user.Groups().Ids(), ShouldContain, everyoneGroup.ID())
			So(adminUser.Groups().Ids(), ShouldHaveLength, 2)
			So(adminUser.Groups().Ids(), ShouldContain, adminGrp.ID())
			So(adminUser.Groups().Ids(), ShouldContain, everyoneGroup.ID())
		})
	})
}
