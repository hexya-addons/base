// Copyright 2018 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"fmt"
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func dropSequence(sequence string) {
	models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		h.Sequence().Search(env, q.Sequence().Code().Equals(sequence))
	})
}

func TestSequenceStandard(t *testing.T) {
	Convey("Testing Standard Sequences", t, func() {
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Create a Sequence", func() {
				seq := h.Sequence().Create(env, h.Sequence().NewData().
					SetCode("test_sequence_type").
					SetName("Test sequence"))
				So(seq.IsEmpty(), ShouldBeFalse)
			})
			Convey("Search for a sequence", func() {
				seqs := h.Sequence().NewSet(env).SearchAll()
				So(seqs.IsEmpty(), ShouldBeFalse)
			})
			Convey("Try to draw a number", func() {
				n := h.Sequence().NewSet(env).NextByCode("test_sequence_type")
				So(n, ShouldNotEqual, 0)
			})
			Convey("Try to draw a number from two transactions.", func() {
				n0 := h.Sequence().NewSet(env).NextByCode("test_sequence_type")
				var n1 string
				models.ExecuteInNewEnvironment(security.SuperUserID, func(env2 models.Environment) {
					n1 = h.Sequence().NewSet(env2).NextByCode("test_sequence_type")
				})
				So(n0, ShouldNotEqual, 0)
				So(n1, ShouldNotEqual, 0)
			})
		}), ShouldBeNil)
	})
	dropSequence("test_sequence_type")
}

func TestSequenceNoGap(t *testing.T) {
	Convey("Testing No Gap sequences", t, func() {
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Create a no gap sequence", func() {
				seq := h.Sequence().Create(env, h.Sequence().NewData().
					SetCode("test_sequence_type_2").
					SetName("Test sequence").
					SetImplementation("no_gap"))
				So(seq.IsEmpty(), ShouldBeFalse)
			})
			Convey("Try to draw a number", func() {
				n := h.Sequence().NewSet(env).NextByCode("test_sequence_type_2")
				So(n, ShouldNotEqual, 0)
			})
			Convey("Try to draw a number from two transactions.", func() {
				n0 := h.Sequence().NewSet(env).NextByCode("test_sequence_type_2")
				var n1 string
				models.ExecuteInNewEnvironment(security.SuperUserID, func(env2 models.Environment) {
					n1 = h.Sequence().NewSet(env2).NextByCode("test_sequence_type_2")
				})
				So(n0, ShouldNotEqual, 0)
				So(n1, ShouldNotEqual, 0)
			})
		}), ShouldBeNil)
	})
	dropSequence("test_sequence_type_2")
}

func TestSequenceChangeImplementation(t *testing.T) {
	Convey("Testing changing sequence implementations", t, func() {
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Create sequences", func() {
				seq := h.Sequence().Create(env, h.Sequence().NewData().
					SetCode("test_sequence_type_3").
					SetName("Test sequence"))
				So(seq.IsEmpty(), ShouldBeFalse)
				seq = h.Sequence().Create(env, h.Sequence().NewData().
					SetCode("test_sequence_type_4").
					SetName("Test sequence").
					SetImplementation("no_gap"))
				So(seq.IsEmpty(), ShouldBeFalse)
			})
			Convey("Change implementation on sequences", func() {
				seqs := h.Sequence().Search(env, q.Sequence().Code().In([]string{"test_sequence_type_3", "test_sequence_type_4"}))
				seqs.SetImplementation("standard")
				seqs.SetImplementation("no_gap")
			})
			Convey("Remove sequences", func() {
				seqs := h.Sequence().Search(env, q.Sequence().Code().In([]string{"test_sequence_type_3", "test_sequence_type_4"}))
				seqs.Unlink()
			})
		}), ShouldBeNil)
	})
	dropSequence("test_sequence_type_3")
	dropSequence("test_sequence_type_4")
}

