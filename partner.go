// Copyright 2016 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"image/color"
	"io/ioutil"
	"net/http"
	"net/mail"
	"net/url"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/hexya-addons/base/basetypes"
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/i18n"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/fieldtype"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/server"
	"github.com/hexya-erp/hexya/src/tools/b64image"
	"github.com/hexya-erp/hexya/src/tools/typesutils"
	"github.com/hexya-erp/hexya/src/views"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

const gravatarBaseURL = "https://www.gravatar.com/avatar"

var (
	// WarningMessage gives the possible warning that can be applied to a partner.
	WarningMessage = types.Selection{
		"no-message": "No Message",
		"warning":    "Warning",
		"block":      "Blocking Message",
	}
	// WarningHelp is the help message for the Warning fields.
	WarningHelp = `Selecting the "Warning" option will notify user with the message,
Selecting "Blocking Message" will throw an exception with the message and block the flow.
The Message has to be written in the next field.`
)

var fields_PartnerTitle = map[string]models.FieldDefinition{
	"Name":     fields.Char{String: "Title", Required: true, Translate: true, Unique: true},
	"Shortcut": fields.Char{String: "Abbreviation", Translate: true},
}

var fields_PartnerCategory = map[string]models.FieldDefinition{
	"Name":  fields.Char{String: "Tag Name", Required: true, Translate: true},
	"Color": fields.Integer{String: "Color Index"},
	"Parent": fields.Many2One{RelationModel: h.PartnerCategory(),
		String: "Parent Tag", Index: true, OnDelete: models.Cascade},
	"Children": fields.One2Many{RelationModel: h.PartnerCategory(), Copy: true,
		ReverseFK: "Parent", String: "Children Tags"},
	"Active": fields.Boolean{Default: models.DefaultValue(true), Required: true,
		Help: "The active field allows you to hide the category without removing it."},
	"Partners": fields.Many2Many{RelationModel: h.Partner()},
}

// CheckParent checks if we have a recursion in the parent tree.
func partnerCategory_CheckParent(rs m.PartnerCategorySet) {
	if !rs.CheckRecursion() {
		log.Panic(rs.T("Error ! You can not create recursive tags."))
	}
}

func partnerCategory_NAmeGet(rs m.PartnerCategorySet) string {
	if rs.Env().Context().GetString("partner_category_display") == "short" {
		return rs.Super().NameGet()
	}
	var names []string

	for current := rs; !current.IsEmpty(); current = current.Parent() {
		names = append([]string{current.Name()}, names...)
	}
	return strings.Join(names, " / ")
}

func partnerCategory_SearchByName(rs m.PartnerCategorySet, name string, op operator.Operator, additionalCond q.PartnerCategoryCondition, limit int) m.PartnerCategorySet {
	if name != "" {
		tokens := strings.Split(name, " / ")
		name = tokens[len(tokens)-1]
	}
	return rs.Super().SearchByName(name, op, additionalCond, limit)
}

