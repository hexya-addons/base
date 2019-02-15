// Copyright 2016 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/tools/emailutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

// BaseAuthBackend is the authentication backend of the Base module
// Users are authenticated against the User model in the database
type BaseAuthBackend struct{}

// Authenticate the user defined by login and secret.
func (bab *BaseAuthBackend) Authenticate(login, secret string, context *types.Context) (uid int64, err error) {
	models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		uid, err = h.User().NewSet(env).WithNewContext(context).Authenticate(login, secret)
	})
	return
}

func init() {
	cpWizard := h.UserChangePasswordWizard().DeclareTransientModel()
	cpWizard.AddFields(map[string]models.FieldDefinition{
		"Users": models.One2ManyField{RelationModel: h.UserChangePasswordWizardLine(),
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
	})

	cpWizard.Methods().ChangePasswordButton().DeclareMethod(
		`ChangePasswordButton is called when the user clicks on 'Apply' button in the popup.
		It updates the user's password.`,
		func(rs m.UserChangePasswordWizardSet) {
			for _, userLine := range rs.Users().Records() {
				userLine.User().SetPassword(userLine.NewPassword())
			}
		})

	cpWizardLine := h.UserChangePasswordWizardLine().DeclareTransientModel()
	cpWizardLine.AddFields(map[string]models.FieldDefinition{
		"Wizard":      models.Many2OneField{RelationModel: h.UserChangePasswordWizard()},
		"User":        models.Many2OneField{RelationModel: h.User(), OnDelete: models.Cascade, Required: true},
		"UserLogin":   models.CharField{},
		"NewPassword": models.CharField{},
	})

	userLogModel := h.UserLog().DeclareModel()
	userLogModel.SetDefaultOrder("id desc")

	userModel := h.User().DeclareModel()
	userModel.SetDefaultOrder("Login")
	userModel.AddFields(map[string]models.FieldDefinition{
		"Partner": models.Many2OneField{RelationModel: h.Partner(), Required: true, Embed: true,
			OnDelete: models.Restrict, String: "Related Partner", Help: "Partner-related data of the user"},
		"Login": models.CharField{Required: true, Unique: true, Help: "Used to log into the system",
			OnChange: h.User().Methods().OnchangeLogin()},
		"Password": models.CharField{Default: models.DefaultValue(""), NoCopy: true,
			Help: "Keep empty if you don't want the user to be able to connect on the system."},
		"NewPassword": models.CharField{String: "Set Password", Compute: h.User().Methods().ComputePassword(),
			Inverse: h.User().Methods().InversePassword(), Depends: []string{""},
			Help: `Specify a value only when creating a user or if you're
changing the user's password, otherwise leave empty. After
a change of password, the user has to login again.`},
		"Signature": models.TextField{}, // TODO Switch to HTML field when implemented in client
		"ActionID": models.CharField{GoType: new(actions.ActionRef), String: "Home Action",
			Help: "If specified, this action will be opened at log on for this user, in addition to the standard menu."},
		"Groups": models.Many2ManyField{RelationModel: h.Group(), JSON: "group_ids"},
		"Logs": models.One2ManyField{RelationModel: h.UserLog(), ReverseFK: "CreateUID", String: "User log entries",
			JSON: "log_ids"},
		"LoginDate": models.DateTimeField{Related: "Logs.CreateDate", String: "Latest Connection"},
		"Share": models.BooleanField{Compute: h.User().Methods().ComputeShare(), Depends: []string{"Groups"},
			String: "Share User", Stored: true, Help: "External user with limited access, created only for the purpose of sharing data."},
		"CompaniesCount": models.IntegerField{String: "Number of Companies",
			Compute: h.User().Methods().ComputeCompaniesCount(), GoType: new(int)},
		"Company": models.Many2OneField{RelationModel: h.Company(), Required: true, Default: func(env models.Environment) interface{} {
			return h.Company().NewSet(env).CompanyDefaultGet()
		}, Help: "The company this user is currently working for.", Constraint: h.User().Methods().CheckCompany()},
		"Companies": models.Many2ManyField{RelationModel: h.Company(), JSON: "company_ids", Required: true,
			Default: func(env models.Environment) interface{} {
				return h.Company().NewSet(env).CompanyDefaultGet()
			}, Constraint: h.User().Methods().CheckCompany()},
	})

	userModel.Methods().SelfReadableFields().DeclareMethod(
		`SelfReadableFields returns the list of its own fields that a user can read.`,
		func(rs m.UserSet) map[string]bool {
			return map[string]bool{
				"Signature": true, "Company": true, "Login": true, "Email": true, "Name": true, "Image": true,
				"ImageMedium": true, "ImageSmall": true, "Lang": true, "TZ": true, "TZOffset": true, "Groups": true,
				"Partner": true, "LastUpdate": true, "ActionID": true,
			}
		})

	userModel.Methods().SelfWritableFields().DeclareMethod(
		`SelfWritableFields returns the list of its own fields that a user can write.`,
		func(rs m.UserSet) map[string]bool {
			return map[string]bool{
				"Signature": true, "ActionID": true, "Company": true, "Email": true, "Name": true,
				"Image": true, "ImageMedium": true, "ImageSmall": true, "Lang": true, "TZ": true,
			}
		})

	userModel.Methods().ComputePassword().DeclareMethod(
		`ComputePassword is a technical function for the new password mechanism. It always returns an empty string`,
		func(rs m.UserSet) m.UserData {
			return h.User().NewData().SetNewPassword("")
		})

	userModel.Methods().InversePassword().DeclareMethod(
		`InversePassword is used in the new password mechanism.`,
		func(rs m.UserSet, vals models.FieldMapper) {
			if rs.NewPassword() == "" {
				return
			}
			if rs.ID() == rs.Env().Uid() {
				log.Panic(rs.T("Please use the change password wizard (in User Preferences or User menu) to change your own password."))
			}
			rs.SetPassword(rs.NewPassword())
		})

	userModel.Methods().ComputeShare().DeclareMethod(
		`ComputeShare checks if this is a shared user`,
		func(rs m.UserSet) m.UserData {
			return h.User().NewData().SetShare(!rs.HasGroup(GroupUser.ID))
		})

	userModel.Methods().ComputeCompaniesCount().DeclareMethod(
		`ComputeCompaniesCount retrieves the number of companies in the system`,
		func(rs m.UserSet) m.UserData {
			return h.User().NewData().SetCompaniesCount(h.Company().NewSet(rs.Env()).Sudo().SearchCount())
		})

	userModel.Methods().OnchangeLogin().DeclareMethod(
		`OnchangeLogin matches the email if the login is an email`,
		func(rs m.UserSet) m.UserData {
			if rs.Login() == "" || !emailutils.IsValidAddress(rs.Login()) {
				return h.User().NewData()
			}
			return h.User().NewData().SetEmail(rs.Login())
		})

	userModel.Methods().CheckCompany().DeclareMethod(
		`CheckCompany checks that the user's company is one of its authorized companies`,
		func(rs m.UserSet) {
			if rs.Company().Intersect(rs.Companies()).IsEmpty() {
				log.Panic(rs.T("The chosen company is not in the allowed companies for this user"))
			}
		})

	userModel.Methods().Read().Extend("",
		func(rs m.UserSet, fields []string) []models.FieldMap {
			rSet := rs
			if len(fields) > 0 && rs.ID() == rs.Env().Uid() {
				var hasUnsafeFields bool
				for _, key := range fields {
					if !rs.SelfReadableFields()[key] {
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
					if res["id"] != rs.Env().Uid() {
						if _, exists := res["password"]; exists {
							result[i]["password"] = "********"
						}
					}
				}
			}
			return result

		})

	userModel.Methods().Search().Extend("",
		func(rs m.UserSet, cond q.UserCondition) m.UserSet {
			if cond.HasField(h.User().Fields().Password()) {
				log.Panic(rs.T("Invalid search criterion: password"))
			}
			return rs.Super().Search(cond)
		})

	userModel.Methods().Create().Extend("",
		func(rs m.UserSet, vals m.UserData) m.UserSet {
			user := rs.Super().Create(vals)
			user.Partner().SetActive(user.Active())
			if !user.Partner().Company().IsEmpty() {
				user.Partner().SetCompany(user.Company())
			}
			return user
		})

	userModel.Methods().Write().Extend("",
		func(rs m.UserSet, data m.UserData) bool {
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
		})

	userModel.Methods().Unlink().Extend("",
		func(rs m.UserSet) int64 {
			for _, id := range rs.Ids() {
				if id == security.SuperUserID {
					log.Panic(rs.T("You can not remove the admin user as it is used internally for resources created by Hexya"))
				}
			}
			return rs.Super().Unlink()
		})

	userModel.Methods().SearchByName().Extend("",
		func(rs m.UserSet, name string, op operator.Operator, additionalCond q.UserCondition, limit int) m.UserSet {
			if name == "" {
				return rs.Super().SearchByName(name, op, additionalCond, limit)
			}
			var users m.UserSet
			if op == operator.Equals || op == operator.IContains {
				users = h.User().Search(rs.Env(), q.User().Login().Equals(name).AndCond(additionalCond)).Limit(limit)
			}
			if users.IsEmpty() {
				users = h.User().Search(rs.Env(), q.User().Name().AddOperator(op, name).AndCond(additionalCond)).Limit(limit)
			}
			return users
		})

	userModel.Methods().Copy().Extend("",
		func(rs m.UserSet, overrides m.UserData) m.UserSet {
			rs.EnsureOne()
			if !overrides.HasName() && !overrides.HasPartner() {
				overrides.SetName(rs.T("%s (copy)", rs.Name()))
			}
			if !overrides.HasLogin() {
				overrides.SetLogin(rs.T("%s (copy)", rs.Login()))
			}
			return rs.Super().Copy(overrides)
		})

	userModel.Methods().ContextGet().DeclareMethod(
		`UsersContextGet returns a context with the user's lang, tz and uid
		This method must be called on a singleton.`,
		func(rs m.UserSet) *types.Context {
			rs.EnsureOne()
			res := types.NewContext().
				WithKey("lang", rs.Lang()).
				WithKey("tz", rs.TZ()).
				WithKey("uid", rs.ID()).
				WithKey("company_id", rs.Company().ID())
			return res
		})

	userModel.Methods().ActionGet().DeclareMethod(
		`ActionGet returns the action for the preferences popup`,
		func(rs m.UserSet) *actions.Action {
			return actions.Registry.GetById("base_action_res_users_my")
		})

	userModel.Methods().UpdateLastLogin().DeclareMethod(
		`UpdateLastLogin updates the last login date of the user`,
		func(rs m.UserSet) {
			// only create new records to avoid any side-effect on concurrent transactions
			// extra records will be deleted by the periodical garbage collection
			h.UserLog().Create(rs.Env(), h.UserLog().NewData())
		})

	userModel.Methods().CheckCredentials().DeclareMethod(
		`CheckCredentials checks that the user defined by its login and secret is allowed to log in.
		It returns the uid of the user on success and an error otherwise.`,
		func(rs m.UserSet, login, secret string) (uid int64, err error) {
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
		})

	userModel.Methods().Authenticate().DeclareMethod(
		"Authenticate the user defined by login and secret",
		func(rs m.UserSet, login, secret string) (uid int64, err error) {
			uid, err = rs.CheckCredentials(login, secret)
			if err != nil {
				rs.UpdateLastLogin()
			}
			return
		})

	userModel.Methods().ChangePassword().DeclareMethod(
		`ChangePassword changes current user password. Old password must be provided explicitly
        to prevent hijacking an existing user session, or for cases where the cleartext
        password is not used to authenticate requests. It returns true or panics.`,
		func(rs m.UserSet, oldPassword, newPassword string) bool {
			currentUser := h.User().NewSet(rs.Env()).CurrentUser()
			uid, err := rs.CheckCredentials(currentUser.Login(), oldPassword)
			if err != nil || rs.Env().Uid() != uid {
				log.Panic("Invalid password", "user", currentUser.Login(), "uid", uid)
			}
			currentUser.SetPassword(newPassword)
			return true
		})

	userModel.Methods().PreferenceSave().DeclareMethod(
		`PreferenceSave is called when validating the preferences popup`,
		func(rs m.UserSet) *actions.Action {
			return &actions.Action{
				Type: actions.ActionClient,
				Tag:  "reload_context",
			}
		})

	userModel.Methods().PreferenceChangePassword().DeclareMethod(
		`PreferenceChangePassword is called when clicking 'Change Password' in the preferences popup`,
		func(rs m.UserSet) *actions.Action {
			return &actions.Action{
				Type:   actions.ActionClient,
				Tag:    "change_password",
				Target: "new",
			}
		})

	userModel.Methods().HasGroup().DeclareMethod(
		`HasGroup returns true if this user belongs to the group with the given ID.
		If this method is called on an empty RecordSet, then it checks if the current
		user belongs to the given group.`,
		func(rs m.UserSet, groupID string) bool {
			userID := rs.ID()
			if userID == 0 {
				userID = rs.Env().Uid()
			}
			group := security.Registry.GetGroup(groupID)
			return security.Registry.HasMembership(userID, group)
		})

	userModel.Methods().IsAdmin().DeclareMethod(
		`IsAdmin returns true if this user is the administrator or member of the 'Access Rights' group`,
		func(rs m.UserSet) bool {
			rs.EnsureOne()
			return rs.IsSuperUser() || rs.HasGroup(GroupERPManager.ID)
		})

	userModel.Methods().IsSuperUser().DeclareMethod(
		`IsSuperUser returns true if this user is the administrator`,
		func(rs m.UserSet) bool {
			rs.EnsureOne()
			return rs.ID() == security.SuperUserID
		})

	userModel.Methods().AddMandatoryGroups().DeclareMethod(
		`AddMandatoryGroups adds the group Everyone to everybody and the admin group to the admin`,
		func(rs m.UserSet) {
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
		})

	userModel.Methods().SyncMemberships().DeclareMethod(
		`SyncMemberships synchronises the users memberships with the Hexya internal registry`,
		func(rs m.UserSet) {
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
		})

	userModel.Methods().CheckGroupsSync().DeclareMethod(
		`CheckGroupSync returns true if the groups in the internal registry match exactly
		database groups of the given users. This method must be called on a singleton`,
		func(rs m.UserSet) bool {
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
		})

	userModel.Methods().GetCompany().DeclareMethod(
		`GetCompany returns the current user's company.`,
		func(rs m.UserSet) m.CompanySet {
			return h.User().NewSet(rs.Env()).CurrentUser().Company()
		})

	userModel.Methods().GetCompanyCurrency().DeclareMethod(
		`GetCompanyCurrency returns the currency of the current user's company.`,
		func(rs m.UserSet) m.CurrencySet {
			return h.User().NewSet(rs.Env()).CurrentUser().Company().Currency()
		})

	userModel.Methods().CurrentUser().DeclareMethod(
		`CurrentUser returns a UserSet with the currently logged in user.`,
		func(rs m.UserSet) m.UserSet {
			return h.User().Browse(rs.Env(), []int64{rs.Env().Uid()})
		})

	security.AuthenticationRegistry.RegisterBackend(new(BaseAuthBackend))

}
