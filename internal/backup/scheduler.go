package backup

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

type Scheduler struct {
	mu      sync.Mutex
	cron    *cron.Cron
	mgr     *Manager
	running bool
}

func NewScheduler(mgr *Manager) *Scheduler {
	return &Scheduler{mgr: mgr}
}

func (s *Scheduler) Start(cronExpr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return nil
	}
	if cronExpr == "" {
		log.Info("backup: no cron expression configured, scheduler idle")
		return nil
	}
	s.cron = cron.New(cron.WithLocation(time.Now().Location()))
	_, err := s.cron.AddFunc(cronExpr, func() {
		log.Info("backup: starting scheduled backup")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		name, err := s.mgr.CreateBackupWithSource(ctx, false, "cron")
		if err != nil {
			log.Errorf("backup: scheduled backup failed: %v", err)
		} else {
			log.Infof("backup: scheduled backup created: %s", name)
		}
	})
	if err != nil {
		return err
	}
	s.cron.Start()
	s.running = true
	log.Infof("backup: scheduler started with cron %q", cronExpr)
	return nil
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil {
		s.cron.Stop()
		s.cron = nil
	}
	s.running = false
	log.Info("backup: scheduler stopped")
}

func (s *Scheduler) Reload(cronExpr string) {
	s.Stop()
	if err := s.Start(cronExpr); err != nil {
		log.Errorf("backup: scheduler reload failed: %v", err)
	}
}
