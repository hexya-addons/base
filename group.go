// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

var fields_Group = map[string]models.FieldDefinition{
	"GroupID": fields.Char{Required: true},
	"Name":    fields.Char{Required: true, Translate: true},
}

func group_Create(rs m.GroupSet, data m.GroupData) m.GroupSet {
	if rs.Env().Context().HasKey("GroupForceCreate") {
		return rs.Super().Create(data)
	}
	log.Panic(rs.T("Trying to create a security group"))
	panic("Unreachable")
}

func group_Write(rs m.GroupSet, _ m.GroupData) bool {
	log.Panic(rs.T("Trying to modify a security group"))
	panic("Unreachable")
}

// ReloadGroups populates the Group table with groups from the security.Registry
// and refresh all memberships from the database to the security.Registry.
func group_ReloadGroups(rs m.GroupSet) {
	log.Debug("Reloading groups")
	// Sync groups: registry => Database
	var existingGroupIds []string
	for _, group := range security.Registry.AllGroups() {
		existingGroupIds = append(existingGroupIds, group.ID())
		if !h.Group().Search(rs.Env(), q.Group().GroupID().Equals(group.ID())).IsEmpty() {
			// The group already exists in the database
			continue
		}
		rs.WithContext("GroupForceCreate", true).Create(h.Group().NewData().
			SetGroupID(group.ID()).
			SetName(group.Name()))
	}
	// Remove unknown groups from database
	h.Group().Search(rs.Env(), q.Group().GroupID().NotIn(existingGroupIds)).Unlink()
	// Sync memberships: DB => Registry
	allUsers := h.User().NewSet(rs.Env()).SearchAll()
	allUsers.AddMandatoryGroups()
	allUsers.SyncMemberships()
}

func init() {
	models.NewModel("Group")
	h.Group().AddFields(fields_Group)

	h.Group().Methods().Create().Extend(group_Create)
	h.Group().Methods().Write().Extend(group_Write)
	h.Group().NewMethod("ReloadGroups", group_ReloadGroups)
}