var fields_Partner = map[string]models.FieldDefinition{
	"Name":  fields.Char{Required: true, Index: true, NoCopy: true},
	"Date":  fields.Date{Index: true},
	"Title": fields.Many2One{RelationModel: h.PartnerTitle()},
	"Parent": fields.Many2One{RelationModel: h.Partner(), Index: true,
		Constraint: h.Partner().Methods().CheckParent(), OnChange: h.Partner().Methods().OnchangeParent(),
		OnChangeWarning: h.Partner().Methods().OnchangeParentWarning(),
	},
	"ParentName": fields.Char{Related: "Parent.Name", ReadOnly: true},

	"Children": fields.One2Many{RelationModel: h.Partner(),
		ReverseFK: "Parent", Filter: q.Partner().Active().Equals(true)},
	"Ref": fields.Char{String: "Internal Reference", Index: true},
	"Lang": fields.Selection{
		String: "Language",
		Default: func(env models.Environment) interface{} {
			return env.Context().GetString("lang")
		},
		SelectionFunc: func() types.Selection {
			out := make(types.Selection)
			for _, lang := range i18n.Langs {
				l := i18n.GetLocale(lang)
				out[lang] = l.Name
			}
			return out
		},
		Help: `If the selected language is loaded in the system, all documents related to
this contact will be printed in this language. If not, it will be English.`},
	"TZ": fields.Char{
		String: "Timezone",
		Default: func(env models.Environment) interface{} {
			return env.Context().GetString("tz")
		},
		Help: `"The partner's timezone, used to output proper date and time values
inside printed reports. It is important to set a value for this field.
You should use the same timezone that is otherwise used to pick and
render date and time values: your computer's timezone.`},
	"TZOffset": fields.Char{
		Compute: h.Partner().Methods().ComputeTZOffset(),
		String:  "Timezone Offset", Depends: []string{"TZ"}},
	"User": fields.Many2One{
		RelationModel: h.User(),
		String:        "Salesperson", Help: "The internal user that is in charge of communicating with this contact if any."},
	"VAT": fields.Char{String: "TIN", Help: `Tax Identification Number.
Fill it if the company is subjected to taxes.
Used by the some of the legal statements.`},
	"Banks": fields.One2Many{
		String: "Bank Accounts", RelationModel: h.BankAccount(), ReverseFK: "Partner"},
	"Website": fields.Char{
		Help: "Website of Partner or Company"},
	"Comment": fields.Char{
		String: "Notes"},
	"Categories": fields.Many2Many{
		RelationModel: h.PartnerCategory(), String: "Tags",
		Default: func(env models.Environment) interface{} {
			return h.PartnerCategory().Browse(env, []int64{env.Context().GetInteger("category_id")})
		}},
	"CreditLimit": fields.Float{},
	"Barcode":     fields.Char{},
	"Active":      fields.Boolean{Required: true, Default: models.DefaultValue(true)},
	"Customer": fields.Boolean{String: "Is a Customer", Default: models.DefaultValue(true),
		Help: "Check this box if this contact is a customer."},
	"Supplier": fields.Boolean{String: "Is a Vendor",
		Help: `Check this box if this contact is a vendor.
If it's not checked, purchase people will not see it when encoding a purchase order.`},
	"Employee": fields.Boolean{Help: "Check this box if this contact is an Employee."},
	"Function": fields.Char{String: "Job Position"},
	"Type": fields.Selection{
		Selection: types.Selection{
			"contact":  "Contact",
			"invoice":  "Invoice Address",
			"delivery": "Shipping Address",
			"other":    "Other Address"},
		Default: models.DefaultValue("contact"), Required: true,
		Help: "Used to select automatically the right address according to the context in sales and purchases documents.",
	},
	"Street":  fields.Char{},
	"Street2": fields.Char{},
	"Zip":     fields.Char{},
	"City":    fields.Char{},
	"State": fields.Many2One{RelationModel: h.CountryState(),
		Filter: q.CountryState().Country().EqualsEval("country_id"), OnDelete: models.Restrict},
	"Country": fields.Many2One{RelationModel: h.Country(),
		OnDelete: models.Restrict, OnChangeFilters: h.Partner().Methods().OnchangeCountryFilters()},
	"Email": fields.Char{OnChange: h.Partner().Methods().OnchangeEmail()},
	"EmailFormatted": fields.Char{Compute: h.Partner().Methods().ComputeEmailFormatted(),
		Help: "Formatted email address 'Name <email@domain>'", Depends: []string{"Name", "Email"}},
	"Phone":  fields.Char{},
	"Fax":    fields.Char{},
	"Mobile": fields.Char{},
	"IsCompany": fields.Boolean{Default: models.DefaultValue(false),
		Help: "Check if the contact is a company, otherwise it is a person"},
	// CompanyType is only an interface field, do not use it in business logic
	"CompanyType": fields.Selection{
		Selection: types.Selection{"person": "Individual", "company": "Company"},
		Compute:   h.Partner().Methods().ComputeCompanyType(),
		Depends:   []string{"IsCompany"}, Inverse: h.Partner().Methods().InverseCompanyType(),
		OnChange: h.Partner().Methods().OnchangeCompanyType(),
		Default:  models.DefaultValue("person")},
	"Company": fields.Many2One{RelationModel: h.Company()},
	"Color":   fields.Integer{},
	"Users":   fields.One2Many{RelationModel: h.User(), ReverseFK: "Partner"},
	"PartnerShare": fields.Boolean{String: "Share Partner",
		Compute: h.Partner().Methods().ComputePartnerShare(), Stored: true, Depends: []string{"Users", "Users.Share"},
		Help: `Either customer (no user), either shared user. Indicated the current partner is a customer without
access or with a limited access created for sharing data.`},
	"ContactAddress": fields.Char{Compute: h.Partner().Methods().ComputeContactAddress(),
		String: "Complete Address", Depends: []string{"Street", "Street2", "Zip", "City", "State", "Country",
			"Country.AddressFormat", "Country.Code", "Country.Name", "CompanyName", "State.Code", "State.Name"}},
	"CommercialPartner": fields.Many2One{RelationModel: h.Partner(),
		Compute: h.Partner().Methods().ComputeCommercialPartner(), String: "Commercial Entity", Stored: true,
		Index: true, Depends: []string{"IsCompany", "Parent", "Parent.CommercialPartner"}},
	"CommercialCompanyName": fields.Char{
		Compute: h.Partner().Methods().ComputeCommercialCompanyName(), Stored: true,
		Depends: []string{"CompanyName", "Parent", "Parent.IsCompany", "CommercialPartner", "CommercialPartner.Name"}},
	"CompanyName": fields.Char{},

	"Image": fields.Binary{
		Help: "This field holds the image used as avatar for this contact, limited to 1024x1024px"},
	"ImageMedium": fields.Binary{
		Help: `Medium-sized image of this contact. It is automatically
resized as a 128x128px image, with aspect ratio preserved.
Use this field in form views or some kanban views.`},
	"ImageSmall": fields.Binary{
		Help: `Small-sized image of this contact. It is automatically
resized as a 64x64px image, with aspect ratio preserved.
Use this field anywhere a small image is required.`},
}

