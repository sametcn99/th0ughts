# Performance verification

The HTTP benchmark exercises a running server seeded with 10,000 published posts. The original language alternates between English and Turkish. Posts span 31 tags and 127 search groups, and 200 posts have a real image attachment. A separate Python process sends requests over loopback. Each deployment gets its own cgroup with a CPU quota of two cores and a 2 GiB memory limit. The PostgreSQL process shares the application's resource budget.

## Workload

The client probes each route after server startup, warms up for two minutes, then issues 50 reads per second for 15 minutes. Half of the reads request a timeline page, including cursor pagination. Focused posts receive 30%, search receives 10%, and RSS and Markdown each receive 5%. A scoped token edits a post every five seconds, alternating publication and withdrawal. These writes invalidate the public cache. The driver records wall-clock HTTP latency through reading the full response body.

The acceptance limits are zero request errors and p95 below 200 ms for feed, focused post, RSS and Markdown, 400 ms for search, and 500 ms for writes. Raw results include one cold probe per operation. The first search builds the in-memory search index. Cold probes refer to the process state. The operating system's filesystem caches were retained.

## Results

On November 26, 2026 both deployments completed **45,000 reads and 180 writes each**, with **zero HTTP errors**. The server was built with Go 1.27.4 for Linux/amd64. The measured executable had SHA-256 `0368c1071edfae78ae8d6224b11e66fa6acc550359a0a16ee1ed5fa6cc81de1c`. The PostgreSQL deployment used version 18.7. Both workloads ran concurrently with separate CPU and memory budgets on the same host. [SQLite report](measurements/sqlite.json) · [PostgreSQL report](measurements/postgresql.json)

| Operation | SQLite p95 | SQLite p99 | PostgreSQL p95 | PostgreSQL p99 |
| --- | ---: | ---: | ---: | ---: |
| Timeline | 0.95 ms | 1.91 ms | 1.18 ms | 3.56 ms |
| Focused post | 2.14 ms | 2.98 ms | 3.43 ms | 3.94 ms |
| Search | 10.86 ms | 14.34 ms | 15.46 ms | 16.96 ms |
| RSS | 1.21 ms | 2.06 ms | 1.46 ms | 3.01 ms |
| Markdown | 0.92 ms | 1.28 ms | 1.54 ms | 2.61 ms |
| Write | 4.85 ms | 6.06 ms | 4.55 ms | 4.99 ms |

SQLite served 57.6% of the measured reads from the public response cache. Its first search took 164.67 ms. The highest memory sample in its slice was 50.6 MiB. PostgreSQL served 57.6% of the measured reads from the public response cache. Its first search took 180.27 ms. The highest memory sample in its slice was 196.8 MiB. The sampler read the cgroups once per second throughout warm-up and measurement. Neither slice recorded an OOM kill. [Resource samples](measurements/resources.json)

## Mobile reader

Eight fresh Chromium contexts alternated English and Turkish on the seeded timeline. Each context used a 390 × 844 viewport, 4× CPU slowdown, 150 ms simulated latency, 1.6 Mbps downstream and 750 Kbps upstream. The input sequence tapped the search field and typed a query. The p75 results were **416 ms LCP**, **32 ms INP**, and **0 CLS**. The page transferred **719 compressed JavaScript bytes** for the reader. These laboratory measurements cover the included Paper theme and this interaction. Real visitors' networks, devices, images and custom themes can produce different results. [Mobile measurements](measurements/mobile.json)

## Storage driver measurements

The Go storage benchmark seeds 500 records and runs 100 iterations of both a 20-record page and a durable update for each driver. It uses native database processes on the development host. Times below are means in microseconds per operation. The HTTP load test above covers SQLite and PostgreSQL. [Go benchmark output](measurements/storage.txt)

| Driver | Read 20 records | Update one record |
| --- | ---: | ---: |
| SQLite | 38.7 | 18.0 |
| PostgreSQL | 140.8 | 165.4 |
| MySQL | 487.9 | 94.0 |
| MariaDB | 159.2 | 194.7 |
| MongoDB | 251.1 | 95.6 |
| Single JSON | 208.9 | 504.1 |
| Directory JSON | 119.9 | 15.5 |

Single-file JSON rewrites its file for each mutation. Directory JSON updates one record at a time. Both keep records in memory and sort matching rows for a page. SQL and MongoDB have persistent ordering indexes.

## Reproduce

Build the frontend and Go executable as described in the README. Use a dedicated empty directory and a disposable database. The seed command writes a two-hour publishing token with owner-only file permissions.

```sh
umask 077
export TH0UGHTS_MASTER_KEY="$(openssl rand -hex 32)"
work="$(mktemp -d)"
go run ./scripts/benchseed -data "$work/data" -driver sqlite
bin/th0ughts -data "$work/data" -listen 127.0.0.1:8080 serve
```

Run the client in a second shell, using the temporary directory created above.

```sh
python3 scripts/benchmark.py http://127.0.0.1:8080 "$work/data/bench-token" --output sqlite.json
bun scripts/mobile-vitals.mjs http://127.0.0.1:8080 mobile.json
```

For PostgreSQL, use a fresh directory and pass `-driver postgres -address <disposable-postgres-URI>` to the seed command. The server reopens the encrypted locator in that directory. On Linux with cgroup v2 and systemd, set `CPUQuota=200%` and `MemoryMax=2G` on a dedicated slice. Start the server and its database in that slice, and start the workload client outside the slice. Read `cpu.max`, `memory.max`, `memory.current`, and `memory.events` from the slice during the run.

To reproduce the storage measurements, set the external database addresses with `TEST_postgres`, `TEST_mysql`, `TEST_mariadb`, and `TEST_mongodb` in the test environment, then run `go test ./storage -run '^$' -bench BenchmarkStore -benchtime=100x`. SQLite and both JSON drivers use temporary directories.
