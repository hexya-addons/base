// Copyright 2019 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package base

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

type cronData struct {
	name     string
	user     int64
	active   bool
	intType  string
	model    string
	method   string
	args     string
	nextCall dates.DateTime
	toRun    bool
}

var cronTestData = []cronData{
	{name: "cron1", user: 1, active: true, intType: "minutes", model: "Partner", method: "Write", args: `[{"Name": "Asus Cron1"}]`, toRun: true},
	{name: "cron2", user: 1, active: true, intType: "hours", model: "Partner", method: "Write", args: `[{"Name": "Asus Cron2"}]`, toRun: true},
	{name: "cron3", user: 1, active: true, intType: "days", model: "Partner", method: "Write", args: `[{"Name": "Asus Cron3"}]`, toRun: true},
	{name: "cron4", user: 1, active: true, intType: "weeks", model: "Partner", method: "Write", args: `[{"Name": "Asus Cron4"}]`, toRun: true},
	{name: "cron5", user: 1, active: true, intType: "months", model: "Partner", method: "Write", args: `[{"Name": "Asus Cron5"}]`, toRun: true},
	{name: "cronInactive", user: 1, active: false, intType: "months", model: "Partner", method: "Write", args: `[{"Name": "Asus CronInactive"}]`, toRun: false},
	{name: "cronLater", user: 1, active: true, intType: "minutes", model: "Partner", method: "Write", args: `[{"Name": "Asus CronLater"}]`, nextCall: dates.Now().AddWeeks(1), toRun: false},
}

func waitAndCheck(job1ID, job2ID int64, state1, state2, result1, result2 string) {
	var done bool
	tries := 20
	for {
		<-time.After(100 * time.Millisecond)
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			job1 := h.QueueJob().BrowseOne(env, job1ID)
			job2 := h.QueueJob().BrowseOne(env, job2ID)
			if job1.State() == "failed" {
				panic("job1 has failed")
			}
			if job2.State() == "failed" {
				panic("job2 has failed")
			}
			if job1.State() != "done" {
				return
			}
			if job2.State() != "done" {
				return
			}
			done = true
			So(job1.State(), ShouldEqual, state1)
			So(job2.State(), ShouldEqual, state2)
			So(job1.Result(), ShouldEqual, result1)
			So(job2.Result(), ShouldEqual, result2)
			job1.Unlink()
			job2.Unlink()
		}), ShouldBeNil)
		if done {
			break
		}
		if tries <= 0 {
			panic("job did not run before timeout")
		}
		tries--
	}
}