func partner_ComputeDisplayName(rs m.PartnerSet) *models.ModelData {
	rSet := rs.
		WithContext("show_address", false).
		WithContext("show_address_only", false).
		WithContext("show_email", false)
	return rSet.Super().ComputeDisplayName()
}

// ComputeTZOffset computes the timezone offset
func partner_ComputeTZOffset(rs m.PartnerSet) m.PartnerData {
	// TODO Implement TZOffset
	return h.Partner().NewData().SetTZOffset("")
}

// ComputePartnerShare computes the PartnerShare field
func partner_ComputePartnerShare(rs m.PartnerSet) m.PartnerData {
	var partnerShare bool
	if rs.Users().IsEmpty() {
		partnerShare = true
	}
	for _, user := range rs.Users().Records() {
		if user.Share() {
			partnerShare = true
			break
		}
	}
	return h.Partner().NewData().SetPartnerShare(partnerShare)
}

// ComputeContactAddress computes the contact's address according to the contact's country standards
func partner_ComputeContactAddress(rs m.PartnerSet) m.PartnerData {
	return h.Partner().NewData().SetContactAddress(rs.DisplayAddress(false))
}

// ComputeCommercialPartner computes the commercial partner, which is the first company ancestor or the top
// ancestor if none are companies`,
func partner_ComputeCommercialPartner(rs m.PartnerSet) m.PartnerData {
	commercialPartner := rs
	if !rs.IsCompany() && !rs.Parent().IsEmpty() {
		commercialPartner = rs.Parent().CommercialPartner()
	}
	return h.Partner().NewData().SetCommercialPartner(commercialPartner)
}

// ComputeCommercialCompanyName returns the name of the commercial partner company
func partner_ComputeCommercialCompanyName(rs m.PartnerSet) m.PartnerData {
	commPartnerName := rs.CommercialPartner().Name()
	if !rs.CommercialPartner().IsCompany() {
		commPartnerName = rs.CompanyName()
	}
	return h.Partner().NewData().SetCommercialCompanyName(commPartnerName)
}

// GetDefaultImage returns a default image for the partner (base64 encoded)
func partner_GetDefaultImage(rs m.PartnerSet, partnerType string, isCompany bool, Parent m.PartnerSet) string {
	if rs.Env().Context().HasKey("install_mode") {
		return ""
	}
	var img string
	if partnerType == "other" && !Parent.IsEmpty() {
		parentImage := Parent.Image()
		if parentImage != "" {
			img = parentImage
		}
	}
	if img == "" {
		var (
			colorize    bool
			imgFileName string
		)
		switch {
		case partnerType == "invoice":
			imgFileName = "money.png"
		case partnerType == "delivery":
			imgFileName = "truck.png"
		case isCompany:
			imgFileName = "company_image.png"
		default:
			imgFileName = "avatar.png"
			colorize = true
		}
		path := filepath.Join(server.ResourceDir, "static", "base", "src", "img", imgFileName)
		content, err := ioutil.ReadFile(path)
		if err != nil {
			log.Warn("error while loading ressource", "image", path, "error", err)
		}
		img = base64.StdEncoding.EncodeToString(content)
		if colorize {
			img = b64image.Colorize(img, color.RGBA{})
		}
	}
	return img
}

// CheckParent checks for recursion in the partners parenthood
func partner_CheckParent(rs m.PartnerSet) {
	if !rs.CheckRecursion() {
		log.Panic(rs.T("You cannot create recursive Partner hierarchies."))
	}
}

func partner_Copy(rs m.PartnerSet, overrides m.PartnerData) m.PartnerSet {
	rs.EnsureOne()
	overrides.SetName(rs.T("%s (copy)", rs.Name()))
	return rs.Super().Copy(overrides)
}

// OnchangeParent updates the current partner data when its parent field
// is modified, in particular copies its parent's address.
func partner_OnchangeParent(rs m.PartnerSet) m.PartnerData {
	if rs.Parent().IsEmpty() || rs.Type() != "contact" {
		return h.Partner().NewData()
	}

	var parentHasAddress bool
	for _, addrField := range rs.AddressFields() {
		if !typesutils.IsZero(rs.Parent().Get(addrField)) {
			parentHasAddress = true
			break
		}
	}
	if !parentHasAddress {
		return h.Partner().NewData()
	}
	res := h.Partner().NewData()
	for _, addrField := range rs.AddressFields() {
		res.Set(addrField, rs.Parent().Get(addrField))
	}

	return res
}

// OnchangeParentWarning issues a warning when trying to change a contact to another parent company
func partner_OnchangeParentWarning(rs m.PartnerSet) string {
	origin, ok := rs.Env().Context().Get("hexya_onchange_origin").(m.PartnerData)
	if ok && origin.Parent().IsNotEmpty() && !origin.Parent().Equals(rs.Parent()) {
		return rs.T(`Changing the company of a contact should only be done if it
was never correctly set. If an existing contact starts working for a new
company then a new contact should be created under that new
company. You can use the "Discard" button to abandon this change.`)
	}
	return ""
}

