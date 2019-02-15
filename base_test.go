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
	"github.com/hexya-erp/pool/m"
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
				ghostStep := h.Partner().Create(env, h.Partner().NewData().
					SetName("GhostStep").
					SetIsCompany(true).
					SetStreet("Main Street, 10").
					SetPhone("123456789").
					SetEmail("info@ghoststep.com").
					SetVAT("BE0477472701").
					SetType("contact"))
				p1 := h.Partner().NewSet(env).NameCreate("Denis Bladesmith <denis.bladesmith@ghoststep.com>")
				So(p1.Type(), ShouldEqual, "contact")
				p1Phone := "123456789#34"
				p1.Write(h.Partner().NewData().
					SetPhone(p1Phone).
					SetParent(ghostStep))
				So(p1.Street(), ShouldEqual, ghostStep.Street())
				So(p1.Phone(), ShouldEqual, p1Phone)
				So(p1.Type(), ShouldEqual, "contact")
				So(p1.Email(), ShouldEqual, "denis.bladesmith@ghoststep.com")
				p1Street := "Different street, 42"
				p1.Write(h.Partner().NewData().
					SetStreet(p1Street).
					SetType("invoice"))
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
				p1 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Isen Hardearth").
					SetStreet("Strongarm Avenue, 12").
					SetParent(ironShield))
				So(p1.Type(), ShouldEqual, "contact")
				So(ironShield.Street(), ShouldEqual, p1.Street())
			})
			Convey("Partner AddressGet", func() {
				elmTree := h.Partner().NewSet(env).NameCreate("ElmTree")
				branch1 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Branch 1").
					SetParent(elmTree).
					SetIsCompany(true))
				leaf10 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Leaf 10").
					SetParent(branch1).
					SetType("invoice"))
				branch11 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Branch 11").
					SetParent(branch1).
					SetType("other"))
				leaf111 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Leaf 111").
					SetParent(branch11).
					SetType("delivery"))
				branch11.SetIsCompany(false) // force IsCompany after creating 1rst child
				branch2 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Branch 2").
					SetParent(elmTree).
					SetIsCompany(true))
				leaf21 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Leaf 21").
					SetParent(branch2).
					SetType("delivery"))
				leaf22 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Leaf 22").
					SetParent(branch2))
				leaf23 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Leaf 23").
					SetParent(branch2).
					SetType("contact"))

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
				p0 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Sigurd Sunknife").
					SetEmail("ssunknife@gmail.com"))
				sunhelm := h.Partner().Create(env, h.Partner().NewData().
					SetName("Sunhelm").
					SetIsCompany(true).
					SetStreet("Rainbow Street, 13").
					SetPhone("1122334455").
					SetEmail("info@sunhelm.com").
					SetVAT("BE0477472701").
					SetChildren(p0.Union(h.Partner().Create(env, h.Partner().NewData().
						SetName("Alrik Greenthorn").
						SetEmail("agr@sunhelm.com")))))
				p1 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Otto Blackwood").
					SetEmail("otto.blackwood@sunhelm.com").
					SetParent(sunhelm))
				p11 := h.Partner().Create(env, h.Partner().NewData().
					SetName("Gini Graywool").
					SetEmail("ggr@sunhelm.com").
					SetParent(p1))
				p2 := h.Partner().Search(env, q.Partner().Email().Equals("agr@sunhelm.com"))
				sunhelm.Write(h.Partner().NewData().
					SetChildren(sunhelm.Children().Union(h.Partner().Create(env, h.Partner().NewData().
						SetName("Ulrik Greenthorn").
						SetEmail("ugr@sunhelm.com")))))
				p3 := h.Partner().Search(env, q.Partner().Email().Equals("ugr@sunhelm.com"))

				for _, p := range []m.PartnerSet{p0, p1, p11, p2, p3} {
					So(p.CommercialPartner().Equals(sunhelm), ShouldBeTrue)
					So(p.VAT(), ShouldEqual, sunhelm.VAT())
				}

				sunhemlVAT := "BE0123456789"
				sunhelm.SetVAT(sunhemlVAT)
				for _, p := range []m.PartnerSet{p0, p1, p11, p2, p3} {
					So(p.VAT(), ShouldEqual, sunhemlVAT)
				}

				p1VAT := "BE0987654321"
				p1.SetVAT(p1VAT)
				for _, p := range []m.PartnerSet{p0, p11, p2, p3} {
					So(p.VAT(), ShouldEqual, sunhemlVAT)
				}

				// promote p1 to commercial entity
				p1.Write(h.Partner().NewData().
					SetParent(sunhelm).
					SetIsCompany(true).
					SetName("SunHelm Subsidiary"))
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
			titleSir := h.PartnerTitle().Create(env, h.PartnerTitle().NewData().SetName("Sir..."))
			titleLady := h.PartnerTitle().Create(env, h.PartnerTitle().NewData().SetName("Lady..."))
			testUsers := []m.UserData{
				h.User().NewData().SetName("Alice").SetLogin("alice").SetColor(1).SetFunction("Friend").SetDate(dates.ParseDate("2015-03-28")).SetTitle(titleLady),
				h.User().NewData().SetName("Alice").SetLogin("alice2").SetColor(0).SetFunction("Friend").SetDate(dates.ParseDate("2015-01-28")).SetTitle(titleLady),
				h.User().NewData().SetName("Bob").SetLogin("bob").SetColor(2).SetFunction("Friend").SetDate(dates.ParseDate("2015-03-02")).SetTitle(titleSir),
				h.User().NewData().SetName("Eve").SetLogin("eve").SetColor(3).SetFunction("Eavesdropper").SetDate(dates.ParseDate("2015-03-20")).SetTitle(titleLady),
				h.User().NewData().SetName("Nab").SetLogin("nab").SetColor(-3).SetFunction("5$ Wrench").SetDate(dates.ParseDate("2014-09-10")).SetTitle(titleSir),
				h.User().NewData().SetName("Nab").SetLogin("nabshe").SetColor(6).SetFunction("5$ Wrench").SetDate(dates.ParseDate("2014-01-02")).SetTitle(titleLady),
			}

			users := h.User().NewSet(env)
			for _, vals := range testUsers {
				users = users.Union(h.User().Create(env, vals))
			}
			condition := q.User().ID().In(users.Ids())

			Convey("Group on local char field without domain and without active_test (-> empty WHERE clause)", func() {
				groupsData := h.User().NewSet(env).WithContext("active_test", false).
					SearchAll().
					GroupBy(q.User().Login()).
					OrderBy("login DESC").
					Aggregates(q.User().Login())
				So(len(groupsData), ShouldBeGreaterThan, 6)

			})

			Convey("Group on local char field with limit", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(q.User().Login()).
					OrderBy("login DESC").
					Limit(3).
					Offset(3).
					Aggregates(q.User().Login())
				So(groupsData, ShouldHaveLength, 3)
				So(groupsData[0].Values().Login(), ShouldEqual, "bob")
				So(groupsData[1].Values().Login(), ShouldEqual, "alice2")
				So(groupsData[2].Values().Login(), ShouldEqual, "alice")
			})

			Convey("Group on inherited char field, aggregate on int field (second groupby ignored on purpose)", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(q.User().Function()).
					Aggregates(q.User().Name(), q.User().Color(), q.User().Function())
				So(groupsData, ShouldHaveLength, 3)
				So(groupsData[0].Values().Function(), ShouldEqual, "5$ Wrench")
				So(groupsData[1].Values().Function(), ShouldEqual, "Eavesdropper")
				So(groupsData[2].Values().Function(), ShouldEqual, "Friend")
				for _, gd := range groupsData {
					So(gd.Values().Color(), ShouldEqual, 3)
				}
			})
			Convey("Group on inherited char field, reverse order", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(q.User().Name()).
					OrderBy("name DESC").
					Aggregates(q.User().Name(), q.User().Color())
				So(groupsData[0].Values().Name(), ShouldEqual, "Nab")
				So(groupsData[1].Values().Name(), ShouldEqual, "Eve")
				So(groupsData[2].Values().Name(), ShouldEqual, "Bob")
				So(groupsData[3].Values().Name(), ShouldEqual, "Alice")

			})

			Convey("Group on int field, default ordering", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(q.User().Color()).
					Aggregates(q.User().Color())
				So(groupsData[0].Values().Color(), ShouldEqual, -3)
				So(groupsData[1].Values().Color(), ShouldEqual, 0)
				So(groupsData[2].Values().Color(), ShouldEqual, 1)
				So(groupsData[3].Values().Color(), ShouldEqual, 2)
				So(groupsData[4].Values().Color(), ShouldEqual, 3)
				So(groupsData[5].Values().Color(), ShouldEqual, 6)
			})

			Convey("Multi group, second level is int field, should still be summed in first level grouping", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(q.User().Name()).
					OrderBy("name DESC").
					Aggregates(q.User().Name(), q.User().Color())
				So(groupsData[0].Values().Name(), ShouldEqual, "Nab")
				So(groupsData[1].Values().Name(), ShouldEqual, "Eve")
				So(groupsData[2].Values().Name(), ShouldEqual, "Bob")
				So(groupsData[3].Values().Name(), ShouldEqual, "Alice")
				So(groupsData[0].Values().Color(), ShouldEqual, 3)
				So(groupsData[1].Values().Color(), ShouldEqual, 3)
				So(groupsData[2].Values().Color(), ShouldEqual, 2)
				So(groupsData[3].Values().Color(), ShouldEqual, 1)
			})

			Convey("Group on inherited char field, multiple orders with directions", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(q.User().Name()).
					OrderBy("color DESC", "name").
					Aggregates(q.User().Name(), q.User().Color())
				So(groupsData, ShouldHaveLength, 4)
				So(groupsData[0].Values().Name(), ShouldEqual, "Eve")
				So(groupsData[1].Values().Name(), ShouldEqual, "Nab")
				So(groupsData[2].Values().Name(), ShouldEqual, "Bob")
				So(groupsData[3].Values().Name(), ShouldEqual, "Alice")
				So(groupsData[0].Count(), ShouldEqual, 1)
				So(groupsData[1].Count(), ShouldEqual, 2)
				So(groupsData[2].Count(), ShouldEqual, 1)
				So(groupsData[3].Count(), ShouldEqual, 2)
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
					GroupBy(q.User().Title()).
					OrderBy("Title.Name").
					Aggregates(q.User().Function(), q.User().Color(), q.User().Title())
				So(groupsData, ShouldHaveLength, 2)
				So(groupsData[0].Values().Title().Equals(titleLady), ShouldBeTrue)
				So(groupsData[1].Values().Title().Equals(titleSir), ShouldBeTrue)
				So(groupsData[0].Values().Color(), ShouldEqual, 10)
				So(groupsData[1].Values().Color(), ShouldEqual, -1)
				So(groupsData[0].Count(), ShouldEqual, 4)
				So(groupsData[1].Count(), ShouldEqual, 2)
			})

			Convey("Group on inherited many2one (res_partner.title), reversed natural order", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(q.User().Title()).
					OrderBy("Title.Name DESC").
					Aggregates(q.User().Function(), q.User().Color(), q.User().Title())
				So(groupsData, ShouldHaveLength, 2)
				So(groupsData[0].Values().Title().Equals(titleSir), ShouldBeTrue)
				So(groupsData[1].Values().Title().Equals(titleLady), ShouldBeTrue)
				So(groupsData[0].Values().Color(), ShouldEqual, -1)
				So(groupsData[1].Values().Color(), ShouldEqual, 10)
				So(groupsData[0].Count(), ShouldEqual, 2)
				So(groupsData[1].Count(), ShouldEqual, 4)
			})

			Convey("Group on inherited many2one (res_partner.title), multiple orders with m2o in second position", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(q.User().Title()).
					OrderBy("color DESC", "Title.Name DESC").
					Aggregates(q.User().Function(), q.User().Color(), q.User().Title())
				So(groupsData, ShouldHaveLength, 2)
				So(groupsData[0].Values().Title().Equals(titleLady), ShouldBeTrue)
				So(groupsData[1].Values().Title().Equals(titleSir), ShouldBeTrue)
				So(groupsData[0].Values().Color(), ShouldEqual, 10)
				So(groupsData[1].Values().Color(), ShouldEqual, -1)
				So(groupsData[0].Count(), ShouldEqual, 4)
				So(groupsData[1].Count(), ShouldEqual, 2)
			})

			Convey("Group on inherited many2one (res_partner.title), ordered by other inherited field (color)", func() {
				groupsData := h.User().NewSet(env).
					Search(condition).
					GroupBy(q.User().Title()).
					OrderBy("color").
					Aggregates(q.User().Function(), q.User().Color(), q.User().Title())
				So(groupsData, ShouldHaveLength, 2)
				So(groupsData[0].Values().Title().Equals(titleSir), ShouldBeTrue)
				So(groupsData[1].Values().Title().Equals(titleLady), ShouldBeTrue)
				So(groupsData[0].Values().Color(), ShouldEqual, -1)
				So(groupsData[1].Values().Color(), ShouldEqual, 10)
				So(groupsData[0].Count(), ShouldEqual, 2)
				So(groupsData[1].Count(), ShouldEqual, 4)
			})
		}), ShouldBeNil)
	})
}