func TestWorkerQueueAndCron(t *testing.T) {
	models.RegisterWorker(models.NewWorkerFunction(runCron, 100*time.Millisecond))
	models.RunWorkerLoop()
	defer models.StopWorkerLoop()

	var (
		jobID, job2ID int64
	)
	Convey("Testing Queue Jobs", t, func() {
		Convey("Creating a job and it should not be processed before the end of the transaction", func() {
			So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				// Creating a new channel to check if we don't have side effects
				h.QueueChannel().Create(env, h.QueueChannel().NewData().SetName("Channel 4"))
				agrolait := h.Partner().NewSet(env).GetRecord("base_res_partner_2")
				agrolait.SetName("Agrolait")
				job := agrolait.Enqueue(
					"Get name", h.Partner().Methods().NameGet()).WithPriority(12)
				So(job.State(), ShouldEqual, "pending")
				<-time.After(300 * time.Millisecond)
				So(job.State(), ShouldEqual, "pending")
				jobID = job.ID()
			}), ShouldBeNil)
		})
		Convey("Waiting for the job to be processed and check result", func() {
			waitAndCheck(jobID, jobID, "done", "done", "Agrolait", "Agrolait")
		})
		Convey("Creating two jobs that must follow each other by priority", func() {
			So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				job1 := h.Partner().NewSet(env).GetRecord("base_res_partner_2").Enqueue(
					"Get name", h.Partner().Methods().NameGet()).WithPriority(12)
				job2 := h.Partner().NewSet(env).GetRecord("base_res_partner_2").Enqueue(
					"Set name", h.Partner().Methods().Write(), h.Partner().NewData().SetName("Agrolait modified")).
					WithPriority(1)
				So(job1.State(), ShouldEqual, "pending")
				So(job2.State(), ShouldEqual, "pending")
				So(job1.Priority(), ShouldEqual, 12)
				So(job2.Priority(), ShouldEqual, 1)
				jobID = job1.ID()
				job2ID = job2.ID()
			}), ShouldBeNil)
		})
		Convey("Job2 should have been executed before job1 (priority)", func() {
			waitAndCheck(jobID, job2ID, "done", "done", "Agrolait modified", "Job executed successfully.")
		})
		Convey("Creating two jobs that must follow each other by link", func() {
			So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				job1 := h.Partner().NewSet(env).GetRecord("base_res_partner_2").Enqueue(
					"Get name", h.Partner().Methods().NameGet()).WithPriority(1)
				job2 := h.Partner().NewSet(env).GetRecord("base_res_partner_2").Enqueue(
					"Set name", h.Partner().Methods().Write(), h.Partner().NewData().SetName("Agrolait modified")).
					WithPriority(10)
				job1.AfterJob(job2)
				So(job1.State(), ShouldEqual, "pending")
				So(job2.State(), ShouldEqual, "pending")
				jobID = job1.ID()
				job2ID = job2.ID()
			}), ShouldBeNil)
		})
		Convey("Job2 should have been executed before job1 (link)", func() {
			waitAndCheck(jobID, job2ID, "done", "done", "Agrolait modified", "Job executed successfully.")
		})

		Convey("Creating and deleting channels should work except for default", func() {
			So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				defChan := h.QueueChannel().Search(env, q.QueueChannel().HexyaExternalID().Equals("base_default_channel"))
				ch4 := h.QueueChannel().Search(env, q.QueueChannel().Name().Equals("Channel 4"))
				So(defChan.IsNotEmpty(), ShouldBeTrue)
				So(ch4.IsNotEmpty(), ShouldBeTrue)
				nb := ch4.Unlink()
				So(nb, ShouldEqual, 1)
				nb2 := defChan.Unlink()
				So(nb2, ShouldEqual, 0)
				So(defChan.IsNotEmpty(), ShouldBeTrue)
			}), ShouldBeNil)
		})
		Convey("Enqueueing on different channels", func() {
			So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				defChan := h.QueueChannel().Search(env, q.QueueChannel().HexyaExternalID().Equals("base_default_channel"))
				ch4 := h.QueueChannel().Search(env, q.QueueChannel().Name().Equals("Channel 4"))
				job := h.Partner().NewSet(env).GetRecord("base_res_partner_3").Enqueue(
					"Get name", h.Partner().Methods().NameGet()).OnChannel("Channel 4")
				So(job.Channel().Equals(ch4), ShouldBeTrue)
				job = h.Partner().NewSet(env).GetRecord("base_res_partner_3").Enqueue(
					"Get name", h.Partner().Methods().NameGet()).OnChannel("Unknown channel")
				So(job.Channel().Equals(defChan), ShouldBeTrue)
			}), ShouldBeNil)
		})
		Convey("Creating a job with wrong model, method, ids or argument should fail", func() {
			So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				jobData := h.QueueJob().NewData().
					SetName("Test Job").
					SetModel("Partner").
					SetMethod("ComputePartnerShare").
					SetRecordsIds("[1,2]").
					SetArguments("[]")
				jobData.SetModel("NoModel")
				So(func() { h.QueueJob().Create(env, jobData) }, ShouldPanicWith, `Unknown model
	model : NoModel
`)
				jobData.SetModel("Partner").SetMethod("NoMethod")
				So(func() { h.QueueJob().Create(env, jobData) }, ShouldPanicWith, `Unknown method in model
	model : Partner
	method : NoMethod
`)
			}), ShouldBeNil)

			err := models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				jobData := h.QueueJob().NewData().
					SetName("Test Job").
					SetModel("Partner").
					SetMethod("ComputePartnerShare").
					SetRecordsIds("[no_ids]").
					SetArguments("[]")
				h.QueueJob().Create(env, jobData)
			})
			So(err, ShouldNotBeNil)
			errTitle := strings.Split(err.Error(), "\n-------")[0]
			So(errTitle, ShouldEqual, `unable to unmarshal RecordIds: invalid character 'o' in literal null (expecting 'u')`)

			err = models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				jobData := h.QueueJob().NewData().
					SetName("Test Job").
					SetModel("Partner").
					SetMethod("ComputePartnerShare").
					SetRecordsIds("[1,2]").
					SetArguments(`[unparseable args`)
				h.QueueJob().Create(env, jobData)
			})
			So(err, ShouldNotBeNil)
			errTitle = strings.Split(err.Error(), "\n-------")[0]
			So(errTitle, ShouldEqual, `unable to unmarshal Arguments: invalid character 'u' looking for beginning of value`)

			err = models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				jobData := h.QueueJob().NewData().
					SetName("Test Job").
					SetModel("Partner").
					SetMethod("ComputePartnerShare").
					SetRecordsIds("[1,2]").
					SetArguments(`["too", "many", "args"]`)
				h.QueueJob().Create(env, jobData)
			})
			So(err, ShouldNotBeNil)
			errTitle = strings.Split(err.Error(), "\n-------")[0]
			So(errTitle, ShouldEqual, `wrong number of arguments given: expect 0 arguments, received [too many args]`)
		})
		Convey("Cleaning up", func() {
			So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				h.QueueChannel().Search(env, q.QueueChannel().Name().Equals("Channel 4")).Unlink()
			}), ShouldBeNil)
		})
	})
	startTime := dates.Now()
	var (
		cronIds []int64
		asusID  int64
	)
	Convey("Testing cron jobs", t, func() {
		Convey("Setup tests", func() {
			So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				So(h.Cron().NewSet(env).SearchAll().IsEmpty(), ShouldBeTrue)
				asusID = h.Partner().NewSet(env).GetRecord("base_res_partner_1").ID()
			}), ShouldBeNil)
		})
		Convey("Creating cron entries", func() {
			So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				for _, c := range cronTestData {
					nc := c.nextCall
					if nc.IsZero() {
						nc = startTime
					}
					id := h.Cron().Create(env, h.Cron().NewData().
						SetName(c.name).
						SetUser(h.User().BrowseOne(env, c.user)).
						SetActive(c.active).
						SetIntervalNumber(1).
						SetIntervalType(c.intType).
						SetModel(c.model).
						SetMethod(c.method).
						SetRecordsIds(fmt.Sprintf("[%d]", asusID)).
						SetArguments(c.args).
						SetNextCall(nc))
					cronIds = append(cronIds, id.ID())
				}
			}), ShouldBeNil)
		})
		Convey("Checking jobs have been created and cron rescheduled", func() {
			<-time.After(300 * time.Millisecond)
			So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
				for _, c := range cronTestData {
					cron := h.Cron().Search(env, q.Cron().Name().Equals(c.name))
					if !c.active {
						So(cron.IsEmpty(), ShouldBeTrue)
						continue
					}
					if !c.toRun {
						So(cron.NextCall().Truncate(time.Millisecond), ShouldEqual, c.nextCall.Truncate(time.Millisecond))
						continue
					}
					job := h.QueueJob().Search(env, q.QueueJob().Name().Equals(fmt.Sprintf("Cron Job: %s", c.name))).Load()
					So(job.IsNotEmpty(), ShouldBeTrue)
					So(job.User().ID(), ShouldEqual, c.user)
					So(job.Model(), ShouldEqual, c.model)
					So(job.Method(), ShouldEqual, c.method)
					So(job.RecordsIds(), ShouldEqual, fmt.Sprintf("[%d]", asusID))
					So(job.Arguments(), ShouldEqual, c.args)

					So(cron.NextCall(), ShouldNotEqual, startTime)
					switch c.intType {
					case "minutes":
						So(cron.NextCall().Truncate(time.Millisecond), ShouldEqual, startTime.Add(time.Minute).Truncate(time.Millisecond))
					case "hours":
						So(cron.NextCall().Truncate(time.Millisecond), ShouldEqual, startTime.Add(time.Hour).Truncate(time.Millisecond))
					case "days":
						So(cron.NextCall().Truncate(time.Millisecond), ShouldEqual, startTime.AddDate(0, 0, 1).Truncate(time.Millisecond))
					case "weeks":
						So(cron.NextCall().Truncate(time.Millisecond), ShouldEqual, startTime.AddDate(0, 0, 7).Truncate(time.Millisecond))
					case "months":
						So(cron.NextCall().Truncate(time.Millisecond), ShouldEqual, startTime.AddDate(0, 1, 0).Truncate(time.Millisecond))
					}
				}
			}), ShouldBeNil)
		})
	})
	Convey("Creating a cron with wrong model, method, ids or argument should fail", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			cData := h.Cron().NewData().
				SetName("Test Cron").
				SetModel("Partner").
				SetMethod("ComputePartnerShare").
				SetRecordsIds("[1,2]").
				SetArguments("[]")
			cData.SetModel("NoModel")
			So(func() { h.Cron().Create(env, cData) }, ShouldPanicWith, `Unknown model
	model : NoModel
`)
			cData.SetModel("Partner").SetMethod("NoMethod")
			So(func() { h.Cron().Create(env, cData) }, ShouldPanicWith, `Unknown method in model
	model : Partner
	method : NoMethod
`)
		}), ShouldBeNil)

		err := models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			cData := h.Cron().NewData().
				SetName("Test Cron").
				SetModel("Partner").
				SetMethod("ComputePartnerShare").
				SetRecordsIds("[no_ids]").
				SetArguments("[]")
			h.Cron().Create(env, cData)
		})
		So(err, ShouldNotBeNil)
		errTitle := strings.Split(err.Error(), "\n-------")[0]
		So(errTitle, ShouldEqual, `unable to unmarshal RecordIds: invalid character 'o' in literal null (expecting 'u')`)

		err = models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			cData := h.Cron().NewData().
				SetName("Test Cron").
				SetModel("Partner").
				SetMethod("ComputePartnerShare").
				SetRecordsIds("[1,2]").
				SetArguments(`[unparseable args`)
			h.Cron().Create(env, cData)
		})
		So(err, ShouldNotBeNil)
		errTitle = strings.Split(err.Error(), "\n-------")[0]
		So(errTitle, ShouldEqual, `unable to unmarshal Arguments: invalid character 'u' looking for beginning of value`)

		err = models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			cData := h.Cron().NewData().
				SetName("Test Cron").
				SetModel("Partner").
				SetMethod("ComputePartnerShare").
				SetRecordsIds("[1,2]").
				SetArguments(`["too", "many", "args"]`)
			h.Cron().Create(env, cData)
		})
		So(err, ShouldNotBeNil)
		errTitle = strings.Split(err.Error(), "\n-------")[0]
		So(errTitle, ShouldEqual, `wrong number of arguments given: expect 0 arguments, received [too many args]`)
	})
	Convey("Cleaning crons and jobs", t, func() {
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			h.Partner().BrowseOne(env, asusID).SetName("ASUSTeK")
			h.Cron().Browse(env, cronIds).Unlink()
		}), ShouldBeNil)
		So(models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			h.QueueJob().Search(env, q.QueueJob().Name().Contains("Cron Job: ")).Unlink()
		}), ShouldBeNil)
	})
}
