// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
)

var (
	// GroupSystem is given to users who are allowed to modify general settings
	GroupSystem *security.Group
	// GroupUser is the group for Employees
	GroupUser *security.Group
	// GroupMultiCurrency displays data to work in a multi-currency context
	GroupMultiCurrency *security.Group
	// GroupPartnerManager is given to users who are allowed to manage contacts
	GroupPartnerManager *security.Group
	// GroupMultiCompany displays data to work in a multi-company context
	GroupMultiCompany *security.Group
	// GroupPortal is granted to portal users
	GroupPortal *security.Group
	// GroupPublic is granted to external users
	GroupPublic *security.Group
	// GroupERPManager can modify access rights for other users
	GroupERPManager *security.Group
	// GroupTechnicalFeatures can see and modify technical parameters of the ERP
	GroupTechnicalFeatures *security.Group
)

func init() {
	GroupERPManager = security.Registry.NewGroup("base_group_erp_manager", "Access Rights")
	GroupSystem = security.Registry.NewGroup("base_group_system", "Settings", GroupERPManager)
	GroupUser = security.Registry.NewGroup("base_group_user", "Employee")
	GroupMultiCompany = security.Registry.NewGroup("base_group_multi_company", "Multi Companies")
	GroupMultiCurrency = security.Registry.NewGroup("base_group_multi_currency", "Multi Currencies")
	GroupTechnicalFeatures = security.Registry.NewGroup("base_group_no_one", "Technical Features")
	GroupPartnerManager = security.Registry.NewGroup("base_group_partner_manager", "Contact Creation")
	GroupPortal = security.Registry.NewGroup("base_group_portal", "Portal")
	GroupPublic = security.Registry.NewGroup("base_group_public", "Public")

	h.Attachment().Methods().Load().AllowGroup(security.GroupEveryone)
	h.Attachment().Methods().AllowAllToGroup(GroupUser)

	h.User().Methods().Load().AllowGroup(security.GroupEveryone)
	h.User().Methods().HasGroup().AllowGroup(security.GroupEveryone)
	h.User().Methods().AllowAllToGroup(GroupERPManager)

	h.CurrencyRate().Methods().Load().AllowGroup(security.GroupEveryone)
	h.CurrencyRate().Methods().AllowAllToGroup(GroupSystem)

	h.Currency().Methods().Load().AllowGroup(security.GroupEveryone)
	h.Currency().Methods().AllowAllToGroup(GroupSystem)

	h.Partner().Methods().Load().AllowGroup(GroupPublic)
	h.Partner().Methods().Load().AllowGroup(GroupPortal)
	h.Partner().Methods().Load().AllowGroup(GroupUser)
	h.Partner().Methods().AllowAllToGroup(GroupPartnerManager)

	h.PartnerTitle().Methods().Load().AllowGroup(security.GroupEveryone)
	h.PartnerTitle().Methods().AllowAllToGroup(GroupPartnerManager)

	h.PartnerCategory().Methods().Load().AllowGroup(GroupUser)
	h.PartnerCategory().Methods().AllowAllToGroup(GroupPartnerManager)

	h.BankAccount().Methods().Load().AllowGroup(GroupUser)
	h.BankAccount().Methods().AllowAllToGroup(GroupPartnerManager)

	h.Bank().Methods().AllowAllToGroup(GroupSystem)
	h.Bank().Methods().AllowAllToGroup(GroupPartnerManager)
	h.Bank().Methods().Load().AllowGroup(GroupUser)

	h.Company().Methods().Load().AllowGroup(security.GroupEveryone, h.User().Methods().ContextGet())

}
