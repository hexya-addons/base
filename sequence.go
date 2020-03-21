// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"fmt"
	"strings"
	"time"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

// Sequences maps Hexya formats to Go time format
var Sequences = map[string]string{
	"year": "2006", "month": "01", "day": "02", "y": "06",
	"h24": "15", "h12": "03", "min": "04", "sec": "05",
}

// SequenceFuncs maps Hexya formats to functions that must be applied to a time.Time object
var SequenceFuncs = map[string]func(time.Time) string{
	"doy": func(t time.Time) string {
		return fmt.Sprintf("%d", t.YearDay())
	},
	"woy": func(t time.Time) string {
		_, woy := t.ISOWeek()
		return fmt.Sprintf("%d", woy)
	},
	"weekday": func(t time.Time) string {
		return fmt.Sprintf("%d", int(t.Weekday()))
	},
}

var fields_Sequence = map[string]models.FieldDefinition{
	"Name": fields.Char{Required: true},
	"Code": fields.Char{String: "Sequence Code"},
	"Implementation": fields.Selection{
		Selection: types.Selection{"standard": "Standard", "no_gap": "No Gap"}, Required: true,
		Default: models.DefaultValue("standard"),
		Help: `Two sequence object implementations are offered: Standard and 'No gap'.
The latter is slower than the former but forbids any
gap in the sequence (while they are possible in the former).`},
	"Active": fields.Boolean{Default: models.DefaultValue(true), Required: true},
	"Prefix": fields.Char{Help: "Prefix value of the record for the sequence"},
	"Suffix": fields.Char{Help: "Suffix value of the record for the sequence"},
	"NumberNext": fields.Integer{String: "Next Number", Required: true,
		Default: models.DefaultValue(1), Help: "Next number of this sequence"},
	"NumberNextActual": fields.Integer{
		Compute: h.Sequence().Methods().ComputeNumberNextActual(), String: "Next Number",
		Inverse: h.Sequence().Methods().InverseNumberNextActual(),
		Help:    "Next number that will be used. This number can be incremented frequently so the displayed value might already be obsolete",
		Depends: []string{"NumberNext"}},
	"NumberIncrement": fields.Integer{String: "Step", Required: true,
		Default: models.DefaultValue(1), Help: "The next number of the sequence will be incremented by this number"},
	"Padding": fields.Integer{String: "Sequence Size", Required: true,
		Default: models.DefaultValue(0),
		Help:    "Hexya will automatically adds some '0' on the left of the 'Next Number' to get the required padding size."},
	"Company": fields.Many2One{RelationModel: h.Company(), Default: func(env models.Environment) interface{} {
		return h.Company().NewSet(env).CompanyDefaultGet()
	}},
	"UseDateRange": fields.Boolean{String: "Use subsequences per Date Range"},
	"DateRanges": fields.One2Many{RelationModel: h.SequenceDateRange(), ReverseFK: "Sequence",
		String: "Subsequences"},
}

// ComputeNumberNextActual returns the real next number for the sequence depending on the implementation
func sequence_ComputeNumberNextActual(rs m.SequenceSet) m.SequenceData {
	res := h.Sequence().NewData().SetNumberNextActual(rs.NumberNext())
	return res
}

// InverseNumberNextActual is the setter function for the NumberNextActual field
func sequence_InverseNumberNextActual(rs m.SequenceSet, value int64) {
	if value == 0 {
		value = 1
	}
	rs.SetNumberNext(value)
}

// AlterHexyaSequence alters the DB sequence that backs this sequence
func sequence_AlterHexyaSequence(rs m.SequenceSet, numberIncrement int64, numberNext int64) {
	rs.EnsureOne()
	hexyaSeq, exists := models.Registry.GetSequence(fmt.Sprintf("sequence_%03d", rs.ID()))
	if !exists {
		// sequence is not created yet, we're inside create() so ignore it, will be set later
		return
	}
	hexyaSeq.Alter(numberIncrement, numberNext)
}

