// Copyright 2018 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tests"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMain(m *testing.M) {
	tests.RunTests(m, "base")
}

var samples = [][3]string{
	{`"Raoul Grosbedon" <raoul@chirurgiens-dentistes.fr> `, `Raoul Grosbedon`, `raoul@chirurgiens-dentistes.fr`},
	{`ryu+giga-Sushi@aizubange.fukushima.jp`, "", "ryu+giga-Sushi@aizubange.fukushima.jp"},
	{"Raoul chirurgiens-dentistes.fr", "Raoul chirurgiens-dentistes.fr", ""},
	{" Raoul O'hara  <!@historicalsociety.museum>", "Raoul O'hara", "!@historicalsociety.museum"},
}

func TestPartners(t *testing.T) {
	Convey("Testing Partners", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Partner NameCreate", func() {
				for _, sample := range samples {
					name, mail := sample[1], sample[2]
					pName, pMail := h.Partner().NewSet(env).ParsePartnerName(sample[0])
					So(pName, ShouldEqual, name)
					So(pMail, ShouldEqual, mail)
					partner := h.Partner().NewSet(env).NameCreate(sample[0])
					So(partner.Name(), ShouldBeIn, []string{name, mail})
					So(partner.Email(), ShouldBeIn, []string{mail, ""})
				}
			})
			Convey("Partner FindorCreate", func() {
				email := samples[0][0]
				partner := h.Partner().NewSet(env).NameCreate(email)
				found := h.Partner().NewSet(env).FindOrCreate(email)
				So(partner.Equals(found), ShouldBeTrue)
				partner2 := h.Partner().NewSet(env).FindOrCreate("sarah.john@connor.com")
				found2 := h.Partner().NewSet(env).FindOrCreate("john@connor.com")
				So(partner2.Equals(found2), ShouldBeFalse)
				newPartner := h.Partner().NewSet(env).FindOrCreate(samples[1][0])
				So(newPartner.ID(), ShouldBeGreaterThan, partner.ID())
				newPartner2 := h.Partner().NewSet(env).FindOrCreate(samples[2][0])
				So(newPartner2.ID(), ShouldBeGreaterThan, newPartner.ID())
			})
			Convey("Partner NameSearch", func() {
				data := []struct {
					name   string
					active bool
				}{
					{`"A Raoul Grosbedon" <raoul@chirurgiens-dentistes.fr>`, false},
					{`B Raoul chirurgiens-dentistes.fr`, true},
					{"C Raoul O'hara  <!@historicalsociety.museum>", true},
					{"ryu+giga-Sushi@aizubange.fukushima.jp", true},
				}
				for _, d := range data {
					h.Partner().NewSet(env).WithContext("default_active", d.active).NameCreate(d.name)
				}
				partners := h.Partner().NewSet(env).SearchByName("Raoul", operator.IContains, q.PartnerCondition{}, 0)
				So(partners.Len(), ShouldEqual, 2)
				partners2 := h.Partner().NewSet(env).SearchByName("Raoul", operator.IContains, q.PartnerCondition{}, 1)
				So(partners2.Len(), ShouldEqual, 1)
				So(partners2.DisplayName(), ShouldEqual, "B Raoul chirurgiens-dentistes.fr")
			})
			Convey("Partner Address Sync", func() {
				ghostStep := h.Partner().Create(env, &h.PartnerData{
					Name:      "GhostStep",
					IsCompany: true,
					Street:    "Main Street, 10",
					Phone:     "123456789",
					Email:     "info@ghoststep.com",
					VAT:       "BE0477472701",
					Type:      "contact",
				})
				p1 := h.Partner().NewSet(env).NameCreate("Denis Bladesmith <denis.bladesmith@ghoststep.com>")
				So(p1.Type(), ShouldEqual, "contact")
				p1Phone := "123456789#34"
				p1.Write(&h.PartnerData{
					Phone:  p1Phone,
					Parent: ghostStep,
				})
				So(p1.Street(), ShouldEqual, ghostStep.Street())
				So(p1.Phone(), ShouldEqual, p1Phone)
				So(p1.Type(), ShouldEqual, "contact")
				So(p1.Email(), ShouldEqual, "denis.bladesmith@ghoststep.com")
				p1Street := "Different street, 42"
				p1.Write(&h.PartnerData{
					Street: p1Street,
					Type:   "invoice",
				})
				So(p1.Street(), ShouldEqual, p1Street)
				So(ghostStep.Street(), ShouldNotEqual, p1Street)
				p1.SetType("contact")
				So(p1.Street(), ShouldEqual, ghostStep.Street())
				So(p1.Phone(), ShouldEqual, p1Phone)
				So(p1.Type(), ShouldEqual, "contact")
				So(p1.Email(), ShouldEqual, "denis.bladesmith@ghoststep.com")
				ghostStreet := "South Street, 25"
				ghostStep.SetStreet(ghostStreet)
				So(p1.Street(), ShouldEqual, ghostStreet)
				So(p1.Phone(), ShouldEqual, p1Phone)
				So(p1.Email(), ShouldEqual, "denis.bladesmith@ghoststep.com")
				p1Street = "My Street, 11"
				p1.SetStreet(p1Street)
				So(ghostStep.Street(), ShouldEqual, ghostStreet)
			})
			Convey("Partner First Contact Sync", func() {
				ironShield := h.Partner().NewSet(env).NameCreate("IronShield")
				So(ironShield.IsCompany(), ShouldBeFalse)
				So(ironShield.Type(), ShouldEqual, "contact")
				ironShield.SetType("contact")
				p1 := h.Partner().Create(env, &h.PartnerData{
					Name:   "Isen Hardearth",
					Street: "Strongarm Avenue, 12",
					Parent: ironShield,
				})
				So(p1.Type(), ShouldEqual, "contact")
				So(ironShield.Street(), ShouldEqual, p1.Street())
			})
			Convey("Partner AddressGet", func() {
				elmTree := h.Partner().NewSet(env).NameCreate("ElmTree")
				branch1 := h.Partner().Create(env, &h.PartnerData{
					Name:      "Branch 1",
					Parent:    elmTree,
					IsCompany: true,
				})
				leaf10 := h.Partner().Create(env, &h.PartnerData{
					Name:   "Leaf 10",
					Parent: branch1,
					Type:   "invoice",
				})
				branch11 := h.Partner().Create(env, &h.PartnerData{
					Name:   "Branch 11",
					Parent: branch1,
					Type:   "other",
				})
				leaf111 := h.Partner().Create(env, &h.PartnerData{
					Name:   "Leaf 111",
					Parent: branch11,
					Type:   "delivery",
				})
				branch11.SetIsCompany(false) // force IsCompany after creating 1rst child
				branch2 := h.Partner().Create(env, &h.PartnerData{
					Name:      "Branch 2",
					Parent:    elmTree,
					IsCompany: true,
				})
				leaf21 := h.Partner().Create(env, &h.PartnerData{
					Name:   "Leaf 21",
					Parent: branch2,
					Type:   "delivery",
				})
				leaf22 := h.Partner().Create(env, &h.PartnerData{
					Name:   "Leaf 22",
					Parent: branch2,
				})
				leaf23 := h.Partner().Create(env, &h.PartnerData{
					Name:   "Leaf 23",
					Parent: branch2,
					Type:   "contact",
				})

				// go up, stop at branch1
				leaf111Addr := leaf111.AddressGet([]string{"delivery", "invoice", "contact", "other"})
				So(leaf111Addr["delivery"].Equals(leaf111), ShouldBeTrue)
				So(leaf111Addr["invoice"].Equals(leaf10), ShouldBeTrue)
				So(leaf111Addr["contact"].Equals(branch1), ShouldBeTrue)
				So(leaf111Addr["other"].Equals(branch11), ShouldBeTrue)
				branch11Addr := branch11.AddressGet([]string{"delivery", "invoice", "contact", "other"})
				So(branch11Addr["delivery"].Equals(leaf111), ShouldBeTrue)
				So(branch11Addr["invoice"].Equals(leaf10), ShouldBeTrue)
				So(branch11Addr["contact"].Equals(branch1), ShouldBeTrue)
				So(branch11Addr["other"].Equals(branch11), ShouldBeTrue)

				// go down, stop at at all child companies
				elmAddr := elmTree.AddressGet([]string{"delivery", "invoice", "contact", "other"})
				So(elmAddr["delivery"].Equals(elmTree), ShouldBeTrue)
				So(elmAddr["invoice"].Equals(elmTree), ShouldBeTrue)
				So(elmAddr["contact"].Equals(elmTree), ShouldBeTrue)
				So(elmAddr["other"].Equals(elmTree), ShouldBeTrue)

				// go down through children
				branch1Addr := branch1.AddressGet([]string{"delivery", "invoice", "contact", "other"})
				So(branch1Addr["delivery"].Equals(leaf111), ShouldBeTrue)
				So(branch1Addr["invoice"].Equals(leaf10), ShouldBeTrue)
				So(branch1Addr["contact"].Equals(branch1), ShouldBeTrue)
				So(branch1Addr["other"].Equals(branch11), ShouldBeTrue)
				branch2Addr := branch2.AddressGet([]string{"delivery", "invoice", "contact", "other"})
				So(branch2Addr["delivery"].Equals(leaf21), ShouldBeTrue)
				So(branch2Addr["invoice"].Equals(branch2), ShouldBeTrue)
				So(branch2Addr["contact"].Equals(branch2), ShouldBeTrue)
				So(branch2Addr["other"].Equals(branch2), ShouldBeTrue)

				// go up then down through siblings
				leaf21Addr := leaf21.AddressGet([]string{"delivery", "invoice", "contact", "other"})
				So(leaf21Addr["delivery"].Equals(leaf21), ShouldBeTrue)
				So(leaf21Addr["invoice"].Equals(branch2), ShouldBeTrue)
				So(leaf21Addr["contact"].Equals(branch2), ShouldBeTrue)
				So(leaf21Addr["other"].Equals(branch2), ShouldBeTrue)

				leaf22Addr := leaf22.AddressGet([]string{"delivery", "invoice", "contact", "other"})
				So(leaf22Addr["delivery"].Equals(leaf21), ShouldBeTrue)
				So(leaf22Addr["invoice"].Equals(leaf22), ShouldBeTrue)
				So(leaf22Addr["contact"].Equals(leaf22), ShouldBeTrue)
				So(leaf22Addr["other"].Equals(leaf22), ShouldBeTrue)

				leaf23Addr := leaf23.AddressGet([]string{"delivery", "invoice", "contact", "other"})
				So(leaf23Addr["delivery"].Equals(leaf21), ShouldBeTrue)
				So(leaf23Addr["invoice"].Equals(leaf23), ShouldBeTrue)
				So(leaf23Addr["contact"].Equals(leaf23), ShouldBeTrue)
				So(leaf23Addr["other"].Equals(leaf23), ShouldBeTrue)

				// empty adr_pref means only 'contact'
				elmTreeAddrC := elmTree.AddressGet(nil)
				So(elmTreeAddrC, ShouldHaveLength, 1)
				So(elmTreeAddrC["contact"].Equals(elmTree), ShouldBeTrue)

				leaf111AddrC := leaf111.AddressGet(nil)
				So(leaf111AddrC, ShouldHaveLength, 1)
				So(leaf111AddrC["contact"].Equals(branch1), ShouldBeTrue)

				branch11.SetType("contact")
				leaf111AddrC2 := leaf111.AddressGet(nil)
				So(leaf111AddrC2["contact"].Equals(branch11), ShouldBeTrue)
			})
			Convey("Partner Commercial Sync", func() {
				p0 := h.Partner().Create(env, &h.PartnerData{
					Name:  "Sigurd Sunknife",
					Email: "ssunknife@gmail.com",
				})
				sunhelm := h.Partner().Create(env, &h.PartnerData{
					Name:      "Sunhelm",
					IsCompany: true,
					Street:    "Rainbow Street, 13",
					Phone:     "1122334455",
					Email:     "info@sunhelm.com",
					VAT:       "BE0477472701",
					Children: p0.Union(h.Partner().Create(env, &h.PartnerData{
						Name:  "Alrik Greenthorn",
						Email: "agr@sunhelm.com",
					})),
				})
				p1 := h.Partner().Create(env, &h.PartnerData{
					Name:   "Otto Blackwood",
					Email:  "otto.blackwood@sunhelm.com",
					Parent: sunhelm,
				})
				p11 := h.Partner().Create(env, &h.PartnerData{
					Name:   "Gini Graywool",
					Email:  "ggr@sunhelm.com",
					Parent: p1,
				})
				p2 := h.Partner().Search(env, q.Partner().Email().Equals("agr@sunhelm.com"))
				sunhelm.Write(&h.PartnerData{
					Children: sunhelm.Children().Union(h.Partner().Create(env, &h.PartnerData{
						Name:  "Ulrik Greenthorn",
						Email: "ugr@sunhelm.com",
					})),
				})
				p3 := h.Partner().Search(env, q.Partner().Email().Equals("ugr@sunhelm.com"))

				for _, p := range []h.PartnerSet{p0, p1, p11, p2, p3} {
					So(p.CommercialPartner().Equals(sunhelm), ShouldBeTrue)
					So(p.VAT(), ShouldEqual, sunhelm.VAT())
				}

				sunhemlVAT := "BE0123456789"
				sunhelm.SetVAT(sunhemlVAT)
				for _, p := range []h.PartnerSet{p0, p1, p11, p2, p3} {
					So(p.VAT(), ShouldEqual, sunhemlVAT)
				}

				p1VAT := "BE0987654321"
				p1.SetVAT(p1VAT)
				for _, p := range []h.PartnerSet{p0, p11, p2, p3} {
					So(p.VAT(), ShouldEqual, sunhemlVAT)
				}

				// promote p1 to commercial entity
				p1.Write(&h.PartnerData{
					Parent:    sunhelm,
					IsCompany: true,
					Name:      "SunHelm Subsidiary",
				})
				So(p1.VAT(), ShouldEqual, p1VAT)
				So(p1.CommercialPartner().Equals(p1), ShouldBeTrue)

				// writing on parent should not touch child commercial entities
				sunhemlVAT2 := "BE0112233445"
				sunhelm.SetVAT(sunhemlVAT2)
				So(p1.VAT(), ShouldEqual, p1VAT)
				So(p0.VAT(), ShouldEqual, sunhemlVAT2)
			})
		}), ShouldBeNil)
	})
}

