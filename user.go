// Copyright 2016 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/tools/emailutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

// AuthBackend is the authentication backend of the Base module
// Users are authenticated against the User model in the database
type AuthBackend struct{}

// Authenticate the user defined by login and secret.
func (bab *AuthBackend) Authenticate(login, secret string, context *types.Context) (uid int64, err error) {
	models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		uid, err = h.User().NewSet(env).WithNewContext(context).Authenticate(login, secret)
	})
	return
}

var fields_UserChangePasswordWizard = map[string]models.FieldDefinition{
	"Users": fields.One2Many{RelationModel: h.UserChangePasswordWizardLine(),
		ReverseFK: "Wizard", Default: func(env models.Environment) interface{} {
			activeIds := env.Context().GetIntegerSlice("active_ids")
			userLines := h.UserChangePasswordWizardLine().NewSet(env)
			for _, user := range h.User().Search(env, q.User().ID().In(activeIds)).Records() {
				ul := h.UserChangePasswordWizardLine().Create(env, h.UserChangePasswordWizardLine().NewData().
					SetUser(user).
					SetUserLogin(user.Login()).
					SetNewPassword(user.Password()))
				userLines = userLines.Union(ul)
			}
			return userLines
		}},
}

// ChangePasswordButton is called when the user clicks on 'Apply' button in the popup.
// It updates the user's password.`,
func userChangePasswordWizard_ChangePasswordButton(rs m.UserChangePasswordWizardSet) {
	for _, userLine := range rs.Users().Records() {
		userLine.User().SetPassword(userLine.NewPassword())
	}
}

var fields_UserChangePasswordWizardLine = map[string]models.FieldDefinition{
	"Wizard":      fields.Many2One{RelationModel: h.UserChangePasswordWizard()},
	"User":        fields.Many2One{RelationModel: h.User(), OnDelete: models.Cascade, Required: true},
	"UserLogin":   fields.Char{},
	"NewPassword": fields.Char{},
}

var fields_User = map[string]models.FieldDefinition{
	"Partner": fields.Many2One{RelationModel: h.Partner(), Required: true, Embed: true,
		OnDelete: models.Restrict, String: "Related Partner", Help: "Partner-related data of the user"},
	"Login": fields.Char{Required: true, Unique: true, Help: "Used to log into the system",
		OnChange: h.User().Methods().OnchangeLogin()},
	"Password": fields.Char{Default: models.DefaultValue(""), NoCopy: true,
		Help: "Keep empty if you don't want the user to be able to connect on the system."},
	"NewPassword": fields.Char{String: "Set Password", Compute: h.User().Methods().ComputePassword(),
		Inverse: h.User().Methods().InversePassword(), Depends: []string{""},
		Help: `Specify a value only when creating a user or if you're
changing the user's password, otherwise leave empty. After
a change of password, the user has to login again.`},
	"Signature": fields.Text{}, // TODO Switch to HTML field when implemented in client
	"ActionID": fields.Char{GoType: new(actions.ActionRef), String: "Home Action",
		Help: "If specified, this action will be opened at log on for this user, in addition to the standard menu."},
	"Groups": fields.Many2Many{RelationModel: h.Group(), JSON: "group_ids"},
	"Logs": fields.One2Many{RelationModel: h.UserLog(), ReverseFK: "CreateUID", String: "User log entries",
		JSON: "log_ids"},
	"LoginDate": fields.DateTime{Related: "Logs.CreateDate", String: "Latest Connection"},
	"Share": fields.Boolean{Compute: h.User().Methods().ComputeShare(), Depends: []string{"Groups"},
		String: "Share User", Stored: true, Help: "External user with limited access, created only for the purpose of sharing data."},
	"CompaniesCount": fields.Integer{String: "Number of Companies",
		Compute: h.User().Methods().ComputeCompaniesCount(), GoType: new(int)},
	"Company": fields.Many2One{RelationModel: h.Company(), Required: true, Default: func(env models.Environment) interface{} {
		return h.Company().NewSet(env).CompanyDefaultGet()
	}, Help: "The company this user is currently working for.", Constraint: h.User().Methods().CheckCompany()},
	"Companies": fields.Many2Many{RelationModel: h.Company(), JSON: "company_ids", Required: true,
		Default: func(env models.Environment) interface{} {
			return h.Company().NewSet(env).CompanyDefaultGet()
		}, Constraint: h.User().Methods().CheckCompany()},
}