func sequence_Create(rs m.SequenceSet, vals m.SequenceData) m.SequenceSet {
	seq := rs.Super().Create(vals)
	if !vals.HasImplementation() || vals.Implementation() == "standard" {
		models.CreateSequence(fmt.Sprintf("sequence_%03d", seq.ID()), seq.NumberIncrement(), seq.NumberNext())
	}
	return seq
}

func sequence_Unlink(rs m.SequenceSet) int64 {
	for _, rec := range rs.Records() {
		hexyaSeq, exists := models.Registry.GetSequence(fmt.Sprintf("sequence_%03d", rec.ID()))
		if exists {
			hexyaSeq.Drop()
		}
	}
	return rs.Super().Unlink()
}

func sequence_Write(rs m.SequenceSet, data m.SequenceData) bool {
	newImplementation := data.Implementation()
	for _, seq := range rs.Records() {
		// 4 cases: we test the previous impl. against the new one.
		i := data.NumberIncrement()
		if i == 0 {
			i = seq.NumberIncrement()
		}
		n := data.NumberNext()
		if n == 0 {
			n = seq.NumberNext()
		}
		if seq.Implementation() == "standard" {
			if newImplementation == "standard" || newImplementation == "" {
				// Implementation has NOT changed.
				// Only change sequence if really requested.
				if data.NumberNext() != 0 {
					seq.AlterHexyaSequence(0, n)
				}
				if seq.NumberIncrement() != i {
					seq.AlterHexyaSequence(i, 0)
					seq.DateRanges().AlterHexyaSequence(i, 0)
				}
			} else {
				if hexyaSeq, ok := models.Registry.GetSequence(fmt.Sprintf("sequence_%03d", seq.ID())); ok {
					hexyaSeq.Drop()
				}
				for _, subSeq := range seq.DateRanges().Records() {
					if subHexyaSeq, ok := models.Registry.GetSequence(fmt.Sprintf("sequence_%03d_%03d", seq.ID(), subSeq.ID())); ok {
						subHexyaSeq.Drop()
					}
				}
			}
			continue
		}
		if newImplementation == "no_gap" || newImplementation == "" {
			continue
		}
		models.CreateSequence(fmt.Sprintf("sequence_%03d", seq.ID()), i, n)
		for _, subSeq := range seq.DateRanges().Records() {
			models.CreateSequence(fmt.Sprintf("sequence_%03d_%03d", seq.ID(), subSeq.ID()), i, n)
		}
	}
	return rs.Super().Write(data)
}

// NextDo returns the next sequence number formatted
func sequence_NextDo(rs m.SequenceSet) string {
	rs.EnsureOne()
	if rs.Implementation() == "standard" {
		hexyaSeq := models.Registry.MustGetSequence(fmt.Sprintf("sequence_%03d", rs.ID()))
		return rs.GetNextChar(hexyaSeq.NextValue())
	}
	return rs.GetNextChar(rs.UpdateNoGap())
}

// UpdateNoGap gets the next number of a "No Gap" sequence
func sequence_UpdateNoGap(rs m.SequenceSet) int64 {
	rs.EnsureOne()
	numberNext := rs.NumberNext()
	rs.Env().Cr().Execute(`SELECT number_next FROM sequence WHERE id=? FOR UPDATE NOWAIT`, rs.ID())
	rs.Env().Cr().Execute(`UPDATE sequence SET number_next=number_next + ? WHERE id=?`, rs.NumberIncrement(), rs.ID())
	rs.Collection().InvalidateCache()
	return numberNext
}

