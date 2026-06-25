package proxy

import (
	"context"
	"fmt"
	"minoxy/config"
	"sync"
	"time"
)

type EventType int

const (
	EventScrapingSource EventType = iota
	EventSourceScraped
	EventScrapingDone
	EventProxyChecked
	EventCheckingDone
	EventScrapeError
)

type RunnerEvent struct {
	Type    EventType
	Payload interface{}
}

type EventSourceScrapedPayload struct {
	Source string
	Count  int
}

type EventProxyCheckedPayload struct {
	Proxy  *CheckedProxy
	IsLive bool
}

type Runner struct {
	mu           sync.Mutex
	Config       *config.Config
	LiveProxies  []*CheckedProxy
	allRaw       []RawProxy
	rawIdx       int
	
	// Stats
	ScrapedCount int
	CheckedCount int
	LiveCount    int
	HTTPCount    int
	Socks4Count  int
	Socks5Count  int
	TotalPing    time.Duration

	// Control flow
	isPaused   bool
	isStopped  bool
	cond       *sync.Cond
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	EventChan chan RunnerEvent
}

func NewRunner(cfg *config.Config) *Runner {
	r := &Runner{
		Config:    cfg,
		EventChan: make(chan RunnerEvent, 100),
	}
	r.cond = sync.NewCond(&r.mu)
	return r
}

func (r *Runner) Start(ctx context.Context) {
	cctx, cancel := context.WithCancel(ctx)
	r.cancelFunc = cancel

	go r.run(cctx)
}

func (r *Runner) Pause() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.isPaused = true
}

func (r *Runner) Resume() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.isPaused = false
	r.cond.Broadcast()
}

func (r *Runner) Stop() {
	r.mu.Lock()
	r.isStopped = true
	r.isPaused = false
	r.cond.Broadcast()
	r.mu.Unlock()

	if r.cancelFunc != nil {
		r.cancelFunc()
	}
}

func (r *Runner) IsPaused() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.isPaused
}

func (r *Runner) run(ctx context.Context) {
	defer close(r.EventChan)

	// Step 1: Scrape all sources concurrently
	var wgScrape sync.WaitGroup
	var muScrape sync.Mutex
	var rawList []RawProxy
	seen := make(map[string]bool)

	for _, src := range r.Config.Sources {
		wgScrape.Add(1)
		go func(url string) {
			defer wgScrape.Done()

			select {
			case <-ctx.Done():
				return
			default:
			}

			r.EventChan <- RunnerEvent{
				Type:    EventScrapingSource,
				Payload: url,
			}

			// Use a 10s context timeout for each source fetch
			sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			scraped, err := ScrapeURL(sctx, url)
			if err != nil {
				r.EventChan <- RunnerEvent{
					Type:    EventScrapeError,
					Payload: fmt.Sprintf("Error %s: %v", url, err),
				}
				return
			}

			muScrape.Lock()
			count := 0
			for _, proxy := range scraped {
				if !seen[proxy.IPPort] {
					seen[proxy.IPPort] = true
					rawList = append(rawList, proxy)
					count++
				}
			}
			r.ScrapedCount = len(rawList)
			muScrape.Unlock()

			r.EventChan <- RunnerEvent{
				Type: EventSourceScraped,
				Payload: EventSourceScrapedPayload{
					Source: url,
					Count:  count,
				},
			}
		}(src)
	}

	wgScrape.Wait()

	r.mu.Lock()
	r.allRaw = rawList
	r.mu.Unlock()

	r.EventChan <- RunnerEvent{
		Type:    EventScrapingDone,
		Payload: len(rawList),
	}

	if len(rawList) == 0 {
		r.EventChan <- RunnerEvent{
			Type:    EventCheckingDone,
			Payload: nil,
		}
		return
	}

	// Step 2: Concurrently check all proxies
	jobsChan := make(chan RawProxy, len(rawList))
	for _, p := range rawList {
		jobsChan <- p
	}
	close(jobsChan)

	threads := r.Config.Threads
	if threads > len(rawList) {
		threads = len(rawList)
	}

	for i := 0; i < threads; i++ {
		r.wg.Add(1)
		go r.worker(ctx, jobsChan)
	}

	r.wg.Wait()

	r.EventChan <- RunnerEvent{
		Type:    EventCheckingDone,
		Payload: r.LiveProxies,
	}
}

func (r *Runner) worker(ctx context.Context, jobs <-chan RawProxy) {
	defer r.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Handle pause/stop controls
		r.mu.Lock()
		if r.isStopped {
			r.mu.Unlock()
			return
		}
		for r.isPaused && !r.isStopped {
			r.cond.Wait()
		}
		if r.isStopped {
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()

		raw, ok := <-jobs
		if !ok {
			return
		}

		// Perform check
		checked, err := CheckProxy(ctx, raw, r.Config.CheckURL, r.Config.Timeout, r.Config.Protocols)

		r.mu.Lock()
		r.CheckedCount++
		isLive := err == nil
		if isLive && checked != nil {
			r.LiveCount++
			r.TotalPing += checked.Ping
			r.LiveProxies = append(r.LiveProxies, checked)

			switch checked.Protocol {
			case "http":
				r.HTTPCount++
			case "socks4":
				r.Socks4Count++
			case "socks5":
				r.Socks5Count++
			}
		}
		r.mu.Unlock()

		// Send event
		r.EventChan <- RunnerEvent{
			Type: EventProxyChecked,
			Payload: EventProxyCheckedPayload{
				Proxy:  checked,
				IsLive: isLive,
			},
		}
	}
}

func (r *Runner) GetStats() (scraped, checked, live, http, s4, s5, dead int, avgPing time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	avg := time.Duration(0)
	if r.LiveCount > 0 {
		avg = r.TotalPing / time.Duration(r.LiveCount)
	}

	return r.ScrapedCount, r.CheckedCount, r.LiveCount, r.HTTPCount, r.Socks4Count, r.Socks5Count, r.CheckedCount - r.LiveCount, avg
}