// SelfReadableFields returns the list of its own fields that a user can read.
func user_SelfReadableFields(rs m.UserSet) map[string]bool {
	return map[string]bool{
		"Signature": true, "Company": true, "Login": true, "Email": true, "Name": true, "Image": true,
		"ImageMedium": true, "ImageSmall": true, "Lang": true, "TZ": true, "TZOffset": true, "Groups": true,
		"Partner": true, "LastUpdate": true, "ActionID": true,
	}
}

// SelfWritableFields returns the list of its own fields that a user can write.
func user_SelfWritableFields(rs m.UserSet) map[string]bool {
	return map[string]bool{
		"Signature": true, "ActionID": true, "Company": true, "Email": true, "Name": true,
		"Image": true, "ImageMedium": true, "ImageSmall": true, "Lang": true, "TZ": true,
	}
}

// ComputePassword is a technical function for the new password mechanism. It always returns an empty string
func user_ComputePassword(rs m.UserSet) m.UserData {
	return h.User().NewData().SetNewPassword("")
}

// InversePassword is used in the new password mechanism.
func user_InversePassword(rs m.UserSet, vals models.FieldMapper) {
	if rs.NewPassword() == "" {
		return
	}
	if rs.ID() == rs.Env().Uid() {
		log.Panic(rs.T("Please use the change password wizard (in User Preferences or User menu) to change your own password."))
	}
	rs.SetPassword(rs.NewPassword())
}

// ComputeShare checks if this is a shared user
func user_ComputeShare(rs m.UserSet) m.UserData {
	return h.User().NewData().SetShare(!rs.HasGroup(GroupUser.ID))
}

// ComputeCompaniesCount retrieves the number of companies in the system
func user_ComputeCompaniesCount(rs m.UserSet) m.UserData {
	return h.User().NewData().SetCompaniesCount(h.Company().NewSet(rs.Env()).Sudo().SearchCount())
}

// OnchangeLogin matches the email if the login is an email
func user_OnchangeLogin(rs m.UserSet) m.UserData {
	if rs.Login() == "" || !emailutils.IsValidAddress(rs.Login()) {
		return h.User().NewData()
	}
	return h.User().NewData().SetEmail(rs.Login())
}

// CheckCompany checks that the user's company is one of its authorized companies
func user_CheckCompany(rs m.UserSet) {
	if rs.Company().Intersect(rs.Companies()).IsEmpty() {
		log.Panic(rs.T("The chosen company is not in the allowed companies for this user"))
	}
}

func user_Read(rs m.UserSet, fields models.FieldNames) []models.RecordData {
	rSet := rs
	if len(fields) > 0 && rs.ID() == rs.Env().Uid() {
		var hasUnsafeFields bool
		for _, key := range fields {
			if !rs.SelfReadableFields()[key.Name()] {
				hasUnsafeFields = true
				break
			}
		}
		if !hasUnsafeFields {
			rSet = rs.Sudo()
		}
	}
	result := rSet.Super().Read(fields)
	if !rs.CheckExecutionPermission(h.User().Methods().Write().Underlying(), true) {
		for i, res := range result {
			if res.Underlying().Get(models.ID) != rs.Env().Uid() {
				if res.Underlying().Has(h.User().Fields().Password()) {
					result[i].Underlying().Set(h.User().Fields().Password(), "********")
				}
			}
		}
	}
	return result

}

func user_Search(rs m.UserSet, cond q.UserCondition) m.UserSet {
	if cond.HasField(h.User().Fields().Password()) {
		log.Panic(rs.T("Invalid search criterion: password"))
	}
	return rs.Super().Search(cond)
}

func user_Create(rs m.UserSet, vals m.UserData) m.UserSet {
	user := rs.Super().Create(vals)
	user.Partner().SetActive(user.Active())
	if !user.Partner().Company().IsEmpty() {
		user.Partner().SetCompany(user.Company())
	}
	return user
}

