// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func init() {
	groupModel := h.Group().DeclareModel()
	groupModel.AddFields(map[string]models.FieldDefinition{
		"GroupID": models.CharField{Required: true},
		"Name":    models.CharField{Required: true, Translate: true},
	})

	groupModel.Methods().Create().Extend("",
		func(rs m.GroupSet, data m.GroupData) m.GroupSet {
			if rs.Env().Context().HasKey("GroupForceCreate") {
				return rs.Super().Create(data)
			}
			log.Panic(rs.T("Trying to create a security group"))
			panic("Unreachable")
		})

	groupModel.Methods().Write().Extend("",
		func(rs m.GroupSet, data m.GroupData) bool {
			log.Panic(rs.T("Trying to modify a security group"))
			panic("Unreachable")
		})

	groupModel.Methods().ReloadGroups().DeclareMethod(
		`ReloadGroups populates the Group table with groups from the security.Registry
		and refresh all memberships from the database to the security.Registry.`,
		func(rs m.GroupSet) {
			log.Debug("Reloading groups")
			// Sync groups: registry => Database
			var existingGroupIds []string
			for _, group := range security.Registry.AllGroups() {
				existingGroupIds = append(existingGroupIds, group.ID)
				if !h.Group().Search(rs.Env(), q.Group().GroupID().Equals(group.ID)).IsEmpty() {
					// The group already exists in the database
					continue
				}
				rs.WithContext("GroupForceCreate", true).Create(h.Group().NewData().
					SetGroupID(group.ID).
					SetName(group.Name))
			}
			// Remove unknown groups from database
			h.Group().Search(rs.Env(), q.Group().GroupID().NotIn(existingGroupIds)).Unlink()
			// Sync memberships: DB => Registry
			allUsers := h.User().NewSet(rs.Env()).SearchAll()
			allUsers.AddMandatoryGroups()
			allUsers.SyncMemberships()
		})
}