// GetNextChar returns the given number formatted as per the sequence data
func sequence_GetNextChar(rs m.SequenceSet, numberNext int64) string {
	interpolate := func(format string, data map[string]string) string {
		if format == "" {
			return ""
		}
		res := format
		for k, v := range data {
			res = strings.Replace(res, fmt.Sprintf("%%(%s)s", k), v, -1)
		}
		return res
	}
	interpolateMap := func() map[string]string {
		location, err := time.LoadLocation(rs.Env().Context().GetString("tz"))
		if err != nil {
			location = time.UTC
		}
		now := time.Now().In(location)
		rangeDate, effectiveDate := now, now
		if rs.Env().Context().HasKey("sequence_date") {
			effectiveDate = rs.Env().Context().GetDate("sequence_date").Time
		}
		if rs.Env().Context().HasKey("sequence_date_range") {
			rangeDate = rs.Env().Context().GetDate("sequence_date_range").Time
		}

		res := make(map[string]string)
		for key, format := range Sequences {
			res[key] = effectiveDate.Format(format)
			res["range_"+key] = rangeDate.Format(format)
			res["current_"+key] = now.Format(format)
		}
		for key, fFunc := range SequenceFuncs {
			res[key] = fFunc(effectiveDate)
			res["range_"+key] = fFunc(rangeDate)
			res["current_"+key] = fFunc(now)
		}
		return res
	}
	d := interpolateMap()
	interpolatedPrefix := interpolate(rs.Prefix(), d)
	interpolatedSuffix := interpolate(rs.Suffix(), d)
	return interpolatedPrefix +
		fmt.Sprintf(fmt.Sprintf("%%0%dd", rs.Padding()), numberNext) +
		interpolatedSuffix
}

// CreateDateRangeSeq creates the date range for the given date
func sequence_CreateDateRangeSeq(rs m.SequenceSet, date dates.Date) m.SequenceDateRangeSet {
	rs.EnsureOne()
	year := date.Year()
	dateFrom := dates.ParseDate(fmt.Sprintf("%d-01-01", year))
	dateTo := dates.ParseDate(fmt.Sprintf("%d-12-31", year))
	dateRange := h.SequenceDateRange().Search(rs.Env(),
		q.SequenceDateRange().Sequence().Equals(rs).
			And().DateFrom().GreaterOrEqual(date).
			And().DateFrom().LowerOrEqual(dateTo)).
		OrderBy("DateFrom DESC").
		Limit(1)
	if !dateRange.IsEmpty() {
		dateTo = dateRange.DateFrom().AddDate(0, 0, -1)
	}
	dateRange = h.SequenceDateRange().Search(rs.Env(),
		q.SequenceDateRange().Sequence().Equals(rs).
			And().DateTo().GreaterOrEqual(dateFrom).
			And().DateTo().LowerOrEqual(date)).
		OrderBy("DateTo DESC").
		Limit(1)
	if !dateRange.IsEmpty() {
		dateTo = dateRange.DateTo().AddDate(0, 0, 1)
	}
	seqDateRange := h.SequenceDateRange().NewSet(rs.Env()).Sudo().Create(h.SequenceDateRange().NewData().
		SetDateFrom(dateFrom).
		SetDateTo(dateTo).
		SetSequence(rs))
	return seqDateRange
}

// Next returns the next number (formatted) in the preferred sequence in all the ones given in self
func sequence_Next(rs m.SequenceSet) string {
	rs.EnsureOne()
	if !rs.UseDateRange() {
		return rs.NextDo()
	}
	// Date mode
	dt := dates.Today()
	if rs.Env().Context().HasKey("sequence_date") {
		dt = rs.Env().Context().GetDate("sequence_date")
	}
	seqDate := h.SequenceDateRange().Search(rs.Env(),
		q.SequenceDateRange().Sequence().Equals(rs).
			And().DateFrom().LowerOrEqual(dt).
			And().DateTo().GreaterOrEqual(dt)).
		Limit(1)
	if seqDate.IsEmpty() {
		seqDate = rs.CreateDateRangeSeq(dt)
	}
	return seqDate.WithContext("sequence_date_range", seqDate.DateFrom()).Next()
}

// NextByID draws an interpolated string using the specified sequence.
func sequence_NextByID(rs m.SequenceSet) string {
	rs.CheckExecutionPermission(h.Sequence().Methods().Read().Underlying())
	return rs.Next()
}

