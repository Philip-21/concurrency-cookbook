# concurrency-cookbook

**Effective concurrency in Go and Python** — standalone, runnable case studies showing how to manage resources and make applications scale.

Every case study is a single file you can run with one command. No shared packages, no build setup, no dependencies. Each one is a **real scalability problem** — a bottleneck you'd hit in production — solved with the right concurrency model.

> **The thesis:** concurrency isn't a language feature you sprinkle on top. It's how you decide *what work runs when, on what resource, under what limit*. Get it right and your app handles 10× the load on the same hardware. Get it wrong and you get race conditions, OOMs, and thundering herds.

---

## Why different case studies per language?

Because scalability problems look different in each runtime:

- **Go** gives you goroutines everywhere — so the challenge is **bounding** them, not spawning them. Go cases focus on pools, limits, cancellation, and backpressure.
- **Python** forces a choice between `asyncio`, threads, and processes — so the challenge is **picking the right model** for the bottleneck (I/O vs CPU). Python cases cover all three.

The comparison lives in [`docs/concepts-map.md`](docs/concepts-map.md): the same resource-management pattern expressed in both languages.

---

## The scalability lens

Every case study answers the same four questions:

| Question | What it means |
|---|---|
| **What's the bottleneck?** | I/O wait? CPU? memory? a downstream service? |
| **What's the resource being managed?** | connections, workers, memory, rate budget, CPU cores |
| **What happens under load?** | does it degrade gracefully or fall over? |
| **What's the cost of getting it wrong?** | race, deadlock, OOM, thundering herd, leaked goroutines/tasks |

Each `main.go` / `main.py` header states the bottleneck, the resource, and the failure mode. The code shows the fix.

---


## Go case studies — bounding and resource management

| # | Case | Scalability problem | Resource managed |
|---|------|---------------------|------------------|
| 01 | [Bounded worker pool](go/01-bounded-worker-pool) | Unbounded goroutines OOM under load | Worker count |
| 02 | [Ticketing system](go/02-ticketing-system) | Race condition oversells inventory | Shared counter |
| 03 | [Rate limiter](go/03-rate-limiter) | Downstream API gets hammered | Request budget |
| 04 | [Backpressure pipeline](go/04-backpressure-pipeline) | Fast producer overwhelms slow consumer | Buffer size |
| 05 | [Graceful shutdown](go/05-graceful-shutdown) | In-flight work lost on deploy | Goroutine lifecycle |
| 06 | [Fan-out with limits](go/06-fanout-with-limits) | Crawler saturates sockets/FDs | Concurrent connections |

## Python case studies — picking the right model

| # | Case | Scalability problem | Model |
|---|------|---------------------|-------|
| 01 | [Async API aggregator](python/01-async-api-aggregator) | 1000 I/O calls, one thread | `asyncio` |
| 02 | [Download manager](python/02-download-manager) | Blocking libs, need concurrency | `ThreadPoolExecutor` |
| 03 | [Image batch](python/03-image-batch) | CPU-bound work, GIL blocks threads | `multiprocessing` |
| 04 | [Producer-consumer](python/04-producer-consumer) | Unbounded queue eats memory | `asyncio.Queue(maxsize)` |
| 05 | [Rate-limited client](python/05-rate-limited-client) | API throttling / 429s | `asyncio.Semaphore` |
| 06 | [Mixed workload](python/06-mixed-workload) | I/O + CPU in one pipeline | `asyncio` + `run_in_executor` |


## Quick start

```bash
# Go
cd go/01-bounded-worker-pool
go run . -race

# Python
cd python/01-async-api-aggregator
python main.py



MIT License

Copyright (c) 2026 Philip Obiora

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.