func BenchmarkPartnersDBLookup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			partners := h.Partner().NewSet(env).SearchAll().Limit(1)
			partners.Name()
		})
	}
}

func BenchmarkPartnersCacheLookup(b *testing.B) {
	models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		partners := h.Partner().NewSet(env).SearchAll().Limit(1)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			partners.Name()
		}
	})
}

func BenchmarkPartnersSimpleMethodCall(b *testing.B) {
	models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		partners := h.Partner().NewSet(env).SearchAll().Limit(1)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			partners.ParsePartnerName("toto@hexya.io")
		}
	})
}

func BenchmarkPartnersNameGetMethodCall(b *testing.B) {
	models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		partners := h.Partner().NewSet(env).SearchAll().Limit(1)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			partners.NameGet()
		}
	})
}

func TestAggregateRead(t *testing.T) {
	Convey("Aggregate Read", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			titleSir := h.PartnerTitle().Create(env, &h.PartnerTitleData{
				Name: "Sir...",
			})
			titleLady := h.PartnerTitle().Create(env, &h.PartnerTitleData{
				Name: "Lady...",
			})
			testUsers := []h.UserData{
				{Name: "Alice", Login: "alice", Color: 1, Function: "Friend", Date: dates.ParseDate("2015-03-28"), Title: titleLady},
				{Name: "Alice", Login: "alice2", Color: 0, Function: "Friend", Date: dates.ParseDate("2015-01-28"), Title: titleLady},
				{Name: "Bob", Login: "bob", Color: 2, Function: "Friend", Date: dates.ParseDate("2015-03-02"), Title: titleSir},
				{Name: "Eve", Login: "eve", Color: 3, Function: "Eavesdropper", Date: dates.ParseDate("2015-03-20"), Title: titleLady},
				{Name: "Nab", Login: "nab", Color: -3, Function: "5$ Wrench", Date: dates.ParseDate("2014-09-10"), Title: titleSir},
				{Name: "Nab", Login: "nab-she", Color: 6, Function: "5$ Wrench", Date: dates.ParseDate("2014-01-02"), Title: titleLady},
			}

			users := h.User().NewSet(env)
			for _, vals := range testUsers {
				users = users.Union(h.User().Create(env, &vals))
			}
			condition := q.User().ID().In(users.Ids())

			Convey("Group on local char field without domain and without active_test (-> empty WHERE clause)", func() {
				groupsData := h.User().NewSet(env).WithContext("active_test", false).
					SearchAll().
					GroupBy(h.User().Login()).
					OrderBy("login DESC").
					Aggregates(h.User().Login())
				So(len(groupsData), ShouldBeGreaterThan, 6)

			})

			Convey("Group on local char field with limit", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(h.User().Login()).
					OrderBy("login DESC").
					Limit(3).
					Offset(3).
					Aggregates(h.User().Login())
				So(groupsData, ShouldHaveLength, 3)
				So(groupsData[0].Values.Login, ShouldEqual, "bob")
				So(groupsData[1].Values.Login, ShouldEqual, "alice2")
				So(groupsData[2].Values.Login, ShouldEqual, "alice")
			})

			Convey("Group on inherited char field, aggregate on int field (second groupby ignored on purpose)", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(h.User().Function()).
					Aggregates(h.User().Name(), h.User().Color(), h.User().Function())
				So(groupsData, ShouldHaveLength, 3)
				So(groupsData[0].Values.Function, ShouldEqual, "5$ Wrench")
				So(groupsData[1].Values.Function, ShouldEqual, "Eavesdropper")
				So(groupsData[2].Values.Function, ShouldEqual, "Friend")
				for _, gd := range groupsData {
					So(gd.Values.Color, ShouldEqual, 3)
				}
			})
			Convey("Group on inherited char field, reverse order", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(h.User().Name()).
					OrderBy("name DESC").
					Aggregates(h.User().Name(), h.User().Color())
				So(groupsData[0].Values.Name, ShouldEqual, "Nab")
				So(groupsData[1].Values.Name, ShouldEqual, "Eve")
				So(groupsData[2].Values.Name, ShouldEqual, "Bob")
				So(groupsData[3].Values.Name, ShouldEqual, "Alice")

			})

			Convey("Group on int field, default ordering", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(h.User().Color()).
					Aggregates(h.User().Color())
				So(groupsData[0].Values.Color, ShouldEqual, -3)
				So(groupsData[1].Values.Color, ShouldEqual, 0)
				So(groupsData[2].Values.Color, ShouldEqual, 1)
				So(groupsData[3].Values.Color, ShouldEqual, 2)
				So(groupsData[4].Values.Color, ShouldEqual, 3)
				So(groupsData[5].Values.Color, ShouldEqual, 6)
			})

			Convey("Multi group, second level is int field, should still be summed in first level grouping", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(h.User().Name()).
					OrderBy("name DESC").
					Aggregates(h.User().Name(), h.User().Color())
				So(groupsData[0].Values.Name, ShouldEqual, "Nab")
				So(groupsData[1].Values.Name, ShouldEqual, "Eve")
				So(groupsData[2].Values.Name, ShouldEqual, "Bob")
				So(groupsData[3].Values.Name, ShouldEqual, "Alice")
				So(groupsData[0].Values.Color, ShouldEqual, 3)
				So(groupsData[1].Values.Color, ShouldEqual, 3)
				So(groupsData[2].Values.Color, ShouldEqual, 2)
				So(groupsData[3].Values.Color, ShouldEqual, 1)
			})

			Convey("Group on inherited char field, multiple orders with directions", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(h.User().Name()).
					OrderBy("color DESC", "name").
					Aggregates(h.User().Name(), h.User().Color())
				So(groupsData, ShouldHaveLength, 4)
				So(groupsData[0].Values.Name, ShouldEqual, "Eve")
				So(groupsData[1].Values.Name, ShouldEqual, "Nab")
				So(groupsData[2].Values.Name, ShouldEqual, "Bob")
				So(groupsData[3].Values.Name, ShouldEqual, "Alice")
				So(groupsData[0].Count, ShouldEqual, 1)
				So(groupsData[1].Count, ShouldEqual, 2)
				So(groupsData[2].Count, ShouldEqual, 1)
				So(groupsData[3].Count, ShouldEqual, 2)
			})

			Convey("Group on inherited date column (res_partner.date) -> Year-Month, default ordering", func() {
				//groups_data = res_users.read_group(domain, fields=['function', 'color', 'date'], groupby=['date'])
				//self.assertEqual(len(groups_data), 4, "Incorrect number of results when grouping on a field")
				//self.assertEqual(['January 2014', 'September 2014', 'January 2015', 'March 2015'], [g['date'] for g in groups_data], 'Incorrect ordering of the list')
				//self.assertEqual([1, 1, 1, 3], [g['date_count'] for g in groups_data], 'Incorrect number of results')
			})

			Convey("Group on inherited date column (res_partner.date) -> Year-Month, custom order", func() {
				//groups_data = res_users.read_group(domain, fields=['function', 'color', 'date'], groupby=['date'], orderby='date DESC')
				//self.assertEqual(len(groups_data), 4, "Incorrect number of results when grouping on a field")
				//self.assertEqual(['March 2015', 'January 2015', 'September 2014', 'January 2014'], [g['date'] for g in groups_data], 'Incorrect ordering of the list')
				//self.assertEqual([3, 1, 1, 1], [g['date_count'] for g in groups_data], 'Incorrect number of results')
			})

			Convey("Group on inherited many2one (res_partner.title), default order", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(h.User().Title()).
					OrderBy("Title.Name").
					Aggregates(h.User().Function(), h.User().Color(), h.User().Title())
				So(groupsData, ShouldHaveLength, 2)
				So(groupsData[0].Values.Title.Equals(titleLady), ShouldBeTrue)
				So(groupsData[1].Values.Title.Equals(titleSir), ShouldBeTrue)
				So(groupsData[0].Values.Color, ShouldEqual, 10)
				So(groupsData[1].Values.Color, ShouldEqual, -1)
				So(groupsData[0].Count, ShouldEqual, 4)
				So(groupsData[1].Count, ShouldEqual, 2)
			})

			Convey("Group on inherited many2one (res_partner.title), reversed natural order", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(h.User().Title()).
					OrderBy("Title.Name DESC").
					Aggregates(h.User().Function(), h.User().Color(), h.User().Title())
				So(groupsData, ShouldHaveLength, 2)
				So(groupsData[0].Values.Title.Equals(titleSir), ShouldBeTrue)
				So(groupsData[1].Values.Title.Equals(titleLady), ShouldBeTrue)
				So(groupsData[0].Values.Color, ShouldEqual, -1)
				So(groupsData[1].Values.Color, ShouldEqual, 10)
				So(groupsData[0].Count, ShouldEqual, 2)
				So(groupsData[1].Count, ShouldEqual, 4)
			})

			Convey("Group on inherited many2one (res_partner.title), multiple orders with m2o in second position", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(h.User().Title()).
					OrderBy("color DESC", "Title.Name DESC").
					Aggregates(h.User().Function(), h.User().Color(), h.User().Title())
				So(groupsData, ShouldHaveLength, 2)
				So(groupsData[0].Values.Title.Equals(titleLady), ShouldBeTrue)
				So(groupsData[1].Values.Title.Equals(titleSir), ShouldBeTrue)
				So(groupsData[0].Values.Color, ShouldEqual, 10)
				So(groupsData[1].Values.Color, ShouldEqual, -1)
				So(groupsData[0].Count, ShouldEqual, 4)
				So(groupsData[1].Count, ShouldEqual, 2)
			})

			Convey("Group on inherited many2one (res_partner.title), ordered by other inherited field (color)", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(h.User().Title()).
					OrderBy("color").
					Aggregates(h.User().Function(), h.User().Color(), h.User().Title())
				So(groupsData, ShouldHaveLength, 2)
				So(groupsData[0].Values.Title.Equals(titleSir), ShouldBeTrue)
				So(groupsData[1].Values.Title.Equals(titleLady), ShouldBeTrue)
				So(groupsData[0].Values.Color, ShouldEqual, -1)
				So(groupsData[1].Values.Color, ShouldEqual, 10)
				So(groupsData[0].Count, ShouldEqual, 2)
				So(groupsData[1].Count, ShouldEqual, 4)
			})
		}), ShouldBeNil)
	})
}

