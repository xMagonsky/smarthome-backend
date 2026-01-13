package scheduler

import (
	"context"
	"log"
	"smarthome/internal/db"
	"smarthome/internal/taskqueue"
	"sync"

	"github.com/robfig/cron/v3"
)

// Scheduler manages time-based triggers
type Scheduler struct {
	cron      *cron.Cron
	db        *db.DB
	jobMap    map[string]cron.EntryID // Maps schedule ID to cron entry ID
	jobMapMux sync.RWMutex            // Protects jobMap
}

// NewScheduler creates a scheduler
func NewScheduler(dbConn *db.DB) *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		db:     dbConn,
		jobMap: make(map[string]cron.EntryID),
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.cron.Start()
	log.Println("SCHEDULER: Cron scheduler started")
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("SCHEDULER: Cron scheduler stopped")
}

// AddJob adds a cron job and returns the entry ID
func (s *Scheduler) AddJob(spec string, fn func()) (cron.EntryID, error) {
	return s.cron.AddFunc(spec, fn)
}

// LoadSchedules dynamically loads all schedules from database and adds them as cron jobs
// This should be called during initialization and whenever schedules are updated
func (s *Scheduler) LoadSchedules() error {
	schedules, err := s.db.GetAllSchedules(context.Background())
	if err != nil {
		log.Printf("SCHEDULER: Failed to load schedules: %v", err)
		return err
	}

	log.Printf("SCHEDULER: Loading %d schedules from database", len(schedules))

	for _, sch := range schedules {
		if sch.Enabled {
			ruleID := sch.RuleID // Capture the variable to avoid closure issue
			scheduleID := sch.ID

			entryID, err := s.AddJob(sch.CronExpression, func() {
				log.Printf("SCHEDULER: Cron job triggered for rule %s (schedule %s)", ruleID, scheduleID)
				if err := taskqueue.EnqueueEvaluation(ruleID, ""); err != nil {
					log.Printf("SCHEDULER: Failed to enqueue evaluation for rule %s: %v", ruleID, err)
				}
			})

			if err != nil {
				log.Printf("SCHEDULER: Failed to schedule rule %s with cron '%s': %v", ruleID, sch.CronExpression, err)
				continue
			}

			s.jobMapMux.Lock()
			s.jobMap[scheduleID] = entryID
			s.jobMapMux.Unlock()

			log.Printf("SCHEDULER: Scheduled rule %s with cron '%s' (entry ID: %d)", ruleID, sch.CronExpression, entryID)
		} else {
			log.Printf("SCHEDULER: Skipping disabled schedule %s for rule %s", sch.ID, sch.RuleID)
		}
	}

	log.Printf("SCHEDULER: Successfully loaded %d enabled schedules", len(s.jobMap))
	return nil
}

// ReloadSchedules removes all existing schedules and reloads them from database
// This should be called when schedules are added, updated, or deleted
func (s *Scheduler) ReloadSchedules() error {
	log.Println("SCHEDULER: Reloading all schedules")

	// Remove all existing scheduled jobs
	s.jobMapMux.Lock()
	for schedID, entryID := range s.jobMap {
		s.cron.Remove(entryID)
		log.Printf("SCHEDULER: Removed schedule %s (entry ID: %d)", schedID, entryID)
	}
	s.jobMap = make(map[string]cron.EntryID)
	s.jobMapMux.Unlock()

	// Reload schedules from database
	return s.LoadSchedules()
}

// RemoveSchedule removes a specific schedule by its ID
func (s *Scheduler) RemoveSchedule(scheduleID string) {
	s.jobMapMux.Lock()
	defer s.jobMapMux.Unlock()

	if entryID, exists := s.jobMap[scheduleID]; exists {
		s.cron.Remove(entryID)
		delete(s.jobMap, scheduleID)
		log.Printf("SCHEDULER: Removed schedule %s (entry ID: %d)", scheduleID, entryID)
	}
}

// AddOrUpdateSchedule adds or updates a single schedule
func (s *Scheduler) AddOrUpdateSchedule(scheduleID, ruleID, cronExpression string, enabled bool) error {
	// Remove existing schedule if it exists
	s.RemoveSchedule(scheduleID)

	if !enabled {
		log.Printf("SCHEDULER: Schedule %s is disabled, not adding", scheduleID)
		return nil
	}

	// Add new schedule
	entryID, err := s.AddJob(cronExpression, func() {
		log.Printf("SCHEDULER: Cron job triggered for rule %s (schedule %s)", ruleID, scheduleID)
		if err := taskqueue.EnqueueEvaluation(ruleID, ""); err != nil {
			log.Printf("SCHEDULER: Failed to enqueue evaluation for rule %s: %v", ruleID, err)
		}
	})

	if err != nil {
		log.Printf("SCHEDULER: Failed to add/update schedule %s with cron '%s': %v", scheduleID, cronExpression, err)
		return err
	}

	s.jobMapMux.Lock()
	s.jobMap[scheduleID] = entryID
	s.jobMapMux.Unlock()

	log.Printf("SCHEDULER: Added/updated schedule %s for rule %s with cron '%s' (entry ID: %d)", scheduleID, ruleID, cronExpression, entryID)
	return nil
}

// GetScheduledJobCount returns the number of currently scheduled jobs
func (s *Scheduler) GetScheduledJobCount() int {
	s.jobMapMux.RLock()
	defer s.jobMapMux.RUnlock()
	return len(s.jobMap)
}