func user_Write(rs m.UserSet, data m.UserData) bool {
	if data.HasActive() && !data.Active() {
		for _, user := range rs.Records() {
			if user.ID() == security.SuperUserID {
				log.Panic(rs.T("You cannot deactivate the admin user."))
			}
			if user.ID() == rs.Env().Uid() {
				log.Panic(rs.T("You cannot deactivate the user you're currently logged in as."))
			}
		}
	}
	rSet := rs
	if rs.ID() == rs.Env().Uid() {
		var hasUnsafeFields bool
		for key := range data.Keys() {
			if !rs.SelfWritableFields()[string(key)] {
				hasUnsafeFields = true
				break
			}
		}
		if !hasUnsafeFields {
			if data.HasCompany() {
				if data.Company().Intersect(h.User().NewSet(rs.Env()).CurrentUser().Companies()).IsEmpty() {
					data.UnsetCompany()
				}
			}
			// safe fields only, so we write as super-user to bypass access rights
			rSet = rs.Sudo()
		}
	}
	res := rSet.Super().Write(data)
	if data.HasGroups() {
		// We get groups before removing all memberships otherwise we might get stuck with permissions if we
		// are modifying our own user memberships.
		rs.SyncMemberships()
	}
	if data.HasCompany() {
		for _, user := range rs.Records() {
			// if partner is global we keep it that way
			if !user.Partner().Company().Equals(data.Company()) {
				user.Partner().SetCompany(user.Company())
			}
		}
	}
	return res
}

func user_Unlink(rs m.UserSet) int64 {
	for _, id := range rs.Ids() {
		if id == security.SuperUserID {
			log.Panic(rs.T("You can not remove the admin user as it is used internally for resources created by Hexya"))
		}
	}
	return rs.Super().Unlink()
}

func user_SearchByName(rs m.UserSet, name string, op operator.Operator, additionalCond q.UserCondition, limit int) m.UserSet {
	if name == "" {
		return rs.Super().SearchByName(name, op, additionalCond, limit)
	}
	users := h.User().NewSet(rs.Env())
	if op == operator.Equals || op == operator.IContains {
		users = h.User().Search(rs.Env(), q.User().Login().Equals(name).AndCond(additionalCond)).Limit(limit)
	}
	if users.IsEmpty() {
		users = h.User().Search(rs.Env(), q.User().Name().AddOperator(op, name).AndCond(additionalCond)).Limit(limit)
	}
	return users
}

func user_Copy(rs m.UserSet, overrides m.UserData) m.UserSet {
	rs.EnsureOne()
	if !overrides.HasName() && !overrides.HasPartner() {
		overrides.SetName(rs.T("%s (copy)", rs.Name()))
	}
	if !overrides.HasLogin() {
		overrides.SetLogin(rs.T("%s (copy)", rs.Login()))
	}
	return rs.Super().Copy(overrides)
}

// ContextGet returns a context with the user's lang, tz and uid
// This method must be called on a singleton.`,
func user_ContextGet(rs m.UserSet) *types.Context {
	rs.EnsureOne()
	res := types.NewContext().
		WithKey("lang", rs.Lang()).
		WithKey("tz", rs.TZ()).
		WithKey("uid", rs.ID()).
		WithKey("company_id", rs.Company().ID())
	return res
}

// ActionGet returns the action for the preferences popup
func user_ActionGet(rs m.UserSet) *actions.Action {
	return actions.Registry.GetByXMLId("base_action_res_users_my")
}

// UpdateLastLogin updates the last login date of the user
func user_UpdateLastLogin(rs m.UserSet) {
	// only create new records to avoid any side-effect on concurrent transactions
	// extra records will be deleted by the periodical garbage collection
	h.UserLog().Create(rs.Env(), h.UserLog().NewData())
}

// CheckCredentials checks that the user defined by its login and secret is allowed to log in.
// It returns the uid of the user on success and an error otherwise.
func user_CheckCredentials(rs m.UserSet, login, secret string) (uid int64, err error) {
	user := rs.Search(q.User().Login().Equals(login))
	if user.Len() == 0 {
		err = security.UserNotFoundError(login)
		return
	}
	if user.Password() == "" || user.Password() != secret {
		err = security.InvalidCredentialsError(login)
		return
	}
	uid = user.ID()
	return
}