func TestPartnerRecursion(t *testing.T) {
	Convey("Testing Partner Recursion", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			p1 := h.Partner().NewSet(env).NameCreate("Elmtree")
			p2 := h.Partner().Create(env, &h.PartnerData{
				Name:   "Elmtree Child 1",
				Parent: p1,
			})
			p3 := h.Partner().Create(env, &h.PartnerData{
				Name:   "Elmtree Grand-Child 1.1",
				Parent: p2,
			})
			Convey("Our initial data is OK", func() {
				So(p3.CheckRecursion(), ShouldBeTrue)
				So(p1.Union(p2).Union(p3).CheckRecursion(), ShouldBeTrue)
			})
			Convey("Creating a recursion on p1 should panic", func() {
				So(func() { p1.SetParent(p3) }, ShouldPanic)
			})
			Convey("Creating a recursion on p2 should panic", func() {
				So(func() { p2.SetParent(p3) }, ShouldPanic)
			})
			Convey("Creating a recursion on p3 should panic", func() {
				So(func() { p3.SetParent(p3) }, ShouldPanic)
			})
			Convey("Multi write on several partners should not panic", func() {
				ps := p1.Union(p2).Union(p3)
				So(func() { ps.SetPhone("123456") }, ShouldNotPanic)
			})
		}), ShouldBeNil)
	})
}

