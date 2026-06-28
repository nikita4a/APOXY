package proxy

import (
	"apoxy/config"
	"context"
	"sync"
	"time"
)

type EventType int
const (
	EventScanStart EventType = iota
	EventSourceDone
	EventProxyChecked
	EventScanDone
	EventError
)

type RunnerEvent struct {
	Type    EventType
	Payload interface{}
}

type SourceDonePayload struct{ Source string; Endpoints int }
type ProxyCheckedPayload struct{ Result *APIProxyResult }

type Runner struct {
	mu          sync.Mutex
	Config      *config.Config
	Results     []*APIProxyResult
	ScannedCount, AliveCount, TotalModels int
	TotalLatency time.Duration
	isPaused, isStopped bool
	cond        *sync.Cond
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
	EventChan   chan RunnerEvent
}

func NewRunner(cfg *config.Config) *Runner {
	r := &Runner{Config: cfg, EventChan: make(chan RunnerEvent, 200)}
	r.cond = sync.NewCond(&r.mu)
	return r
}

func (r *Runner) Start(ctx context.Context) {
	cctx, cancel := context.WithCancel(ctx)
	r.cancelFn = cancel
	go r.run(cctx)
}

func (r *Runner) Pause()  { r.mu.Lock(); r.isPaused = true; r.mu.Unlock() }
func (r *Runner) Resume() { r.mu.Lock(); r.isPaused = false; r.cond.Broadcast(); r.mu.Unlock() }
func (r *Runner) Stop()   { r.mu.Lock(); r.isStopped = true; r.isPaused = false; r.cond.Broadcast(); r.mu.Unlock(); if r.cancelFn != nil { r.cancelFn() } }
func (r *Runner) IsPaused() bool { r.mu.Lock(); defer r.mu.Unlock(); return r.isPaused }

func (r *Runner) run(ctx context.Context) {
	defer close(r.EventChan)
	sources := append(r.Config.Sources, AddBuiltinSources()...)
	endpoints, _ := ScrapeAPISources(ctx, sources)
	r.EventChan <- RunnerEvent{Type: EventScanStart, Payload: len(endpoints)}
	if len(endpoints) == 0 {
		r.EventChan <- RunnerEvent{Type: EventScanDone, Payload: r.Results}
		return
	}
	jobs := make(chan RawAPIEndpoint, len(endpoints))
	for _, ep := range endpoints { jobs <- ep }
	close(jobs)
	threads := r.Config.Threads
	if threads > len(endpoints) { threads = len(endpoints) }
	for i := 0; i < threads; i++ { r.wg.Add(1); go r.worker(ctx, jobs) }
	r.wg.Wait()
	r.EventChan <- RunnerEvent{Type: EventScanDone, Payload: r.Results}
}

func (r *Runner) worker(ctx context.Context, jobs <-chan RawAPIEndpoint) {
	defer r.wg.Done()
	for {
		select {
		case <-ctx.Done(): return
		default:
		}
		r.mu.Lock()
		if r.isStopped { r.mu.Unlock(); return }
		for r.isPaused && !r.isStopped { r.cond.Wait() }
		r.mu.Unlock()
		ep, ok := <-jobs
		if !ok { return }
		result := CheckAPIProxy(ctx, ep.URL, r.Config.Timeout, r.Config.CheckModels)
		r.mu.Lock()
		r.ScannedCount++
		if result.Alive { r.AliveCount++; r.TotalLatency += result.Latency; r.TotalModels += result.ModelsCount; r.Results = append(r.Results, result) }
		r.mu.Unlock()
		r.EventChan <- RunnerEvent{Type: EventProxyChecked, Payload: ProxyCheckedPayload{Result: result}}
	}
}

func (r *Runner) GetStats() (scanned, alive, totalModels int, avgLatency time.Duration) {
	r.mu.Lock(); defer r.mu.Unlock()
	avg := time.Duration(0)
	if r.AliveCount > 0 { avg = r.TotalLatency / time.Duration(r.AliveCount) }
	return r.ScannedCount, r.AliveCount, r.TotalModels, avg
}
