package scheduler

import (
	"log/slog"
	"sync"
	"time"
)

type Job struct {
	ID       int
	Execute  func()
	Interval time.Duration
}

type bucket struct {
	mu   sync.Mutex
	jobs []*Job
}

type Scheduler struct {
	wheel      []*bucket
	resolution time.Duration
	numBuckets int
	current    int
	workQueue  chan *Job
	stopCh     chan struct{}
	wg         sync.WaitGroup
	mu         sync.Mutex
}

func New(resolution time.Duration, numBuckets int, workerCount int, queueSize int) *Scheduler {
	s := &Scheduler{
		wheel:      make([]*bucket, numBuckets),
		resolution: resolution,
		numBuckets: numBuckets,
		workQueue:  make(chan *Job, queueSize),
		stopCh:     make(chan struct{}),
	}
	for i := range s.wheel {
		s.wheel[i] = &bucket{}
	}

	for i := 0; i < workerCount; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}

	s.wg.Add(1)
	go s.tick()

	return s
}

func (s *Scheduler) Schedule(job *Job) {
	ticks := int(job.Interval / s.resolution)
	if ticks <= 0 {
		ticks = 1
	}

	s.mu.Lock()
	idx := (s.current + ticks) % s.numBuckets
	s.mu.Unlock()

	b := s.wheel[idx]
	b.mu.Lock()
	b.jobs = append(b.jobs, job)
	b.mu.Unlock()
}

func (s *Scheduler) Remove(jobID int) {
	for _, b := range s.wheel {
		b.mu.Lock()
		for i, j := range b.jobs {
			if j.ID == jobID {
				b.jobs = append(b.jobs[:i], b.jobs[i+1:]...)
				b.mu.Unlock()
				return
			}
		}
		b.mu.Unlock()
	}
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *Scheduler) tick() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.resolution)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.mu.Lock()
			s.current = (s.current + 1) % s.numBuckets
			idx := s.current
			s.mu.Unlock()

			b := s.wheel[idx]
			b.mu.Lock()
			jobs := b.jobs
			b.jobs = nil
			b.mu.Unlock()

			for _, job := range jobs {
				select {
				case s.workQueue <- job:
				default:
					slog.Warn("work queue full, dropping job", "job_id", job.ID)
				}
			}

		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) worker(id int) {
	defer s.wg.Done()
	for {
		select {
		case job := <-s.workQueue:
			job.Execute()
			// Re-schedule for next interval
			s.Schedule(job)
		case <-s.stopCh:
			return
		}
	}
}