func TestParentStore(t *testing.T) {
	Convey("Testing recursive queries", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			root := h.PartnerCategory().Create(env, &h.PartnerCategoryData{
				Name: "Root Category",
			})
			cat0 := h.PartnerCategory().Create(env, &h.PartnerCategoryData{
				Name:   "Parent Category",
				Parent: root,
			})
			cat1 := h.PartnerCategory().Create(env, &h.PartnerCategoryData{
				Name:   "Child 1",
				Parent: cat0,
			})
			cat2 := h.PartnerCategory().Create(env, &h.PartnerCategoryData{
				Name:   "Child 2",
				Parent: cat0,
			})
			h.PartnerCategory().Create(env, &h.PartnerCategoryData{
				Name:   "Child 2-1",
				Parent: cat2,
			})
			Convey("Duplicate the parent category and verify that the children have been duplicated too", func() {
				newCat0 := cat0.Copy(nil)
				newStruct := h.PartnerCategory().Search(env, q.PartnerCategory().Parent().ChildOf(newCat0))
				So(newStruct.Len(), ShouldEqual, 3)
				oldStruct := h.PartnerCategory().Search(env, q.PartnerCategory().Parent().ChildOf(cat0))
				So(oldStruct.Len(), ShouldEqual, 3)
				So(newStruct.Intersect(oldStruct).IsEmpty(), ShouldBeTrue)
			})
			Convey("Duplicate the parent category and check with id child of", func() {
				newCat0 := cat0.Copy(nil)
				newStruct := h.PartnerCategory().Search(env, q.PartnerCategory().ID().ChildOf(newCat0.ID()))
				So(newStruct.Len(), ShouldEqual, 4)
				oldStruct := h.PartnerCategory().Search(env, q.PartnerCategory().ID().ChildOf(cat0.ID()))
				So(oldStruct.Len(), ShouldEqual, 4)
				So(newStruct.Intersect(oldStruct).IsEmpty(), ShouldBeTrue)
			})
			Convey("Duplicate the children then reassign them to the new parent (1st method).", func() {
				newCat1 := cat1.Copy(nil)
				newCat2 := cat2.Copy(nil)
				newCat0 := cat0.Copy(nil, h.PartnerCategory().Children())
				So(newCat0.Children().IsEmpty(), ShouldBeTrue)
				newCat1.Union(newCat2).SetParent(newCat0)
				newStruct := h.PartnerCategory().Search(env, q.PartnerCategory().Parent().ChildOf(newCat0))
				So(newStruct.Len(), ShouldEqual, 3)
				oldStruct := h.PartnerCategory().Search(env, q.PartnerCategory().Parent().ChildOf(cat0))
				So(oldStruct.Len(), ShouldEqual, 3)
				So(newStruct.Intersect(oldStruct).IsEmpty(), ShouldBeTrue)
			})
			Convey("Duplicate the children then reassign them to the new parent (2nd method).", func() {
				newCat1 := cat1.Copy(nil)
				newCat2 := cat2.Copy(nil)
				newCat0 := cat0.Copy(&h.PartnerCategoryData{Children: newCat1.Union(newCat2)})
				newStruct := h.PartnerCategory().Search(env, q.PartnerCategory().Parent().ChildOf(newCat0))
				So(newStruct.Len(), ShouldEqual, 3)
				oldStruct := h.PartnerCategory().Search(env, q.PartnerCategory().Parent().ChildOf(cat0))
				So(oldStruct.Len(), ShouldEqual, 3)
				So(newStruct.Intersect(oldStruct).IsEmpty(), ShouldBeTrue)
			})
		}), ShouldBeNil)
	})
}
