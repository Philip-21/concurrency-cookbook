
---

## `docs/concepts-map.md`

```markdown
# Concepts Map: Resource Management in Go ↔ Python

The same scalability problems, solved with different primitives. Use this to translate a resource-management pattern from one language to the other.

> **The lens:** every row answers *"how do I control how much of X runs at once?"* — not just *"what's the equivalent API?"*

---

## 1. The big picture

| | Go | Python |
|---|---|---|
| Unit of concurrency | goroutine | `asyncio.Task` / `Thread` / `Process` |
| Default cost | ~2 KB, cheap | task ~KB, thread ~MB, process ~tens of MB |
| CPU parallelism | free across `GOMAXPROCS` | needs `multiprocessing` (GIL) |
| The scalability risk | **too many goroutines** | **wrong model for the bottleneck** |
| Primary control | channels + `context` | semaphores + executors |
| Backpressure | full channel blocks sender | `Queue(maxsize)` blocks producer |

**The one-sentence difference:** Go makes it easy to start concurrent work, so the hard part is *limiting* it. Python makes it hard to start concurrent work, so the hard part is *choosing the right kind*.

---

## 2. Resource-management patterns

### 2.1 Bounding concurrency

| Goal | Go | Python |
|---|---|---|
| Cap concurrent work at N | worker pool + buffered channel | `ThreadPoolExecutor(max_workers=N)` |
| Cap with cancellation | `errgroup.SetLimit(N)` | `asyncio.Semaphore(N)` |
| Cap CPU-bound work | worker pool sized to `GOMAXPROCS` | `Pool(processes=N)` |
| Cap memory via buffers | `make(chan T, N)` | `asyncio.Queue(maxsize=N)` |

**Failure mode when you don't bound:** Go → goroutine explosion + OOM. Python → thread explosion or event-loop starvation.

### 2.2 Backpressure

| Goal | Go | Python |
|---|---|---|
| Slow producer when consumer is busy | unbuffered / small-buffered channel | `Queue(maxsize=N)` + `await put()` |
| Drop when full (lossy) | `select { case ch <- v: default: }` | `queue.put_nowait()` + `QueueFull` |
| Buffer to smooth bursts | larger buffered channel | larger `maxsize` |

**Failure mode without backpressure:** producer runs ahead of consumer → memory grows until OOM. This is the #1 cause of "it worked in staging" production incidents.

### 2.3 Cancellation and timeouts

| Goal | Go | Python |
|---|---|---|
| Cancel a subtree of work | `context.WithCancel` | `TaskGroup` + `task.cancel()` |
| Timeout a call | `context.WithTimeout` | `asyncio.timeout(d)` |
| Deadline propagation | pass `ctx` as first arg | pass `deadline` explicitly |
| Stop on first error | `errgroup.WithContext` | `asyncio.gather` + cancel siblings |

**Failure mode without cancellation:** leaked goroutines/tasks pile up on every timeout → memory and FD leak.

### 2.4 Graceful shutdown

| Goal | Go | Python |
|---|---|---|
| Stop accepting new work | close the job channel | stop the producer task |
| Drain in-flight work | `wg.Wait()` after close | `await asyncio.gather(*tasks)` |
| Hard deadline | `context.WithTimeout` | `asyncio.wait(..., timeout=...)` |
| Signal handling | `signal.NotifyContext` | `loop.add_signal_handler` |

**Failure mode without graceful shutdown:** deploys drop in-flight requests; queue messages are lost.

### 2.5 Rate limiting

| Goal | Go | Python |
|---|---|---|
| N requests per second | `time.Ticker` + token bucket | `asyncio.Semaphore` + sleep |
| Burst + steady rate | `golang.org/x/time/rate` (stdlib pattern: leaky bucket) | leaky bucket via timestamps |
| Per-key limits | `sync.Map` of limiters | `dict` of semaphores |

**Failure mode without rate limiting:** you hammer a downstream API and get 429s or banned.

---

## 3. The three Python models vs Go's one (scalability view)

| Workload | Go | Python | Why |
|---|---|---|---|
| **I/O-bound, high concurrency** | goroutines (cheap) | `asyncio` | asyncio task ≈ goroutine; threads cost ~1 MB each |
| **I/O-bound, blocking libs** | goroutines | `ThreadPoolExecutor` | threads release GIL during I/O |
| **CPU-bound** | goroutines (free parallel) | `multiprocessing` | GIL serializes threads; processes don't |
| **Mixed** | `errgroup` + worker pool | `asyncio` + `run_in_executor(ProcessPool)` | isolate the CPU part in a process pool |

**Scaling math:**
- 10k concurrent HTTP calls: Go → 10k goroutines (~20 MB). Python → 10k asyncio tasks (~10–50 MB). Python threads → **10 GB, won't work.**
- 8-core CPU job: Go → `GOMAXPROCS=8`, 8× speedup. Python threads → **1× (GIL)**. Python processes → 8× speedup.

---

## 4. Anti-patterns → scalability failures

| Anti-pattern | Go symptom | Python symptom | Resource lost |
|---|---|---|---|
| Unbounded spawning | `go fn()` in a loop | `create_task` in a loop | memory, scheduler |
| Missing backpressure | sender blocks forever / OOM | `Queue()` unbounded grows | memory |
| Forgotten cancel | goroutines leak on timeout | tasks pile up on timeout | memory, FDs |
| Blocking the loop | n/a | `time.sleep` in `async def` | entire app stalls |
| Shared state without lock | race, `-race` flags it | GIL hides it — still a bug | correctness |
| Threads for CPU work | n/a | 1× speed, +overhead | CPU, wall time |
| No rate limit | downstream 429s | downstream 429s | downstream quota |

---

## 5. Decision table — "my app needs to scale, what do I do?"

| Symptom | Go fix | Python fix |
|---|---|---|
| High memory, many goroutines/tasks | worker pool + semaphore | `Semaphore` or `max_workers` |
| Slow under I/O load | check for blocking calls; use goroutines | go `asyncio`, avoid blocking libs |
| CPU pegged, threads don't help | n/a (goroutines scale) | switch to `multiprocessing` |
| Downstream 429s | `time.Ticker` rate limiter | `asyncio.Semaphore` + backoff |
| Deploys drop requests | `signal.NotifyContext` + `wg.Wait` | `add_signal_handler` + drain |
| Queue grows unbounded | smaller channel buffer / add consumers | `Queue(maxsize=...)` |
| Timeouts leak resources | thread `ctx` everywhere | `asyncio.timeout` + `TaskGroup` |

---