// Authenticate the user defined by login and secret
func user_Authenticate(rs m.UserSet, login, secret string) (uid int64, err error) {
	uid, err = rs.CheckCredentials(login, secret)
	if err != nil {
		rs.UpdateLastLogin()
	}
	return
}

// ChangePassword changes current user password. Old password must be provided explicitly
// to prevent hijacking an existing user session, or for cases where the cleartext
// password is not used to authenticate requests. It returns true or panics.
func user_ChangePassword(rs m.UserSet, oldPassword, newPassword string) bool {
	currentUser := h.User().NewSet(rs.Env()).CurrentUser()
	uid, err := rs.CheckCredentials(currentUser.Login(), oldPassword)
	if err != nil || rs.Env().Uid() != uid {
		log.Panic("Invalid password", "user", currentUser.Login(), "uid", uid)
	}
	currentUser.SetPassword(newPassword)
	return true
}

// PreferenceSave is called when validating the preferences popup
func user_PreferenceSave(rs m.UserSet) *actions.Action {
	return &actions.Action{
		Type: actions.ActionClient,
		Tag:  "reload_context",
	}
}

// PreferenceChangePassword is called when clicking 'Change Password' in the preferences popup
func user_PreferenceChangePassword(rs m.UserSet) *actions.Action {
	return &actions.Action{
		Type:   actions.ActionClient,
		Tag:    "change_password",
		Target: "new",
	}
}

// HasGroup returns true if this user belongs to the group with the given ID.
// If this method is called on an empty RecordSet, then it checks if the current
// user belongs to the given group.
func user_HasGroup(rs m.UserSet, groupID string) bool {
	userID := rs.ID()
	if userID == 0 {
		userID = rs.Env().Uid()
	}
	group := security.Registry.GetGroup(groupID)
	return security.Registry.HasMembership(userID, group)
}

// IsAdmin returns true if this user is the administrator or member of the 'Access Rights' group
func user_IsAdmin(rs m.UserSet) bool {
	rs.EnsureOne()
	return rs.IsSuperUser() || rs.HasGroup(GroupERPManager.ID)
}

// IsSuperUser returns true if this user is the administrator
func user_IsSuperUser(rs m.UserSet) bool {
	rs.EnsureOne()
	return rs.ID() == security.SuperUserID
}

// AddMandatoryGroups adds the group Everyone to everybody and the admin group to the admin
func user_AddMandatoryGroups(rs m.UserSet) {
	for _, user := range rs.Records() {
		dbGroupEveryone := h.Group().Search(rs.Env(), q.Group().GroupID().Equals(security.GroupEveryoneID))
		dbGroupAdmin := h.Group().Search(rs.Env(), q.Group().GroupID().Equals(security.GroupAdminID))
		groups := user.Groups()
		// Add groupAdmin for admin
		if user.ID() == security.SuperUserID {
			groups = groups.Union(dbGroupAdmin)
		}
		// Add groupEveryone if not already the case
		groups = groups.Union(dbGroupEveryone)

		user.SetGroups(groups)
	}
}

// SyncMemberships synchronises the users memberships with the Hexya internal registry
func user_SyncMemberships(rs m.UserSet) {
	for _, user := range rs.Records() {
		if user.CheckGroupsSync() {
			continue
		}
		log.Debug("Updating user groups", "user", rs.Name(), "uid", rs.ID(), "groups", rs.Groups())
		// Push memberships to registry
		security.Registry.RemoveAllMembershipsForUser(user.ID())
		for _, dbGroup := range user.Groups().Records() {
			security.Registry.AddMembership(user.ID(), security.Registry.GetGroup(dbGroup.GroupID()))
		}
	}
}

