// Copyright 2019 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fields"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

var fields_Cron = map[string]models.FieldDefinition{
	"Name": fields.Char{Required: true},
	"User": fields.Many2One{RelationModel: h.User(), Required: true, Default: func(env models.Environment) interface{} {
		return h.User().NewSet(env).CurrentUser()
	}},
	"Active":         fields.Boolean{Default: models.DefaultValue(true)},
	"IntervalNumber": fields.Integer{Default: models.DefaultValue(1), Help: "Repeat every x.", GoType: new(int)},
	"IntervalType": fields.Selection{Selection: types.Selection{
		"minutes": "Minutes",
		"hours":   "Hours",
		"days":    "Days",
		"weeks":   "Weeks",
		"months":  "Months",
	}, String: "Interval Unit", Default: models.DefaultValue("months")},
	"NextCall": fields.DateTime{String: "Next Execution Date", Required: true, Default: models.DefaultValue(dates.Now()),
		Help: "Next planned execution date for this job."},
	"Model":  fields.Char{Required: true, Constraint: h.Cron().Methods().CheckParameters()},
	"Method": fields.Char{Required: true, Constraint: h.Cron().Methods().CheckParameters()},
	"RecordsIds": fields.Text{Default: models.DefaultValue("[]"), Constraint: h.Cron().Methods().CheckParameters(),
		Help: `Use a JSON list format (e.g. [1, 2])`},
	"Arguments": fields.Text{Default: models.DefaultValue("[]"), Constraint: h.Cron().Methods().CheckParameters(),
		Help: `Use a JSON list format (e.g. [[1, 2], "My string value", true]).
For relation fields, pass the ID or the list of IDs`},
}

// CheckParameters checks if model, method, record ids and arguments are correct
func cron_CheckParameters(rs m.QueueJobSet) {
	// Check model exists
	relModel := models.Registry.MustGet(rs.Model())
	// Check we can parse ids
	var ids []int64
	if err := json.Unmarshal([]byte(rs.RecordsIds()), &ids); err != nil {
		panic(fmt.Errorf("unable to unmarshal RecordIds: %s", err))
	}
	// Check we can parse arguments
	var arguments []interface{}
	if err := json.Unmarshal([]byte(rs.Arguments()), &arguments); err != nil {
		panic(fmt.Errorf("unable to unmarshal Arguments: %s", err))
	}
	// Check we have the right number of arguments
	meth := relModel.Methods().MustGet(rs.Method())
	if len(arguments) != meth.MethodType().NumIn()-1 {
		panic(fmt.Errorf("wrong number of arguments given: expect %d arguments, received %v", meth.MethodType().NumIn()-1, arguments))
	}
}

// GetFutureCall returns the DateTime of the call after NextCall.`,
func cron_GetFutureCall(rs m.CronSet) dates.DateTime {
	var res dates.DateTime
	switch rs.IntervalType() {
	case "minutes":
		res = rs.NextCall().Add(time.Duration(rs.IntervalNumber()) * time.Minute)
	case "hours":
		res = rs.NextCall().Add(time.Duration(rs.IntervalNumber()) * time.Hour)
	case "days":
		res = rs.NextCall().AddDate(0, 0, rs.IntervalNumber())
	case "weeks":
		res = rs.NextCall().AddWeeks(rs.IntervalNumber())
	case "months":
		res = rs.NextCall().AddDate(0, rs.IntervalNumber(), 0)
	}
	return res
}

func init() {
	models.NewModel("Cron")
	h.Cron().AddFields(fields_Cron)

	h.Cron().NewMethod("CheckParameters", cron_CheckParameters)
	h.Cron().NewMethod("FutureCallDate", cron_GetFutureCall)

	models.RegisterWorker(models.NewWorkerFunction(runCron, 30*time.Second))
}

// runCron is registered in the core Hexya loop to check and run crons.
func runCron() {
	var cronIds []int64
	models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		crons := h.Cron().Search(env, q.Cron().NextCall().Lower(dates.Now()))
		for _, cron := range crons.Records() {
			h.QueueJob().Create(env, h.QueueJob().NewData().
				SetName(fmt.Sprintf("Cron Job: %s", cron.Name())).
				SetModel(cron.Model()).
				SetMethod(cron.Method()).
				SetRecordsIds(cron.RecordsIds()).
				SetArguments(cron.Arguments()).
				SetUser(cron.User()))
		}
		cronIds = crons.Ids()
	})
	// Set next call in a different transaction in case creating the job failed and rolled back
	models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		for _, cron := range h.Cron().Browse(env, cronIds).Records() {
			cron.SetNextCall(cron.FutureCallDate())
		}
	})
}