// NextByCode draws an interpolated string using a sequence with the requested code.
// If several sequences with the correct code are available to the user
// (multi-company cases), the one from the user's current company will be used.
//
// The context may contain a 'force_company' key with the ID of the company to
// use instead of the user's current company for the sequence selection.
// A matching sequence for that specific company will get higher priority
func sequence_NextByCode(rs m.SequenceSet, sequenceCode string) string {
	rs.CheckExecutionPermission(h.Sequence().Methods().Read().Underlying())
	companies := h.Company().NewSet(rs.Env()).SearchAll()
	seqs := h.Sequence().Search(rs.Env(),
		q.Sequence().Code().Equals(sequenceCode).AndCond(
			q.Sequence().Company().In(companies).Or().Company().IsNull()))
	if seqs.IsEmpty() {
		log.Debug("No Sequence has been found for this code", "code", sequenceCode, "companies", companies)
		return "False"
	}
	forceCompanyID := rs.Env().Context().GetInteger("force_company")
	if forceCompanyID == 0 {
		forceCompanyID = h.User().NewSet(rs.Env()).CurrentUser().Company().ID()
	}
	for _, seq := range seqs.Records() {
		if seq.Company().ID() == forceCompanyID {
			return seq.Next()
		}
	}
	return seqs.Records()[0].Next()
}

var fields_SequenceDateRange = map[string]models.FieldDefinition{
	"DateFrom": fields.Date{String: "From", Required: true},
	"DateTo":   fields.Date{String: "To", Required: true},
	"Sequence": fields.Many2One{String: "Main Sequence", RelationModel: h.Sequence(),
		Required: true, OnDelete: models.Cascade},
	"NumberNext": fields.Integer{String: "Next Number",
		Required: true, Default: models.DefaultValue(1), Help: "Next number of this sequence"},
	"NumberNextActual": fields.Integer{String: "Next Number",
		Compute: h.SequenceDateRange().Methods().ComputeNumberNextActual(),
		Inverse: h.SequenceDateRange().Methods().InverseNumberNextActual(),
		Help:    "Next number that will be used. This number can be incremented frequently so the displayed value might already be obsolete",
		Depends: []string{"NumberNext"}},
}

// ComputeNumberNextActual returns the real next number for the sequence depending on the implementation
func sequenceDateRange_ComputeNumberNextActual(rs m.SequenceDateRangeSet) m.SequenceDateRangeData {
	res := h.SequenceDateRange().NewData().SetNumberNextActual(rs.NumberNext())
	return res
}

// InverseNumberNextActual is the setter function for the NumberNextActual field
func sequenceDateRange_InverseNumberNextActual(rs m.SequenceDateRangeSet, value int64) {
	if value == 0 {
		value = 1
	}
	rs.SetNumberNext(value)
}

// Next returns the next number (formatted) of this sequence date range.
func sequenceDateRange_Next(rs m.SequenceDateRangeSet) string {
	if rs.Sequence().Implementation() == "standard" {
		hexyaSeq := models.Registry.MustGetSequence(fmt.Sprintf("sequence_%03d_%03d", rs.Sequence().ID(), rs.ID()))
		return rs.Sequence().GetNextChar(hexyaSeq.NextValue())
	}
	return rs.Sequence().GetNextChar(rs.UpdateNoGap())
}

// AlterHexyaSequence alters the date range sequences in one go
func sequenceDateRange_AlterHexyaSequence(rs m.SequenceDateRangeSet, numberIncrement int64, numberNext int64) {
	for _, seq := range rs.Records() {
		hexyaSeq, exists := models.Registry.GetSequence(fmt.Sprintf("sequence_%03d_%03d", seq.Sequence().ID(), seq.ID()))
		if !exists {
			// sequence is not created yet, we're inside create() so ignore it, will be set later
			return
		}
		hexyaSeq.Alter(numberIncrement, numberNext)
	}
}

func sequenceDateRange_Create(rs m.SequenceDateRangeSet, data m.SequenceDateRangeData) m.SequenceDateRangeSet {
	seq := rs.Super().Create(data)
	mainSeq := seq.Sequence()
	if mainSeq.Implementation() == "standard" {
		next := data.NumberNextActual()
		if next == 0 {
			next = 1
		}
		models.CreateSequence(fmt.Sprintf("sequence_%03d_%03d", mainSeq.ID(), seq.ID()),
			mainSeq.NumberIncrement(), next)
	}
	return seq
}

