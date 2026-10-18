// Package scheduler runs checks on an interval and watches for a tracker that
// stopped working.
package scheduler

import (
	"context"
	"log"
	"time"

	"dropcatch/internal/checker"
	"dropcatch/internal/notify"
	"dropcatch/internal/store"
)

const StaleLimit = 12 * time.Hour

// Due compares wall clock time, so a laptop that slept past the interval runs
// on the first tick after it wakes.
func Due(lastRun time.Time, interval time.Duration, now time.Time) bool {
	return lastRun.IsZero() || now.Sub(lastRun) >= interval
}

func Stale(lastSuccess time.Time, hasTracked bool, now time.Time, limit time.Duration) bool {
	return hasTracked && now.Sub(lastSuccess) >= limit
}

type Scheduler struct {
	Checker *checker.Checker
	Store   *store.Store
	Notices *notify.Notices
	Now     func() time.Time
	Tick    time.Duration // default 1 minute
}

func (s *Scheduler) Run(ctx context.Context) {
	tick := s.Tick
	if tick == 0 {
		tick = time.Minute
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		s.TickOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// TickOnce checks for staleness, then runs all products if a run is due.
func (s *Scheduler) TickOnce(ctx context.Context) {
	now := s.Now()
	s.checkStale(ctx, now)

	settings, err := s.Store.GetSettings(ctx)
	if err != nil {
		log.Printf("scheduler: %v", err)
		return
	}
	last, err := s.Store.GetTime(ctx, "last_run_at")
	if err != nil {
		log.Printf("scheduler: %v", err)
		return
	}
	if !Due(last, time.Duration(settings.CheckIntervalHours)*time.Hour, now) {
		return
	}
	// Record the start first so a crash mid-run doesn't loop on retries.
	if err := s.Store.SetTime(ctx, "last_run_at", now); err != nil {
		log.Printf("scheduler: %v", err)
		return
	}
	log.Print("running scheduled check")
	if err := s.Checker.RunAll(ctx); err != nil {
		log.Printf("scheduled check: %v", err)
	}
}

func (s *Scheduler) checkStale(ctx context.Context, now time.Time) {
	tracked, err := s.Store.HasTrackedVariants(ctx)
	if err != nil {
		log.Printf("stale check: %v", err)
		return
	}
	last, err := s.Store.GetTime(ctx, "last_success_at")
	if err != nil {
		log.Printf("stale check: %v", err)
		return
	}
	if last.IsZero() {
		// Nothing has ever succeeded. Count from when tracking started so a
		// brand new install doesn't email right away.
		products, _ := s.Store.ListProducts(ctx)
		if len(products) == 0 {
			return
		}
		last = products[0].CreatedAt
	}
	if Stale(last, tracked, now, StaleLimit) {
		subject, text := notify.StaleEmail(last)
		if err := s.Notices.Raise(ctx, "stale", subject, text); err != nil {
			log.Printf("stale email: %v", err)
		}
		return
	}
	_ = s.Notices.Clear(ctx, "stale")
}