func TestSequenceGenerate(t *testing.T) {
	Convey("Create sequence objects and generate some values", t, func() {
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Create standard sequence", func() {
				seq := h.Sequence().Create(env, h.Sequence().NewData().
					SetCode("test_sequence_type_5").
					SetName("Test sequence"))
				So(seq.IsEmpty(), ShouldBeFalse)
			})
			Convey("Read from standard sequence", func() {
				for i := 1; i <= 10; i++ {
					n := h.Sequence().NewSet(env).NextByCode("test_sequence_type_5")
					So(n, ShouldEqual, fmt.Sprint(i))
				}
			})
			Convey("Create no gap sequence", func() {
				seq := h.Sequence().Create(env, h.Sequence().NewData().
					SetCode("test_sequence_type_6").
					SetName("Test sequence").
					SetImplementation("no_gap"))
				So(seq.IsEmpty(), ShouldBeFalse)
			})
			Convey("Read from no gap sequence", func() {
				for i := 1; i <= 10; i++ {
					n := h.Sequence().NewSet(env).NextByCode("test_sequence_type_6")
					So(n, ShouldEqual, fmt.Sprint(i))
				}
			})
		}), ShouldBeNil)
	})
	dropSequence("test_sequence_type_5")
	dropSequence("test_sequence_type_6")
}

func TestSequenceInit(t *testing.T) {
	Convey("Test whether the read method returns the right number_next value", t, func() {
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Create and read the sequence", func() {
				seq := h.Sequence().Create(env, h.Sequence().NewData().
					SetNumberNext(1).
					SetPadding(4).
					SetNumberIncrement(1).
					SetImplementation("standard").
					SetName("test-sequence-00"))
				seq.NextByID()
				seq.NextByID()
				seq.NextByID()
				n := seq.NextByID()
				So(n, ShouldEqual, "0004")
				seq.SetNumberNext(1)
				n = seq.NextByID()
				So(n, ShouldEqual, "0001")
			})
		}), ShouldBeNil)
	})
}

func TestSequenceDateRangeStandard(t *testing.T) {
	Convey("A few tests for a 'Standard' (i.e. PostgreSQL) sequence", t, func() {
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Create a sequence object with date ranges enabled", func() {
				seq := h.Sequence().Create(env, h.Sequence().NewData().
					SetCode("test_sequence_date_range").
					SetName("Test sequence").
					SetUseDateRange(true))
				So(seq.IsEmpty(), ShouldBeFalse)
			})
			Convey("Draw numbers to create a first subsequence then change its date range", func() {
				year := dates.Today().Year() - 1
				january := func(d int) dates.Date {
					return dates.ParseDate(fmt.Sprintf("%d-01-%02d", year, d))
				}

				seq16 := h.Sequence().NewSet(env).WithContext("sequence_date", january(16))
				n := seq16.NextByCode("test_sequence_date_range")
				So(n, ShouldEqual, "1")
				n = seq16.NextByCode("test_sequence_date_range")
				So(n, ShouldEqual, "2")

				seqDateRange := h.SequenceDateRange().Search(env,
					q.SequenceDateRange().SequenceFilteredOn(
						q.Sequence().Code().Equals("test_sequence_date_range")).
						And().DateFrom().Equals(january(1)))
				seqDateRange.SetDateFrom(january(18))
				n = seq16.NextByCode("test_sequence_date_range")
				So(n, ShouldEqual, "1")

				seqDateRange = h.SequenceDateRange().Search(env,
					q.SequenceDateRange().SequenceFilteredOn(
						q.Sequence().Code().Equals("test_sequence_date_range")).
						And().DateFrom().Equals(january(1)))
				So(seqDateRange.DateTo().Equal(january(17)), ShouldBeTrue)
			})
			Convey("Remove sequence", func() {
				seq := h.Sequence().Search(env, q.Sequence().Code().Equals("test_sequence_date_range"))
				seq.Unlink()
			})
		}), ShouldBeNil)
	})
}

