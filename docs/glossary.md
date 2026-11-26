# Glossary

Short definitions of the concurrency and resource-management terms used across this repo. No code — just what the term means and why it matters for scaling.

---

## Go

| Term | What it is | Why it matters for scale |
|---|---|---|
| **Goroutine** | Lightweight concurrent function run by the Go runtime (~2 KB stack). | Cheap enough to spawn thousands — so the risk is *too many*, not too few. |
| **Channel** | Typed pipe for passing values between goroutines. | The primary way to share work without locks. Buffered channels act as bounded queues. |
| **Buffered channel** | Channel with a fixed-size buffer (`make(chan T, N)`). | When full, the sender blocks — this is **backpressure**. Buffer size = burst you can absorb. |
| **Unbuffered channel** | Channel with no buffer (`make(chan T)`). | Every send waits for a receiver — tightest coupling, lowest throughput. |
| **`select`** | Waits on multiple channel operations, runs the first that's ready. | How you multiplex, add timeouts, and implement non-blocking sends (`default`). |
| **`sync.Mutex`** | Mutual exclusion lock. | Guards shared state. Needed whenever two goroutines write the same variable. |
| **`sync.RWMutex`** | Reader/writer lock. | Many readers, one writer — better throughput for read-heavy shared state. |
| **`sync.WaitGroup`** | Counter to wait for a set of goroutines to finish. | The standard "wait for fan-out to complete" tool. `Add` before `go`, `Done` in the goroutine, `Wait` in the parent. |
| **`sync.Pool`** | Reusable object pool to reduce allocations. | Cuts GC pressure under high throughput — a *performance* tool, not a *concurrency-limit* tool. |
| **`sync/atomic`** | Lock-free operations on integers/pointers. | Cheapest way to increment a counter safely. No mutex needed for simple counters. |
| **Worker pool** | Fixed set of goroutines reading from a shared job channel. | Bounds concurrency to a known number. Scales to millions of jobs at constant memory. |
| **Semaphore (channel-based)** | Buffered channel of `struct{}` used as a counter. | Bounds concurrent operations (sockets, FDs) without spawning a fixed pool. |
| **`context.Context`** | Carries cancellation, deadlines, and request-scoped values. | How you reclaim resources on timeout and propagate cancellation to child goroutines. |
| **`errgroup`** | `golang.org/x/sync` helper: `WaitGroup` + error collection + `ctx` cancellation + `SetLimit`. | The one-liner for "bounded, cancellable, error-aware fan-out." |
| **`time.Ticker`** | Fires on a fixed interval. | Simple rate limiter: one tick = one request slot. |
| **Token bucket** | Rate limiter: tokens refill at a steady rate, requests spend tokens, bursts allowed up to bucket size. | Protects downstream APIs and your own quota. |
| **Graceful shutdown** | Stop accepting work, drain in-flight, then exit. | Deploys don't drop requests; queue messages aren't lost. |
| **`signal.NotifyContext`** | Turns OS signals (SIGTERM) into a cancelled `context`. | The idiomatic Go entry point for graceful shutdown. |
| **Goroutine leak** | A goroutine blocked forever on a channel/operation nobody will satisfy. | Each leak holds memory and possibly an FD. Over a day, thousands → OOM. |
| **`-race`** | Go's race detector (`go run -race .`, `go test -race`). | Catches data races at runtime. Non-negotiable for concurrent code. |

---

## Python