func sequenceDateRange_Unlink(rs m.SequenceDateRangeSet) int64 {
	for _, rec := range rs.Records() {
		hexyaSeq, exists := models.Registry.GetSequence(fmt.Sprintf("sequence_%03d_%03d", rec.Sequence().ID(), rec.ID()))
		if exists {
			hexyaSeq.Drop()
		}
	}
	return rs.Super().Unlink()
}

func sequenceDateRange_Write(rs m.SequenceDateRangeSet, data m.SequenceDateRangeData) bool {
	if data.NumberNext() != 0 {
		seqToAlter := rs.Filtered(func(rs m.SequenceDateRangeSet) bool {
			return rs.Sequence().Implementation() == "standard"
		})
		for _, rec := range seqToAlter.Records() {
			hexyaSeq, exists := models.Registry.GetSequence(fmt.Sprintf("sequence_%03d_%03d", rec.Sequence().ID(), rec.ID()))
			if exists {
				hexyaSeq.Alter(data.NumberNext(), 0)
			}
		}
	}
	return rs.Super().Write(data)
}

// UpdateNoGap gets the next number of a "No Gap" sequence
func sequenceDateRange_UpdateNoGap(rs m.SequenceDateRangeSet) int64 {
	rs.EnsureOne()
	numberNext := rs.NumberNext()
	rs.Env().Cr().Execute(`SELECT number_next FROM sequence_date_range WHERE id=? FOR UPDATE NOWAIT`, rs.ID())
	rs.Env().Cr().Execute(`UPDATE sequence_date_range SET number_next=number_next + ? WHERE id=?`, rs.Sequence().NumberIncrement(), rs.ID())
	rs.Collection().InvalidateCache()
	return numberNext
}

// init
func init() {
	models.NewModel("Sequence")
	h.Sequence().AddFields(fields_Sequence)

	h.Sequence().NewMethod("ComputeNumberNextActual", sequence_ComputeNumberNextActual)
	h.Sequence().NewMethod("InverseNumberNextActual", sequence_InverseNumberNextActual)
	h.Sequence().NewMethod("AlterHexyaSequence", sequence_AlterHexyaSequence)
	h.Sequence().Methods().Create().Extend(sequence_Create)
	h.Sequence().Methods().Unlink().Extend(sequence_Unlink)
	h.Sequence().Methods().Write().Extend(sequence_Write)
	h.Sequence().NewMethod("NextDo", sequence_NextDo)
	h.Sequence().NewMethod("UpdateNoGap", sequence_UpdateNoGap)
	h.Sequence().NewMethod("GetNextChar", sequence_GetNextChar)
	h.Sequence().NewMethod("CreateDateRangeSeq", sequence_CreateDateRangeSeq)
	h.Sequence().NewMethod("Next", sequence_Next)
	h.Sequence().NewMethod("NextByID", sequence_NextByID)
	h.Sequence().NewMethod("NextByCode", sequence_NextByCode)

	models.NewModel("SequenceDateRange")
	h.SequenceDateRange().AddFields(fields_SequenceDateRange)

	h.SequenceDateRange().NewMethod("ComputeNumberNextActual", sequenceDateRange_ComputeNumberNextActual)
	h.SequenceDateRange().NewMethod("InverseNumberNextActual", sequenceDateRange_InverseNumberNextActual)
	h.SequenceDateRange().NewMethod("Next", sequenceDateRange_Next)
	h.SequenceDateRange().NewMethod("AlterHexyaSequence", sequenceDateRange_AlterHexyaSequence)
	h.SequenceDateRange().Methods().Create().Extend(sequenceDateRange_Create)
	h.SequenceDateRange().Methods().Unlink().Extend(sequenceDateRange_Unlink)
	h.SequenceDateRange().Methods().Write().Extend(sequenceDateRange_Write)
	h.SequenceDateRange().NewMethod("UpdateNoGap", sequenceDateRange_UpdateNoGap)
}