// OnchangeCountryFilters sets the filters on state when country is modified
func partner_OnchangeCountryFilters(rs m.PartnerSet) map[models.FieldName]models.Conditioner {
	res := make(map[models.FieldName]models.Conditioner)
	res[h.Partner().Fields().State()] = q.CountryState().NewCondition()
	if rs.Country().IsNotEmpty() {
		res[h.Partner().Fields().State()] = q.CountryState().Country().Equals(rs.Country())
	}
	return res
}

// OnchangeEmail updates the user Gravatar image
func partner_OnchangeEmail(rs m.PartnerSet) m.PartnerData {
	if rs.Image() != "" || rs.Email() == "" || rs.Env().Context().HasKey("no_gravatar") {
		return h.Partner().NewData()
	}
	return h.Partner().NewData().SetImage(rs.GetGravatarImage(rs.Email()))
}

// ComputeEmailFormatted returns a 'Name <email@domain>' formatted string
func partner_ComputeEmailFormatted(rs m.PartnerSet) m.PartnerData {
	addr := mail.Address{Name: rs.Name(), Address: rs.Email()}
	return h.Partner().NewData().SetEmailFormatted(addr.String())
}

// ComputeIsCompany computes the IsCompany field from the selected CompanyType
func partner_ComputeIsCompany(rs m.PartnerSet) m.PartnerData {
	companyType := "person"
	if rs.IsCompany() {
		companyType = "company"
	}
	return h.Partner().NewData().SetCompanyType(companyType)
}

// InverseCompanyType sets the IsCompany field according to the given CompanyType
func partner_InverseCompanyType(rs m.PartnerSet, companyType string) {
	rs.SetIsCompany(companyType == "company")
}

// OnchangeCompanyType updates the IsCompany field according to the selected type
func partner_OnchangeCompanyType(rs m.PartnerSet) m.PartnerData {
	res := h.Partner().NewData().SetIsCompany(rs.CompanyType() == "company")
	return res
}

// UpdateFieldValues returns a PartnerData struct with its values set to
// this partner's values on the given fields. The other fields are left to their
// Go default value. This method is used to update fields from a partner to its
// relatives.`,
func partner_UpdateFieldValues(rs m.PartnerSet, fields ...models.FieldName) m.PartnerData {
	res := h.Partner().NewData()
	fInfos := rs.FieldsGet(models.FieldsGetArgs{})
	for _, f := range fields {
		if fInfos[f.JSON()].Type == fieldtype.One2Many {
			log.Panic(rs.T("One2Many fields cannot be synchronized as part of 'commercial_fields' or 'address fields'"))
		}
		res.Set(f, rs.Get(f))
	}
	return res
}

// AddressFields returns the list of fields which are part of the address.
// These are used to automate behaviours on contact addresses.
func partner_AddressFields(rs m.PartnerSet) []models.FieldName {
	return []models.FieldName{
		h.Partner().Fields().Street(), h.Partner().Fields().Street2(), h.Partner().Fields().Zip(),
		h.Partner().Fields().City(), h.Partner().Fields().State(), h.Partner().Fields().Country(),
	}
}

// UpdateAddress updates this PartnerSet only with the address fields of
// the given vals. Other values passed are discarded.
func partner_UpdateAddress(rs m.PartnerSet, vals m.PartnerData) bool {
	res := h.Partner().NewData()
	for _, addrField := range rs.AddressFields() {
		if vals.Has(addrField) {
			res.Set(addrField, vals.Get(addrField))
		}
	}
	if len(res.Keys()) == 0 {
		return false
	}
	return rs.WithContext("goto_super", true).Write(res)
}

// CommercialFields returns the list of fields that are managed by the commercial entity
// to which a partner belongs. These fields are meant to be hidden on
// partners that aren't "commercial entities"" themselves, and will be
// delegated to the parent "commercial entity"". The list is meant to be
// extended by inheriting classes.
func partner_CommercialFields(rs m.PartnerSet) []models.FieldName {
	return []models.FieldName{
		h.Partner().Fields().VAT(),
		h.Partner().Fields().CreditLimit(),
	}
}

// CommercialSyncFromCompany handle sync of commercial fields when a new parent commercial entity is set,
// as if they were related fields.
func partner_CommercialSyncFromCompany(rs m.PartnerSet) bool {
	if rs.Equals(rs.CommercialPartner()) {
		return false
	}
	values := rs.CommercialPartner().UpdateFieldValues(rs.CommercialFields()...)
	return rs.Write(values)
}

// CommercialSyncToChildren handle sync of commercial fields to descendants
func partner_CommercialSyncToChildren(rs m.PartnerSet) bool {
	partnerData := rs.CommercialPartner().UpdateFieldValues(rs.CommercialFields()...)
	syncChildren := rs.Children().Filtered(func(rs m.PartnerSet) bool {
		return !rs.IsCompany()
	})
	if syncChildren.IsEmpty() {
		return false
	}
	for _, child := range syncChildren.Records() {
		child.CommercialSyncToChildren()
	}
	partnerData.SetCommercialPartner(rs.CommercialPartner())
	return syncChildren.WithContext("hexya_force_compute_write", true).Write(partnerData)
}