func TestPartnerRecursion(t *testing.T) {
	Convey("Testing Partner Recursion", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			p1 := h.Partner().NewSet(env).NameCreate("Elmtree")
			p2 := h.Partner().Create(env, h.Partner().NewData().
				SetName("Elmtree Child 1").
				SetParent(p1))
			p3 := h.Partner().Create(env, h.Partner().NewData().
				SetName("Elmtree Grand-Child 1.1").
				SetParent(p2))
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
			root := h.PartnerCategory().Create(env, h.PartnerCategory().NewData().SetName("Root Category"))
			cat0 := h.PartnerCategory().Create(env, h.PartnerCategory().NewData().SetName("Parent Category").SetParent(root))
			cat1 := h.PartnerCategory().Create(env, h.PartnerCategory().NewData().SetName("Child 1").SetParent(cat0))
			cat2 := h.PartnerCategory().Create(env, h.PartnerCategory().NewData().SetName("Child 2").SetParent(cat0))
			h.PartnerCategory().Create(env, h.PartnerCategory().NewData().SetName("Child 2-1").SetParent(cat2))
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
				newCat0 := cat0.Copy(h.PartnerCategory().NewData().SetChildren(h.PartnerCategory().NewSet(env)))
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
				newCat0 := cat0.Copy(h.PartnerCategory().NewData().SetChildren(newCat1.Union(newCat2)))
				newStruct := h.PartnerCategory().Search(env, q.PartnerCategory().Parent().ChildOf(newCat0))
				So(newStruct.Len(), ShouldEqual, 3)
				oldStruct := h.PartnerCategory().Search(env, q.PartnerCategory().Parent().ChildOf(cat0))
				So(oldStruct.Len(), ShouldEqual, 3)
				So(newStruct.Intersect(oldStruct).IsEmpty(), ShouldBeTrue)
			})
		}), ShouldBeNil)
	})
}
