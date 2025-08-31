package scheduler

import (
	"context"
	"smarthome/internal/db"
	"smarthome/internal/taskqueue"

	"github.com/robfig/cron/v3"
)

// Scheduler manages time-based triggers
type Scheduler struct {
	cron *cron.Cron
	db   *db.DB
}

// NewScheduler creates a scheduler
func NewScheduler(dbConn *db.DB) *Scheduler {
	return &Scheduler{
		cron: cron.New(),
		db:   dbConn,
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// AddJob adds a cron job
func (s *Scheduler) AddJob(spec string, fn func()) (cron.EntryID, error) {
	return s.cron.AddFunc(spec, fn)
}

// For expansion: LoadSchedules dynamically loads and adds jobs
func (s *Scheduler) LoadSchedules() error {
	schedules, err := s.db.GetAllSchedules(context.Background())
	if err != nil {
		return err
	}
	for _, sch := range schedules {
		if sch.Enabled {
			s.AddJob(sch.CronExpression, func() {
				taskqueue.EnqueueEvaluation(sch.RuleID, "")
			})
		}
	}
	return nil
}

// Expand with dynamic reload on config change