// FieldsSync syncs commercial fields and address fields from company and to children after create/update,
// just as if those were all modeled as fields.related to the parent
func partner_FieldsSync(rs m.PartnerSet, vals m.PartnerData, fieldsToUnset ...models.FieldName) {
	// 1. From UPSTREAM: sync from parent
	// 1a. Commercial fields: sync if parent changed
	if !vals.Parent().IsEmpty() {
		rs.CommercialSyncFromCompany()
	}
	// 1b. Address fields: sync if parent or use_parent changed *and* both are now set
	if !rs.Parent().IsEmpty() && rs.Type() == "contact" {
		onchangePartnerData := rs.OnchangeParent()
		rs.UpdateAddress(onchangePartnerData)
	}
	// 2. To DOWNSTREAM: sync children
	if rs.Children().IsEmpty() {
		return
	}
	// 2a. Commercial Fields: sync if commercial entity
	if rs.Equals(rs.CommercialPartner()) {
		for _, commField := range rs.CommercialFields() {
			if !typesutils.IsZero(rs.Get(commField)) {
				rs.CommercialSyncToChildren()
				break
			}
		}
	}
	personChildren := rs.Children().Filtered(func(rs m.PartnerSet) bool {
		return !rs.IsCompany()
	})
	for _, child := range personChildren.Records() {
		if !child.CommercialPartner().Equals(rs.CommercialPartner()) {
			rs.CommercialSyncToChildren()
			break
		}

	}
	// 2b. Address fields: sync if address changed
	for _, addrField := range rs.AddressFields() {
		if vals.Has(addrField) {
			contacts := rs.Children().Search(q.Partner().Type().Equals("contact"))
			contacts.UpdateAddress(vals)
			break
		}
	}
}

// HandleFirsrtContactCreation: on creation of first contact for a company (or root) that has no address,
// assume contact address was meant to be company address
func partner_HandleFirsrtContactCreation(rs m.PartnerSet) {
	if !rs.Parent().IsCompany() && !rs.Parent().Parent().IsEmpty() {
		// Our parent is not a company, nor a root contact
		return
	}
	if rs.Parent().Children().Len() != 1 {
		// Our parent already has other children
		return
	}
	var addressDefined, parentAddressDefined bool
	for _, addrField := range rs.AddressFields() {
		if !typesutils.IsZero(rs.Parent().Get(addrField)) {
			parentAddressDefined = true
		}
		if !typesutils.IsZero(rs.Get(addrField)) {
			addressDefined = true
		}
	}
	if addressDefined && !parentAddressDefined {
		partnerData := rs.UpdateFieldValues(rs.AddressFields()...)
		rs.Parent().UpdateAddress(partnerData)
	}
}

// CleanWebsite returns a cleaned website url including scheme.
func partner_CleanWebsite(rs m.PartnerSet, website string) string {
	websiteURL, err := url.Parse(website)
	if err != nil {
		panic(fmt.Errorf("invalid URL for website: %s. %s", website, err))
	}
	if websiteURL.Scheme == "" {
		websiteURL.Scheme = "http"
	}
	return websiteURL.String()
}

func partner_Write(rs m.PartnerSet, vals m.PartnerData) bool {
	if rs.Env().Context().HasKey("goto_super") {
		return rs.Super().Write(vals)
	}
	rs.ResizeImageData(vals)
	if vals.Website() != "" {
		vals.SetWebsite(rs.CleanWebsite(vals.Website()))
	}
	if !vals.Parent().IsEmpty() {
		vals.SetCompanyName("")
	}
	// Partner must only allow to set the Company of a partner if it
	// is the same as the Company of all users that inherit from this partner
	// (this is to allow the code from User to write to the Partner!) or
	// if setting the Company to nil (this is compatible with any user
	// company)
	if !vals.Company().IsEmpty() {
		for _, partner := range rs.Records() {
			for _, user := range partner.Users().Records() {
				if !user.Company().Equals(vals.Company()) {
					log.Panic(rs.T("You can not change the company as the partner/user has multiple users linked with different companies.", "company", vals.Company().Name()))
				}
			}
		}
	}
	res := rs.Super().Write(vals)
	for _, partner := range rs.Records() {
		for _, user := range partner.Users().Records() {
			if user.HasGroup("base_group_user") {
				h.User().NewSet(rs.Env()).CheckExecutionPermission(h.CommonMixin().Methods().Write().Underlying())
				break
			}
		}
		partner.FieldsSync(vals)
	}
	return res
}