| Term | What it is | Why it matters for scale |
|---|---|---|
| **GIL (Global Interpreter Lock)** | CPython lock allowing only one thread to run Python bytecode at a time. | Threads don't scale CPU work. They *do* scale I/O, because the GIL is released during I/O. |
| **`asyncio`** | Single-threaded event loop with cooperative coroutines. | Scales I/O to 100k+ concurrent tasks on one thread. One blocking call freezes everything. |
| **Coroutine (`async def`)** | Function that can pause at `await` and resume later. | The unit of concurrency in `asyncio`. Must be awaited or scheduled as a `Task`. |
| **`asyncio.Task`** | A scheduled coroutine running on the event loop. | ~KB each. This is Python's closest equivalent to a goroutine. |
| **`await`** | Yield point — lets the event loop run other tasks. | The only place concurrency happens in `asyncio`. No `await` = no concurrency. |
| **Event loop** | The scheduler that runs `asyncio` tasks. | One per thread. Blocking it stalls *every* task. |
| **`asyncio.gather`** | Run many awaitables concurrently, collect results. | The "fan-out and wait" tool. Unbounded by default — pair with a semaphore. |
| **`asyncio.Queue`** | Async FIFO queue, optionally bounded (`maxsize=N`). | Bounded queue = backpressure. Unbounded = memory grows under burst. |
| **`asyncio.Semaphore`** | Async counter limiting concurrent holders. | The standard way to bound `asyncio` fan-out. |
| **`asyncio.timeout`** | Context manager that cancels the block after N seconds. | Reclaims resources on slow downstreams. Without it, tasks pile up. |
| **`TaskGroup`** (3.11+) | Structured concurrency: run tasks as a group, cancel all on first error. | The modern replacement for ad-hoc `gather` + manual cancel. |
| **`threading.Thread`** | OS thread. | ~1 MB stack. Good for blocking I/O, useless for CPU (GIL). |
| **`threading.Lock`** | Mutual exclusion lock for threads. | The GIL is *not* a safety net — `x += 1` still races. |
| **`queue.Queue`** | Thread-safe FIFO, optionally bounded. | Backpressure for thread-based code. |
| **`ThreadPoolExecutor`** | Pool of worker threads (`max_workers=N`). | Bounds thread count. Best for I/O with blocking libraries. |
| **`ProcessPoolExecutor`** | Pool of worker processes. | Real CPU parallelism — bypasses the GIL. Pickling tax on arguments/results. |
| **`multiprocessing.Pool`** | Older process-pool API. | Same idea as `ProcessPoolExecutor`. Chunk work coarsely to amortize pickling. |
| **`concurrent.futures`** | Umbrella module for `ThreadPoolExecutor` / `ProcessPoolExecutor` and `Future`. | Uniform API for both models. `as_completed` gives completion-order results. |
| **`Future`** | Handle to a result that will exist later. | How you submit work and collect it without blocking on submission. |
| **`as_completed`** | Iterate futures in completion order. | Process results as they finish instead of waiting for the slowest. |
| **`run_in_executor`** | Run a blocking function in a thread/process pool from async code. | The bridge for mixed workloads: async I/O + CPU/blocking in a pool. |
| **Pickling** | Serializing Python objects to bytes for IPC. | The cost of `multiprocessing`. Big objects = big overhead; chunk coarsely. |
| **Blocking the loop** | Calling sync code (`time.sleep`, `requests.get`) inside `async def`. | Freezes *all* tasks. The #1 `asyncio` bug. |
| **Backpressure** | Making a fast producer wait for a slow consumer. | Prevents unbounded memory growth. In Python: `Queue(maxsize=N)`. |

---

## Shared concepts (both languages)

| Term | Meaning | Why it matters for scale |
|---|---|---|
| **Concurrency** | Making progress on multiple tasks by interleaving. | Handles more work without more cores. |
| **Parallelism** | Running multiple tasks *simultaneously* on multiple cores. | Handles more CPU work. Requires multiple threads/processes (Go: free; Python: processes). |
| **Bounded concurrency** | A hard cap on how many operations run at once. | The single most important scaling decision. Unbounded = OOM or downstream ban. |
| **Backpressure** | Slow producer when consumer is busy. | Bounds memory. Turns overload into latency instead of a crash. |
| **Fan-out / fan-in** | Spawn many tasks, then collect their results. | The shape of most concurrent code. Must be bounded. |
| **Cancellation** | Telling in-flight work to stop. | Reclaims resources on timeout/shutdown. |
| **Timeout** | Cancellation after a deadline. | Prevents slow downstreams from leaking resources. |
| **Graceful shutdown** | Drain before exiting. | Deploys don't lose work. |
| **Rate limiting** | Cap requests per unit time. | Protects downstreams and your quota. |
| **Thundering herd** | Many tasks wake at once for the same resource. | Causes latency spikes and 429s. Fix with jitter + rate limits. |
| **Resource exhaustion** | Running out of FDs, memory, connections, or CPU. | The thing concurrency bugs actually *do*. Bound it or it finds you. |

---

## See also

- [Concepts map](concepts-map.md)