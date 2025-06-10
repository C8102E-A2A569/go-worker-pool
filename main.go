package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

type Worker struct {
	id     int
	ctx    context.Context
	cancel context.CancelFunc
	jobs   <-chan string
	wg     *sync.WaitGroup
}

func NewWorker(id int, jobs <-chan string, wg *sync.WaitGroup) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{id: id, ctx: ctx, cancel: cancel, jobs: jobs, wg: wg}
}

func (w *Worker) Run() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		log.Printf("Worker %d started", w.id)
		for {
			select {
			case <-w.ctx.Done():
				log.Printf("Worker %d stopped", w.id)
				return
			case job, ok := <-w.jobs:
				if !ok {
					log.Printf("Worker %d: channel closed", w.id)
					return
				}
				log.Printf("[Worker %d] processing: %s", w.id, job)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
}

func (w *Worker) Stop() {
	w.cancel()
}

type Pool struct {
	mu      sync.Mutex
	jobs    chan string
	workers map[int]*Worker
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	nextID  int
}

func NewPool(initialWorkers int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{
		jobs:    make(chan string, 100),
		workers: make(map[int]*Worker),
		ctx:     ctx,
		cancel:  cancel,
		nextID:  1,
	}

	log.Printf("Creating pool with %d workers", initialWorkers)
	for i := 0; i < initialWorkers; i++ {
		p.addWorker()
	}
	return p
}

func (p *Pool) addWorker() {
	worker := NewWorker(p.nextID, p.jobs, &p.wg)
	p.workers[p.nextID] = worker
	worker.Run()
	log.Printf("Added worker %d (total: %d)", p.nextID, len(p.workers))
	p.nextID++
}

func (p *Pool) AddWorker() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.addWorker()
}

func (p *Pool) RemoveWorker() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.workers) == 0 {
		log.Println("No workers to remove")
		return
	}

	for id, worker := range p.workers {
		worker.Stop()
		delete(p.workers, id)
		log.Printf("Removed worker %d (total: %d)", id, len(p.workers))
		break
	}
}

func (p *Pool) Submit(job string) {
	select {
	case p.jobs <- job:
	case <-p.ctx.Done():
		log.Printf("Dropped job: %s", job)
	}
}

func (p *Pool) Shutdown() {
	log.Println("Shutting down pool...")
	p.cancel()

	p.mu.Lock()
	for _, w := range p.workers {
		w.Stop()
	}
	p.mu.Unlock()

	close(p.jobs)
	p.wg.Wait()
	log.Println("Pool shutdown complete")
}

func main() {
	pool := NewPool(2)

	for i := 1; i <= 5; i++ {
		pool.Submit(fmt.Sprintf("job-%d", i))
	}
	time.Sleep(500 * time.Millisecond)

	log.Println("Adding worker...")
	pool.AddWorker()

	for i := 6; i <= 10; i++ {
		pool.Submit(fmt.Sprintf("job-%d", i))
	}
	time.Sleep(500 * time.Millisecond)

	log.Println("Removing worker...")
	pool.RemoveWorker()

	for i := 11; i <= 15; i++ {
		pool.Submit(fmt.Sprintf("job-%d", i))
	}
	time.Sleep(500 * time.Millisecond)

	pool.Shutdown()
	log.Println("All done.")
}