// ResizeImageData updates the given data struct with images set for the different sizes.
func partner_ResizeImageData(set m.PartnerSet, data m.PartnerData) {
	switch {
	case data.HasImage():
		data.SetImage(b64image.Resize(data.Image(), 1024, 1024, true))
		data.SetImageMedium(b64image.Resize(data.Image(), 128, 128, false))
		data.SetImageSmall(b64image.Resize(data.Image(), 64, 64, false))
	case data.HasImageMedium():
		data.SetImage(b64image.Resize(data.ImageMedium(), 1024, 1024, true))
		data.SetImageMedium(b64image.Resize(data.ImageMedium(), 128, 128, true))
		data.SetImageSmall(b64image.Resize(data.ImageMedium(), 64, 64, false))
	case data.HasImageSmall():
		data.SetImage(b64image.Resize(data.ImageSmall(), 1024, 1024, true))
		data.SetImageMedium(b64image.Resize(data.ImageSmall(), 128, 128, true))
		data.SetImageSmall(b64image.Resize(data.ImageSmall(), 64, 64, true))
	}
}

func partner_Create(rs m.PartnerSet, vals m.PartnerData) m.PartnerSet {
	if vals.Website() != "" {
		vals.SetWebsite(rs.CleanWebsite(vals.Website()))
	}
	if !vals.Parent().IsEmpty() {
		vals.SetCompanyName("")
	}
	if vals.Image() == "" {
		vals.SetImage(rs.GetDefaultImage(vals.Type(), vals.IsCompany(), vals.Parent()))
	}
	rs.ResizeImageData(vals)
	partner := rs.Super().Create(vals)
	partner.FieldsSync(vals)
	partner.HandleFirsrtContactCreation()
	return partner
}

// CreateCompany creates the parent company of this partner if it has been given a CompanyName.
func partner_CreateCompany(rs m.PartnerSet) bool {
	rs.EnsureOne()
	if rs.CompanyName() != "" {
		// Create parent company
		values := rs.UpdateFieldValues(rs.AddressFields()...)
		values.SetName(rs.CompanyName())
		values.SetIsCompany(true)
		newCompany := rs.Create(values)
		// Set newCompany as my parent
		rs.SetParent(newCompany)
		rs.Children().Write(h.Partner().NewData().SetParent(newCompany))
	}
	return true
}

// OpenCommercialEntity is a utility method used to add an "Open Company" button in partner views
func partner_OpenCommercialEntity(rs m.PartnerSet) *actions.Action {
	rs.EnsureOne()
	return &actions.Action{
		Type:     actions.ActionActWindow,
		Model:    "Partner",
		ViewMode: "form",
		ResID:    rs.CommercialPartner().ID(),
		Target:   "current",
		Flags:    map[string]interface{}{"form": map[string]interface{}{"action_buttons": true}},
	}
}

// OpenParent is a utility method used to add an "Open Parent" button in partner views
func partner_OpenParent(rs m.PartnerSet) *actions.Action {
	rs.EnsureOne()
	addressFormID := "base_view_partner_address_form"
	return &actions.Action{
		Type:     actions.ActionActWindow,
		Model:    "Partner",
		ViewMode: "form",
		Views:    []views.ViewTuple{{ID: addressFormID, Type: views.ViewTypeForm}},
		ResID:    rs.Parent().ID(),
		Target:   "new",
		Flags:    map[string]interface{}{"form": map[string]interface{}{"action_buttons": true}},
	}
}

func partner_NameGet(rs m.PartnerSet) string {
	name := rs.Name()
	if rs.CompanyName() != "" || !rs.Parent().IsEmpty() {
		if name == "" {
			switch rs.Type() {
			case "invoice", "delivery", "other":
				fInfo := rs.FieldGet(h.Partner().Fields().Type())
				name = fInfo.Selection[rs.Type()]
			}
		}
		if !rs.IsCompany() {
			name = fmt.Sprintf("%s, %s", rs.CommercialCompanyName(), name)
		}
	}
	if rs.Env().Context().GetBool("show_address_only") {
		name = rs.DisplayAddress(true)
	}
	if rs.Env().Context().GetBool("show_address") {
		name = name + "\n" + rs.DisplayAddress(true)
	}
	name = strings.Replace(name, "\n\n", "\n", -1)
	name = strings.Replace(name, "\n\n", "\n", -1)
	if rs.Env().Context().GetBool("show_email") && rs.Email() != "" {
		name = rs.EmailFormatted()
	}
	if rs.Env().Context().GetBool("html_format") {
		name = strings.Replace(name, "\n", "<br/>", -1)
	}
	return name
}

func partner_SearchByName(rs m.PartnerSet, name string, op operator.Operator, additionalCond q.PartnerCondition, limit int) m.PartnerSet {
	if name == "" {
		return rs.Super().SearchByName(name, op, additionalCond, limit)
	}
	var cond q.PartnerCondition
	switch op {
	case operator.Equals, operator.Contains, operator.IContains, operator.Like, operator.ILike:
		cond = q.Partner().Name().AddOperator(op, name).Or().
			Email().AddOperator(op, name).Or().
			Ref().AddOperator(op, name)
	}
	return rs.Search(cond).Limit(limit)
}

