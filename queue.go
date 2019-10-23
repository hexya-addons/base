// Copyright 2019 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/typesutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

// QueueJobPeriod is the delay between two polls of the job queue
const QueueJobPeriod = 10 * time.Millisecond

// QueueJobHoldDelay is the delay that the system waits before polling
// the job queue again if it has not seen any job left on the last poll.
const QueueJobHoldDelay = 500 * time.Millisecond

// QueueJobStates is the selection for the states of the QueueJob model.
var QueueJobStates = types.Selection{
	"pending":  "Pending",
	"enqueued": "Enqueued",
	"started":  "Started",
	"done":     "Done",
	"failed":   "Failed",
}

func init() {
	h.QueueChannel().DeclareModel()
	h.QueueChannel().AddFields(map[string]models.FieldDefinition{
		"Name":     models.CharField{Required: true, Index: true, Unique: true},
		"Capacity": models.IntegerField{Required: true, Default: models.DefaultValue(1), GoType: new(int)},
	})

	h.QueueChannel().Methods().Unlink().Extend("",
		func(rs m.QueueChannelSet) int64 {
			return rs.Filtered(func(r m.QueueChannelSet) bool {
				return r.HexyaExternalID() != "base_default_channel"
			}).Super().Unlink()
		})

	h.QueueJob().DeclareModel()
	h.QueueJob().SetDefaultOrder("Priority", "CreateDate", "ID")
	h.QueueJob().AddFields(map[string]models.FieldDefinition{
		"Name":   models.CharField{Required: true, Index: true},
		"Model":  models.CharField{Required: true, Constraint: h.QueueJob().Methods().CheckParameters()},
		"Method": models.CharField{Required: true, Constraint: h.QueueJob().Methods().CheckParameters()},
		"RecordsIds": models.TextField{Default: models.DefaultValue("[]"), Help: `Use a JSON list format (e.g. [1, 2])`,
			Constraint: h.QueueJob().Methods().CheckParameters()},
		"Arguments": models.TextField{Default: models.DefaultValue("[]"), Constraint: h.QueueJob().Methods().CheckParameters(),
			Help: `Use a JSON list format (e.g. [[1, 2], "My string value", true]).
For relation fields, pass the ID or the list of IDs`},
		"User": models.Many2OneField{RelationModel: h.User(), Required: true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser()
			}},
		"Company": models.Many2OneField{RelationModel: h.Company(), Required: true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company()
			}},
		"Channel": models.Many2OneField{RelationModel: h.QueueChannel(),
			Default: func(env models.Environment) interface{} {
				ch := h.QueueChannel().NewSet(env).GetRecord("base_default_channel")
				return ch
			}},
		"Priority": models.IntegerField{},
		"ExecuteAfterJob": models.Many2OneField{RelationModel: h.QueueJob(), String: "Execute only after",
			Help: `Execute the current job only after this one has been correctly executed`},
		"ExecuteBeforeJobs": models.One2ManyField{RelationModel: h.QueueJob(), ReverseFK: "ExecuteAfterJob",
			Help: `List of jobs that will be executed after the current one`},
		"State": models.SelectionField{Selection: QueueJobStates, Required: true, Index: true, ReadOnly: true,
			Default: models.DefaultValue("pending")},
		"ExcInfo":      models.TextField{String: "Exception Info", ReadOnly: true},
		"Result":       models.TextField{ReadOnly: true},
		"DateStarted":  models.DateTimeField{ReadOnly: true},
		"DateEnqueued": models.DateTimeField{ReadOnly: true},
		"DateDone":     models.DateTimeField{ReadOnly: true},
		"ETA":          models.DateTimeField{String: "Execute only after"},
		"Retry":        models.IntegerField{String: "Current try"},
		"MaxRetries": models.IntegerField{Help: `The job will fail if the number of tries reach the max. retries.
Retries are infinite when equals zero.`},
	})

	h.QueueJob().Methods().CheckParameters().DeclareMethod(
		`CheckParameters checks if model, method, record ids and arguments are correct`,
		func(rs m.QueueJobSet) {
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
		})

	h.QueueJob().Methods().Run().DeclareMethod(
		`Run this job's method with its arguments. It returns the result of the called method as a string if there is 
		a return value, otherwise a default success string. It panics in cas of error.

		Note that this method (and its overrides) MUST NOT modify the current job.`,
		func(rs m.QueueJobSet) string {
			var ids []int64
			json.Unmarshal([]byte(rs.RecordsIds()), &ids)
			records := models.Registry.MustGet(rs.Model()).Browse(rs.Env(), ids)
			var arguments []interface{}
			json.Unmarshal([]byte(rs.Arguments()), &arguments)
			methArgs := make([]interface{}, len(arguments))
			meth := records.Collection().Model().Methods().MustGet(rs.Method())
			for i := 1; i < meth.MethodType().NumIn(); i++ {
				arg := arguments[i-1]
				methArgType := meth.MethodType().In(i)
				switch {
				case methArgType.Implements(reflect.TypeOf((*models.RecordSet)(nil)).Elem()):
					relRc := rs.Env().Pool(records.ModelName())
					typesutils.Convert(arg, relRc, true)
					methArgs[i-1] = relRc
				case methArgType.Implements(reflect.TypeOf((*models.RecordData)(nil)).Elem()):
					relRD := models.NewModelDataFromRS(records)
					typesutils.Convert(arg, relRD, false)
					methArgs[i-1] = relRD
				default:
					methArgs[i-1] = arg
				}
			}

			res := records.Call(rs.Method(), methArgs...)
			if res != nil {
				if resString, ok := res.(string); ok {
					return resString
				}
			}
			return "Job executed successfully."
		})

	h.QueueJob().Methods().OnChannel().DeclareMethod(
		`OnChannel sets the Channel of this job to the channel with the given name`,
		func(rs m.QueueJobSet, channel string) m.QueueJobSet {
			ch := h.QueueChannel().Search(rs.Env(), q.QueueChannel().Name().Equals(channel))
			if ch.IsEmpty() {
				log.Warn("Trying to set non existent channel", "job", rs.ID(), "channel", channel)
				return rs
			}
			rs.SetChannel(ch)
			return rs
		})

	h.QueueJob().Methods().WithPriority().DeclareMethod(
		`WithPriority sets this job with the given priority.`,
		func(rs m.QueueJobSet, priority int64) m.QueueJobSet {
			rs.SetPriority(priority)
			return rs
		})

	h.QueueJob().Methods().AfterJob().DeclareMethod(
		`AfterJob sets this job to execute only when the given job has succeeded.`,
		func(rs m.QueueJobSet, job m.QueueJobSet) m.QueueJobSet {
			rs.SetExecuteAfterJob(job)
			return rs
		})

	h.CommonMixin().Methods().Enqueue().DeclareMethod(
		`Enqueue queues the execution of the given method with the given arguments on this recordset.
		description will be the name given to the job.`,
		func(rs m.CommonMixinSet, description string, method models.Methoder, arguments ...interface{}) m.QueueJobSet {
			jsonArgs, _ := json.Marshal(arguments)
			jsonIds, _ := json.Marshal(rs.Ids())
			job := h.QueueJob().Create(rs.Env(), h.QueueJob().NewData().
				SetName(description).
				SetModel(rs.ModelName()).
				SetMethod(method.Underlying().Name()).
				SetRecordsIds(string(jsonIds)).
				SetArguments(string(jsonArgs)))
			return job
		})

	models.RegisterWorker(models.NewWorkerFunction(runQueueJobs, QueueJobPeriod))
}

