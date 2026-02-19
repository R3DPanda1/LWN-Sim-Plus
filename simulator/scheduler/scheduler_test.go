package scheduler

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestSchedulerExecutesJobs(t *testing.T) {
	var counter int64
	s := New(10*time.Millisecond, 100, 4, 100)

	s.Schedule(&Job{
		ID:       1,
		Interval: 20 * time.Millisecond,
		Execute: func() {
			atomic.AddInt64(&counter, 1)
		},
	})

	time.Sleep(100 * time.Millisecond)
	s.Stop()

	count := atomic.LoadInt64(&counter)
	if count < 2 {
		t.Errorf("expected at least 2 executions, got %d", count)
	}
}

func TestSchedulerStop(t *testing.T) {
	s := New(10*time.Millisecond, 100, 4, 100)
	s.Stop() // should not hang
}

func TestSchedulerRemove(t *testing.T) {
	var counter int64
	s := New(10*time.Millisecond, 100, 4, 100)

	s.Schedule(&Job{
		ID:       1,
		Interval: 20 * time.Millisecond,
		Execute: func() {
			atomic.AddInt64(&counter, 1)
		},
	})

	time.Sleep(30 * time.Millisecond)
	s.Remove(1)
	countAtRemove := atomic.LoadInt64(&counter)

	time.Sleep(50 * time.Millisecond)
	s.Stop()

	countAfter := atomic.LoadInt64(&counter)
	if countAfter > countAtRemove+1 {
		t.Errorf("job kept running after removal: %d at remove, %d after", countAtRemove, countAfter)
	}
}