// ParsePartnerName parses an email address to get the partner's name.
// It returns the name as first argument and the email as the second.
//
// Supported syntax:
//   - 'Raoul <raoul@grosbedon.fr>': will find name and email address
//   - otherwise: default, everything is set as the name (email is returned empty)
func partner_ParsePartnerName(rs m.PartnerSet, email string) (string, string) {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return email, ""
	}
	return addr.Name, addr.Address
}

// NameCreate creates a partner from a single string which may be a name and/or an email.
//
// If only an email address is received and that the regex cannot find
// a name, the name will have the email value.
// If 'force_email' key in context: must find the email address.
func partner_NameCreate(rs m.PartnerSet, name string) m.PartnerSet {
	name, email := rs.ParsePartnerName(name)
	if email == "" && rs.Env().Context().GetBool("force_email") {
		panic(rs.T("Couldn't create contact without email address!"))
	}
	if name == "" && email != "" {
		name = email
	}
	if email == "" {
		email = rs.Env().Context().GetString("default_email")
	}
	partner := h.Partner().Create(rs.Env(), h.Partner().NewData().
		SetName(name).
		SetEmail(email))
	return partner
}

// FindOrCreate finds a partner with the given 'email' or creates one.
// The given string should contain at least one email,
// e.g. "Raoul Grosbedon <r.g@grosbedon.fr>"
func partner_FindOrCreate(rs m.PartnerSet, email string) m.PartnerSet {
	if _, emailParsed := rs.ParsePartnerName(email); emailParsed != "" {
		email = emailParsed
	}
	partners := h.Partner().Search(rs.Env(), q.Partner().Email().ILike(email)).Limit(1)
	if partners.IsEmpty() {
		partners = rs.NameCreate(email)
	}
	return partners
}

// GetGravatarImage returns the image from Gravatar associated with the given email.
// Image is returned as a base64 encoded string.
func partner_GetGravatarImage(rs m.PartnerSet, email string) string {
	emailHash := md5.Sum([]byte(strings.ToLower(email)))
	gravatarURL := fmt.Sprintf("%s/%x?%s", gravatarBaseURL, emailHash, "d=404&s=128")
	client := &http.Client{
		Timeout: 1 * time.Second,
	}
	resp, err := client.Get(gravatarURL)
	if err != nil || resp.StatusCode == http.StatusNotFound {
		return ""
	}
	img, err := ioutil.ReadAll(resp.Body)
	if len(img) == 0 || err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(img)
}

// AddressGet finds contacts/addresses of the right type(s) by doing a depth-first-search
// through descendants within company boundaries (stop at entities flagged 'IsCompany')
// then continuing the search at the ancestors that are within the same company boundaries.
// Defaults to partners of type 'default' when the exact type is not found, or to the
// provided partner itself if no type 'default' is found either.
//
// Result map keys are the contact types, such as 'contact', 'delivery', etc.`,
func partner_AddressGet(rs m.PartnerSet, addrTypes []string) map[string]m.PartnerSet {
	atMap := make(map[string]bool)
	for _, at := range addrTypes {
		atMap[at] = true
	}
	atMap["contact"] = true
	result := make(map[string]m.PartnerSet)
	visited := make(map[int64]bool)
	for _, partner := range rs.Records() {
		currentPartner := partner
		for !currentPartner.IsEmpty() {
			toScan := []m.PartnerSet{currentPartner}
			for len(toScan) > 0 {
				record := toScan[0]
				toScan = toScan[1:]
				visited[record.ID()] = true
				if _, exists := result[record.Type()]; atMap[record.Type()] && !exists {
					result[record.Type()] = record
				}
				if len(result) == len(atMap) {
					return result
				}
				for _, child := range record.Children().Records() {
					if !visited[child.ID()] && !child.IsCompany() {
						toScan = append(toScan, child)
					}
				}
			}
			// Continue scanning at ancestor if current_partner is not a commercial entity
			if currentPartner.IsCompany() || currentPartner.Parent().IsEmpty() {
				break
			}
			currentPartner = currentPartner.Parent()
		}
	}
	// default to type 'contact' or the partner itself
	def := rs
	if ct, ok := result["contact"]; ok {
		def = ct
	}
	for _, addrType := range addrTypes {
		if _, ok := result[addrType]; !ok {
			result[addrType] = def
		}
	}
	return result
}

// DisplayAddress builds and returns an address formatted accordingly to the
// standards of the country where it belongs.`,
func partner_DisplayAddress(rs m.PartnerSet, withoutCompany bool) string {
	addressFormat := rs.Country().AddressFormat()
	if addressFormat == "" {
		addressFormat = "{{ .Street }}\n{{ .Street2 }}\n{{ .City }} {{ .StateCode }} {{ .Zip }}\n{{ .CountryName}}"
	}
	data := basetypes.AddressData{
		Street:      rs.Street(),
		Street2:     rs.Street2(),
		City:        rs.City(),
		Zip:         rs.Zip(),
		StateCode:   rs.State().Code(),
		StateName:   rs.State().Name(),
		CountryCode: rs.Country().Code(),
		CountryName: rs.Country().Name(),
		CompanyName: rs.CommercialCompanyName(),
	}
	if data.CompanyName != "" {
		addressFormat = "{{ .CompanyName }}\n" + addressFormat
	}
	addressTemplate := template.Must(template.New("").Parse(addressFormat))
	var buf bytes.Buffer
	err := addressTemplate.Execute(&buf, data)
	if err != nil {
		log.Panic("Error while parsing address", "format", addressFormat, "data", data)
	}
	return buf.String()
}