// CheckGroupSync returns true if the groups in the internal registry match exactly
// database groups of the given users. This method must be called on a singleton
func user_CheckGroupSync(rs m.UserSet) bool {
	rs.EnsureOne()
dbLoop:
	for _, dbGroup := range rs.Groups().Records() {
		for grp := range security.Registry.UserGroups(rs.ID()) {
			if grp.ID == dbGroup.GroupID() {
				continue dbLoop
			}
		}
		return false
	}
rLoop:
	for grp := range security.Registry.UserGroups(rs.ID()) {
		for _, dbGroup := range rs.Groups().Records() {
			if grp.ID == dbGroup.GroupID() {
				continue rLoop
			}
		}
		return false
	}
	return true
}

// GetCompany returns the current user's company.
func user_GetCompany(rs m.UserSet) m.CompanySet {
	return h.User().NewSet(rs.Env()).CurrentUser().Company()
}

// GetCompanyCurrency returns the currency of the current user's company.
func user_GetCompanyCurrency(rs m.UserSet) m.CurrencySet {
	return h.User().NewSet(rs.Env()).CurrentUser().Company().Currency()
}

// CurrentUser returns a UserSet with the currently logged in user.
func user_CurrentUser(rs m.UserSet) m.UserSet {
	return h.User().Browse(rs.Env(), []int64{rs.Env().Uid()})
}

// init
func init() {
	models.NewTransientModel("UserChangePasswordWizard")
	h.UserChangePasswordWizard().AddFields(fields_UserChangePasswordWizard)
	h.UserChangePasswordWizard().NewMethod("ChangePasswordButton", userChangePasswordWizard_ChangePasswordButton)

	models.NewTransientModel("UserChangePasswordWizardLine")
	h.UserChangePasswordWizardLine().AddFields(fields_UserChangePasswordWizardLine)

	models.NewModel("UserLog")
	h.UserLog().SetDefaultOrder("id desc")

	models.NewModel("User")
	h.User().SetDefaultOrder("Login")
	h.User().AddFields(fields_User)

	h.User().NewMethod("SelfReadableFields", user_SelfReadableFields)
	h.User().NewMethod("SelfWritableFields", user_SelfWritableFields)
	h.User().NewMethod("ComputePassword", user_ComputePassword)
	h.User().NewMethod("InversePassword", user_InversePassword)
	h.User().NewMethod("ComputeShare", user_ComputeShare)
	h.User().NewMethod("ComputeCompaniesCount", user_ComputeCompaniesCount)
	h.User().NewMethod("OnchangeLogin", user_OnchangeLogin)
	h.User().NewMethod("CheckCompany", user_CheckCompany)
	h.User().Methods().Read().Extend(user_Read)
	h.User().Methods().Search().Extend(user_Search)
	h.User().Methods().Create().Extend(user_Create)
	h.User().Methods().Write().Extend(user_Write)
	h.User().Methods().Unlink().Extend(user_Unlink)
	h.User().Methods().SearchByName().Extend(user_SearchByName)
	h.User().Methods().Copy().Extend(user_Copy)
	h.User().NewMethod("ContextGet", user_ContextGet)
	h.User().NewMethod("ActionGet", user_ActionGet)
	h.User().NewMethod("UpdateLastLogin", user_UpdateLastLogin)
	h.User().NewMethod("CheckCredentials", user_CheckCredentials)
	h.User().NewMethod("Authenticate", user_Authenticate)
	h.User().NewMethod("ChangePassword", user_ChangePassword)
	h.User().NewMethod("PreferenceSave", user_PreferenceSave)
	h.User().NewMethod("PreferenceChangePassword", user_PreferenceChangePassword)
	h.User().NewMethod("HasGroup", user_HasGroup)
	h.User().NewMethod("IsAdmin", user_IsAdmin)
	h.User().NewMethod("IsSuperUser", user_IsSuperUser)
	h.User().NewMethod("AddMandatoryGroups", user_AddMandatoryGroups)
	h.User().NewMethod("SyncMemberships", user_SyncMemberships)
	h.User().NewMethod("CheckGroupsSync", user_CheckGroupSync)
	h.User().NewMethod("GetCompany", user_GetCompany)
	h.User().NewMethod("GetCompanyCurrency", user_GetCompanyCurrency)
	h.User().NewMethod("CurrentUser", user_CurrentUser)

	security.AuthenticationRegistry.RegisterBackend(new(AuthBackend))
}
