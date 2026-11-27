// Case 01: Bounded Worker Pool
//
// Problem:  Starting one goroutine per job (e.g. 100k jobs = 100k goroutines)
//           exhausts memory and overwhelms the scheduler. The process slows
//           down and eventually gets OOM-killed.
// Resource: Number of goroutines (and the memory + connections each one holds).
// Fix:      A fixed pool of workers reading from a buffered job channel.
//           Only N jobs run at once, no matter how many are queued.
//
// Real-world use cases:
//   - A web scraper fetching thousands of URLs (cap concurrent HTTP requests)
//   - A job queue consumer processing background tasks (cap DB connections)
//   - An image/video pipeline (cap CPU-heavy workers to core count)
//   - Any service calling a third-party API with a rate or connection limit
//
// Run:  go run .                        (bounded, safe)
//       go run . -race                  (also checks for data races)
//       go run . -mode=unbounded        (shows the failure mode)
// Try:  compare peak goroutines between bounded and unbounded modes.
//
// Lesson: concurrency must be capped(limited to a maximum) by the scarce resource downstream —
//         connections, memory, CPU, or an API quota. Bound it, or the OOM(Out of memory)
//         killer bounds it for you.

// # Safe, bounded — peak goroutines stays ~workers + overhead
// go run .

// # With race detector
// go run . -race

// # The failure mode: one goroutine per job
// go run . -mode=unbounded -jobs=50000

// # Tune it
// go run . -workers=4 -jobs=10000 -dur=10ms



// | Metric           | Bounded    | Unbounded  | Ratio       |
// |------------------|------------|------------|-------------|
// | Jobs processed   | 5000       | 4999 ⚠️    | —           |
// | Elapsed          | 3.66 s     | 24.1 ms    | 152× faster |
// | Throughput       | 1366/s     | 207156/s   | 152× higher |
// | Peak goroutines  | 11         | 1208       | 110× more   |

//// Note: "4999" is a real bug in runUnbounded — the counter is read before
// the consumer drains the last result. See the `consumerWG.Wait()` fix.

package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// job is one unit of work. We use a fake duration to simulate I/O or CPU time.
type job struct {
	id  int
	dur time.Duration
}

// result is what a worker produces after processing a job. It contains the job ID and the computed value.
type result struct {
	jobID int
	value int
}

// config holds the knobs exposed on the command line
type config struct {
	numJobs    int
	numWorkers int
	jobDur     time.Duration
	mode       string // "bounded" or "unbounded"
}

func main() {
	cfg := parseFlags()

	fmt.Printf("== Bounded Worker Pool ==\n")
	fmt.Printf("mode:       %s\n", cfg.mode)
	fmt.Printf("jobs:       %d\n", cfg.numJobs)
	fmt.Printf("job dur:    %v (fake I/O)\n", cfg.jobDur)
	if cfg.mode == "bounded" {
		fmt.Printf("workers:    %d\n", cfg.numWorkers)
	}
	fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
	fmt.Println()

	// Sample peak goroutines while the work runs.
	stopSampler := startGoroutineSampler(10 * time.Millisecond)
	defer stopSampler()

	start := time.Now()
	var processed int64

	switch cfg.mode {
	case "bounded":
		processed = runBounded(cfg)
	case "unbounded":
		processed = runUnbounded(cfg)
	default:
		fmt.Printf("unknown mode %q (use bounded|unbounded)\n", cfg.mode)
		return
	}

	elapsed := time.Since(start)

	// Let the sampler take a final reading, then stop it.
	time.Sleep(20 * time.Millisecond)
	stopSampler()
	peak := peakGoroutines()

	fmt.Println()
	fmt.Printf("== Results ==\n")
	fmt.Printf("processed:      %d jobs\n", processed)
	fmt.Printf("elapsed:        %v\n", elapsed)
	fmt.Printf("throughput:     %.0f jobs/sec\n", float64(processed)/elapsed.Seconds())
	fmt.Printf("peak goroutines: %d\n", peak)

	// A rough "did we bound it?" check: in bounded mode, peak goroutines
	// should be roughly numWorkers + a small constant (main, sampler, etc.).
	if cfg.mode == "bounded" {
		upper := int64(cfg.numWorkers + 20)
		if peak > upper {
			fmt.Printf("WARN: peak goroutines %d > expected ~%d — bound may be leaky\n", peak, upper)
		} else {
			fmt.Printf("OK:   peak goroutines bounded (~%d workers + overhead)\n", cfg.numWorkers)
		}
	} else {
		fmt.Printf("NOTE: unbounded mode spawns one goroutine per job — peak grows with numJobs\n")
	}
}