func init() {
	models.NewModel("PartnerTitle")
	h.PartnerTitle().SetDefaultOrder("Name")
	h.PartnerTitle().AddFields(fields_PartnerTitle)

	models.NewModel("PartnerCategory")
	h.PartnerCategory().AddFields(fields_PartnerCategory)

	h.PartnerCategory().NewMethod("CheckParent", partnerCategory_CheckParent)
	h.PartnerCategory().Methods().NameGet().Extend(partnerCategory_NAmeGet)
	h.PartnerCategory().Methods().SearchByName().Extend(partnerCategory_SearchByName)

	models.NewModel("Partner")

	h.Partner().AddFields(fields_Partner)
	h.Partner().Fields().DisplayName().SetDepends([]string{"IsCompany", "Name", "Parent.Name", "Type", "CompanyName"})
	h.Partner().AddSQLConstraint("check_name",
		"CHECK( (type='contact' AND name IS NOT NULL) or (type != 'contact') )",
		"Contacts require a name.")

	h.Partner().Methods().ComputeDisplayName().Extend(partner_ComputeDisplayName)
	h.Partner().NewMethod("ComputeTZOffset", partner_ComputeTZOffset)
	h.Partner().NewMethod("ComputePartnerShare", partner_ComputePartnerShare)
	h.Partner().NewMethod("ComputeContactAddress", partner_ComputeContactAddress)
	h.Partner().NewMethod("ComputeCommercialPartner", partner_ComputeCommercialPartner)
	h.Partner().NewMethod("ComputeCommercialCompanyName", partner_ComputeCommercialCompanyName)
	h.Partner().NewMethod("GetDefaultImage", partner_GetDefaultImage)
	h.Partner().NewMethod("CheckParent", partner_CheckParent)
	h.Partner().Methods().Copy().Extend(partner_Copy)
	h.Partner().NewMethod("OnchangeParent", partner_OnchangeParent)
	h.Partner().NewMethod("OnchangeParentWarning", partner_OnchangeParentWarning)
	h.Partner().NewMethod("OnchangeCountryFilters", partner_OnchangeCountryFilters)
	h.Partner().NewMethod("OnchangeEmail", partner_OnchangeEmail)
	h.Partner().NewMethod("ComputeEmailFormatted", partner_ComputeEmailFormatted)
	h.Partner().NewMethod("ComputeCompanyType", partner_ComputeIsCompany)
	h.Partner().NewMethod("InverseCompanyType", partner_InverseCompanyType)
	h.Partner().NewMethod("OnchangeCompanyType", partner_OnchangeCompanyType)
	h.Partner().NewMethod("UpdateFieldValues", partner_UpdateFieldValues)
	h.Partner().NewMethod("AddressFields", partner_AddressFields)
	h.Partner().NewMethod("UpdateAddress", partner_UpdateAddress)
	h.Partner().NewMethod("CommercialFields", partner_CommercialFields)
	h.Partner().NewMethod("CommercialSyncFromCompany", partner_CommercialSyncFromCompany)
	h.Partner().NewMethod("CommercialSyncToChildren", partner_CommercialSyncToChildren)
	h.Partner().NewMethod("FieldsSync", partner_FieldsSync)
	h.Partner().NewMethod("HandleFirsrtContactCreation", partner_HandleFirsrtContactCreation)
	h.Partner().NewMethod("CleanWebsite", partner_CleanWebsite)
	h.Partner().Methods().Write().Extend(partner_Write)
	h.Partner().NewMethod("ResizeImageData", partner_ResizeImageData)
	h.Partner().Methods().Create().Extend(partner_Create)
	h.Partner().NewMethod("CreateCompany", partner_CreateCompany)
	h.Partner().NewMethod("OpenCommercialEntity", partner_OpenCommercialEntity)
	h.Partner().NewMethod("OpenParent", partner_OpenParent)
	h.Partner().Methods().NameGet().Extend(partner_NameGet)
	h.Partner().Methods().SearchByName().Extend(partner_SearchByName)
	h.Partner().NewMethod("ParsePartnerName", partner_ParsePartnerName)
	h.Partner().NewMethod("NameCreate", partner_NameCreate)
	h.Partner().NewMethod("FindOrCreate", partner_FindOrCreate)
	h.Partner().NewMethod("GetGravatarImage", partner_GetGravatarImage)
	h.Partner().NewMethod("AddressGet", partner_AddressGet)
	h.Partner().NewMethod("DisplayAddress", partner_DisplayAddress)
}