// runQueueJobs is registered in the core Hexya loop to run QueueJobs
func runQueueJobs() {
	var (
		jobIDS []int64
		more   bool
	)
	// Step 1: Enqueue candidate jobs on each channel to reach channel capacity and get enqueued job ids
	models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
		candidateCond := q.QueueJob().State().Equals("pending").
			AndCond(q.QueueJob().ExecuteAfterJob().IsNull().
				Or().ExecuteAfterJobFilteredOn(q.QueueJob().State().Equals("done")))
		for _, channel := range h.QueueChannel().NewSet(env).SearchAll().Records() {
			managedJobs := h.QueueJob().Search(env,
				q.QueueJob().Channel().Equals(channel).And().State().In([]string{"enqueued", "running"}))
			toAdd := channel.Capacity() - managedJobs.Len()
			if toAdd <= 0 {
				continue
			}
			candidateJobs := h.QueueJob().Search(env, candidateCond.And().Channel().Equals(channel)).Limit(toAdd)
			candidateJobs.Write(h.QueueJob().NewData().
				SetState("enqueued").
				SetDateEnqueued(dates.Now()))
		}
		enqueuedJobs := h.QueueJob().Search(env, q.QueueJob().State().Equals("enqueued"))
		jobIDS = enqueuedJobs.Ids()
		more = h.QueueJob().Search(env, candidateCond).Limit(1).IsNotEmpty()
	})
	// Step 2: Run enqueued jobs
	for _, jobID := range jobIDS {
		go func(jobID int64) {
			var result string
			// We set our job to running in a separate transaction to tell everyone else
			models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				job := h.QueueJob().BrowseOne(env, jobID)
				job.Write(h.QueueJob().NewData().
					SetState("running").
					SetDateStarted(dates.Now()))
			})
			// We use 2 transactions here to recover the error from running the job.
			err := models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				job := h.QueueJob().BrowseOne(env, jobID)
				result = job.Sudo(job.User().ID()).Run()
			})
			models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				job := h.QueueJob().BrowseOne(env, jobID)
				if err != nil {
					job.Write(h.QueueJob().NewData().
						SetState("failed").
						SetDateDone(dates.Now()).
						SetExcInfo(err.Error()))
					return
				}
				job.Write(h.QueueJob().NewData().
					SetState("done").
					SetDateDone(dates.Now()).
					SetResult(result))
			})
		}(jobID)
	}
	if !more {
		// Calm the system down if there are no more candidate jobs behind
		<-time.After(QueueJobHoldDelay)
	}
}