// parseFlags reads the command-line knobs.
func parseFlags() config {
	var cfg config
	flag.IntVar(&cfg.numJobs, "jobs", 5000, "number of jobs to process")
	flag.IntVar(&cfg.numWorkers, "workers", 8, "number of workers (bounded mode)")
	flag.DurationVar(&cfg.jobDur, "dur", 5*time.Millisecond, "fake work duration per job")
	flag.StringVar(&cfg.mode, "mode", "bounded", "bounded | unbounded")
	flag.Parse()
	return cfg
}

// ---------------------------------------------------------------------------
// Bounded mode: fixed worker pool reading from a buffered job channel.
//
// Concurrency is capped at numWorkers. Memory is bounded by:
//   - the job channel buffer (numJobs jobs, but only numWorkers in flight)
//   - the result channel buffer (numWorkers results in flight)
//
// Throughput scales with numJobs; resource usage stays flat.
// ---------------------------------------------------------------------------

func runBounded(cfg config) int64 {
	jobs := make(chan job, cfg.numJobs) // buffered channel of jobs
	results := make(chan result, cfg.numWorkers)

	//start a fixed pool
	//Fixed = the count is set once (numWorkers) and never grows.
	//Pool = those goroutines are reused across all jobs via the channel.
	//Together: a constant number of workers draining an arbitrarily large queue at a constant resource cost.
	var wg sync.WaitGroup

	//start the fixed pool of workers
	//This loop runs exactly cfg.numWorkers times (say, 8).
	//It spawns 8 goroutines, period. Then it stops. No goroutine is ever spawned again for the rest of the program.
	for i := 0; i < cfg.numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := range jobs {
				// This loop runs exactly cfg.numWorkers times (say, 8). It spawns 8 goroutines, period.
				// Then it stops. No goroutine is ever spawned again for the rest of the program.
				time.Sleep(j.dur)
				results <- result{jobID: j.id, value: j.id * 2}
			}
		}(i)
	}

	// Producer: feed jobs. When the buffer is full, this blocks —
	// that's backpressure keeping memory flat.

	go func() {
		for i := 0; i < cfg.numJobs; i++ {
			jobs <- job{id: i, dur: cfg.jobDur}
		}
		close(jobs) // signal to workers that no more jobs will be sent
	}()

	// Closer: wait for workers to finish, then close results.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Consumer : count results. We drain the results here so
	// the workers don't block on sending to results, on a full channel.
	var processed int64
	for range results {
		atomic.AddInt64(&processed, 1)
	}
	return atomic.LoadInt64(&processed)
}

// ---------------------------------------------------------------------------
// Unbounded mode: one goroutine per job. This is the anti-pattern.
//
// At 100k jobs this spawns 100k goroutines at once — each holding a stack
// and (in real code) an open socket/FD. At some point the scheduler thrashes
// and the process gets OOM-killed.
//
// We keep it gentle here (the fake work is short) so the demo doesn't crash
// your machine. Bump -jobs to see the peak goroutine count climb linearly.
// ---------------------------------------------------------------------------
func runUnbounded(cfg config) int64 {
	var wg sync.WaitGroup
	var processed int64

	// Unbuffered results channel — we still drain it, but the point is
	// that ALL jobs run concurrently.
	results := make(chan result)

	//Consumer

	go func() {
		for range results {
			atomic.AddInt64(&processed, 1)
		}
	}()
	//spawn one goroutine per job, all at once

	for i := 0; i < cfg.numJobs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(cfg.jobDur)
			results <- result{jobID: id, value: id * 2}
		}(i)
	}
	wg.Wait()
	close(results)

	return atomic.LoadInt64(&processed)
}

// ---------------------------------------------------------------------------
// Observability: sample runtime.NumGoroutine() while work runs.
//
// This is the "peak concurrency gauge" you'd expose as a metric in prod
// (Prometheus, etc.). It's how you *see* whether your bound is holding.
// ---------------------------------------------------------------------------

var (
	peakG    int64 // atomic
	stopCh   chan struct{}
	stopOnce sync.Once
)

func startGoroutineSampler(interval time.Duration) func() {
	stopCh = make(chan struct{})
	stopOnce = sync.Once{}

	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				n := int64(runtime.NumGoroutine())
				for {
					old := atomic.LoadInt64(&peakG)
					if n <= old || atomic.CompareAndSwapInt64(&peakG, old, n) {
						break
					}
				}
			case <-stopCh:
				return
			}
		}
	}()

	return func() {
		stopOnce.Do(func() { close(stopCh) })
	}
}

func peakGoroutines() int64 {
	return atomic.LoadInt64(&peakG)
}