func TestSequenceDateRangeNoGap(t *testing.T) {
	Convey("A few tests for a 'no gap' sequence", t, func() {
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Create a sequence object with date ranges enabled", func() {
				seq := h.Sequence().Create(env, h.Sequence().NewData().
					SetCode("test_sequence_date_range_2").
					SetName("Test sequence").
					SetUseDateRange(true).
					SetImplementation("no_gap"))
				So(seq.IsEmpty(), ShouldBeFalse)
			})
			Convey("Draw numbers to create a first subsequence then change its date range", func() {
				year := dates.Today().Year() - 1
				january := func(d int) dates.Date {
					return dates.ParseDate(fmt.Sprintf("%d-01-%02d", year, d))
				}

				seq16 := h.Sequence().NewSet(env).WithContext("sequence_date", january(16))
				n := seq16.NextByCode("test_sequence_date_range_2")
				So(n, ShouldEqual, "1")
				n = seq16.NextByCode("test_sequence_date_range_2")
				So(n, ShouldEqual, "2")

				seqDateRange := h.SequenceDateRange().Search(env,
					q.SequenceDateRange().SequenceFilteredOn(
						q.Sequence().Code().Equals("test_sequence_date_range_2")).
						And().DateFrom().Equals(january(1)))
				seqDateRange.SetDateFrom(january(18))
				n = seq16.NextByCode("test_sequence_date_range_2")
				So(n, ShouldEqual, "1")

				seqDateRange = h.SequenceDateRange().Search(env,
					q.SequenceDateRange().SequenceFilteredOn(
						q.Sequence().Code().Equals("test_sequence_date_range_2")).
						And().DateFrom().Equals(january(1)))
				So(seqDateRange.DateTo().Equal(january(17)), ShouldBeTrue)
			})
			Convey("Remove sequence", func() {
				seq := h.Sequence().Search(env, q.Sequence().Code().Equals("test_sequence_date_range_2"))
				seq.Unlink()
			})
		}), ShouldBeNil)
	})
}

func TestSequenceDateRangeChangeImplementation(t *testing.T) {
	Convey("Create sequence objects and change their 'implementation' field", t, func() {
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Try to create a sequence object", func() {
				seq := h.Sequence().Create(env, h.Sequence().NewData().
					SetCode("test_sequence_date_range_3").
					SetName("Test Sequence").
					SetUseDateRange(true))
				So(seq.IsEmpty(), ShouldBeFalse)

				seq = h.Sequence().Create(env, h.Sequence().NewData().
					SetCode("test_sequence_date_range_4").
					SetName("Test Sequence").
					SetUseDateRange(true).
					SetImplementation("no_gap"))
				So(seq.IsEmpty(), ShouldBeFalse)
			})
			Convey("Make some use of the sequences to create some subsequences", func() {
				year := dates.Today().Year() - 1
				january := func(d int) dates.Date {
					return dates.ParseDate(fmt.Sprintf("%d-01-%02d", year, d))
				}

				seq := h.Sequence().NewSet(env)
				seq16 := h.Sequence().NewSet(env).WithContext("sequence_date", january(16))
				for i := 1; i <= 5; i++ {
					n := seq.NextByCode("test_sequence_date_range_3")
					So(n, ShouldEqual, fmt.Sprint(i))
				}
				for i := 1; i <= 5; i++ {
					n := seq16.NextByCode("test_sequence_date_range_3")
					So(n, ShouldEqual, fmt.Sprint(i))
				}
				for i := 1; i <= 5; i++ {
					n := seq.NextByCode("test_sequence_date_range_4")
					So(n, ShouldEqual, fmt.Sprint(i))
				}
				for i := 1; i <= 5; i++ {
					n := seq16.NextByCode("test_sequence_date_range_4")
					So(n, ShouldEqual, fmt.Sprint(i))
				}
			})
			Convey("swap the implementation method on both", func() {
				seqs := h.Sequence().Search(env, q.Sequence().Code().In([]string{"test_sequence_date_range_3", "test_sequence_date_range_4"}))
				seqs.SetImplementation("standard")
				seqs.SetImplementation("no_gap")
			})
			Convey("Unlink sequences", func() {
				seqs := h.Sequence().Search(env, q.Sequence().Code().In([]string{"test_sequence_date_range_3", "test_sequence_date_range_4"}))
				seqs.Unlink()
			})
		}), ShouldBeNil)
	})
}
