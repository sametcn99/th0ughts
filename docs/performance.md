# Performance verification

## Image-heavy feed and concurrent readers

We compared v0.1.0 (commit `c0cd7ba`) with the optimized server on Linux/amd64 and Go 1.27.4. The SQLite fixture has 10,000 published posts alternating between English and Turkish and 5,000 independently stored, independently addressed copies of the same 600 × 403 JPEG (166,452 bytes each). Each timeline page contains 20 posts and ten photographs. The server ran on two logical CPUs with `GOMAXPROCS=2`. The HTTP/1.1 client ran on six separate logical CPUs on the same Intel Core Ultra 9 285H. Each build ran three times, alternating versions. Each run started a fresh process with an empty thumbnail directory and retained the OS filesystem cache. [Raw runs, mobile samples and executable hashes](measurements/image-feed.json)

| Scenario | Work per run | Before p95 | After p95 | After p99 |
| --- | --- | ---: | ---: | ---: |
| Cold timeline | 128 readers released together | 119.90 ms | 27.25 ms | 27.37 ms |
| First 480 px thumbnails | 10 concurrent transformations | 106.61 ms | 56.82 ms | 56.82 ms |
| Mixed reading | 4,096 requests across 64 workers | 9.02 ms | 5.96 ms | 18.36 ms |
| Uncached public search | 512 API calls across 32 workers, common word | 88.31 ms | 10.71 ms | 12.73 ms |
| Deep timeline pagination | 512 API calls across 32 workers, cursor near the oldest 1% | 63.43 ms | 7.93 ms | 11.76 ms |
| Warm timeline | 10,000 readers released together | 243.54 ms | 187.14 ms | 192.47 ms |
| Timeline and first two photos | 10,000 visits, 30,000 HTTP requests | 2.52 s | 1.51 s | 1.51 s |

Each table cell is the median of three run-level percentiles. All six runs completed with zero request errors. Mixed reading includes permalinks, search results, Markdown, RSS, thumbnails and full originals. Each simulated visit reads the complete homepage and two 480 px photographs in sequence with empty client caches. The client permits connection reuse, advertises gzip and counts encoded body bytes. A common start signal models a flash crowd. Latency is measured from each worker starting its request or visit until its last byte. The median wall time to finish **all** 10,000 two-photo visits fell from 2.57 to 1.53 seconds.

The two-photo burst transferred **6.97 GB before and 1.12 GB after**, excluding HTTP headers. Full-workload server CPU fell from 6.31 to 2.74 CPU-seconds. Median process peak RSS fell from 519 to 495 MiB. RSS excludes kernel socket buffers, the client and filesystem cache. All measurements use local disk and loopback. At 100 Mbit/s, sending the optimized burst's bodies alone takes at least **90 seconds** across the server's uplink. TLS, reverse proxies, storage latency, other tenants, video and many readers visiting different cold pages can all change capacity.

### What changed

- Simultaneous public requests for the same page and publication version share one render. The 128-reader cold burst consistently went from 128 renders to one. The bounded cache retains identity and gzip representations, negotiates each client's encoding and validates each representation separately. Authorized pages bypass the shared cache.
- The timeline sends the translations its public browser tools use. Owner and comment interfaces retain their catalogs. The browser prioritizes the first photo in the first two posts, receives accurate responsive widths and lazy-loads later images.
- A 480 px JPEG derivative now takes **54,651 bytes**, down from a 333,417-byte PNG. Alpha-bearing input keeps PNG output. The server can return a cached derivative without opening the original object, releases image workers before streaming, and supports conditional reads. Public post attachment checks are bounded and invalidated on publication changes. Comment media goes through its moderation and visibility checks on every request. For this fixture the originals occupy 832 MB. Generating a 480 px preview for every photo would add about 273 MB on disk. The benchmark requests only the previews reached by the workload.
- Search keeps entries in feed order and starts after the supplied cursor. Concurrent readers share the index and stop when they have found a page plus one match. Rare or absent terms can still scan all indexed text. The first search builds the index from published posts and its memory use scales with the searchable text.
- SQLite and PostgreSQL now seek directly to the compound rank and ID cursor. Both JSON drivers reuse ordered per-kind snapshots across reads and invalidate the affected snapshot on writes. In the existing 500-record Go microbenchmark, median 20-record reads fell from 87.8 to 0.99 µs for single JSON and from 127.3 to 0.49 µs for directory JSON (three runs, 500 iterations each). Single-file JSON still serializes and durably replaces the whole database on every mutation.

### Mobile and sustained load

On a 390 × 844 Chromium viewport at 2× device scale, 4× CPU slowdown, 150 ms latency and 1.6 Mbit/s download, p75 Largest Contentful Paint fell from **3.89 s to 1.68 s**. Both builds measured 32 ms p75 interaction latency and zero layout shift. Each build used eight fresh contexts alternating English and Turkish, with the browser choosing the image width.

A separate two-minute run issued 50 reads per second while a scoped writer alternated publication and withdrawal every five seconds. It completed 6,000 measured reads and 24 writes with zero errors. Timeline p95 was 1.01 ms and write p95 was 4.43 ms. The raw report includes per-route percentiles and cold probes.

The SQLite burst and mobile results cover the included Paper theme, a shared hot timeline, identical image dimensions, and local-disk SQLite. This run did not measure concurrent comment submissions, S3, cache-cold reads spread over all 5,000 objects, or a production VPS uplink.

### PostgreSQL comparison

We repeated all seven HTTP scenarios three times on each build using a separate PostgreSQL 16.16 cluster and the same seed recipe. The Go process and database shared the same two CPUs, communicating over a local Unix socket. All requests succeeded in all six runs. The optimized PostgreSQL binary also uses the tuple cursor for deep pagination. The raw file includes its digest and all per-run figures. [PostgreSQL and SQLite samples](measurements/image-feed.json)

| Scenario | Before p95 | After p95 |
| --- | ---: | ---: |
| Cold timeline, 128 readers | 149.99 ms | 32.27 ms |
| Mixed reading, 64 workers | 12.45 ms | 10.76 ms |
| Uncached common-word search | 82.77 ms | 12.12 ms |
| Deep timeline pagination | 34.81 ms | 7.38 ms |
| Warm timeline, 10,000 readers | 261.45 ms | 166.49 ms |
| 10,000 visits with two photos each | 2.13 s | 1.37 s |

### Reproduce the image-heavy workload

Build the server as described in the README. Start from a dedicated empty directory with at least 1 GiB available. Run the seed once, then start the server with the same master key.

```sh
umask 077
export TH0UGHTS_MASTER_KEY="$(openssl rand -hex 32)"
work="$(mktemp -d)"
go run ./scripts/benchseed -data "$work/data" -posts 10000 \
  -image scripts/fixtures/great-wave.jpg -image-every 2
GOMAXPROCS=2 bin/th0ughts -data "$work/data" -listen 127.0.0.1:18081 serve
```

Run the client in a separate shell. The client connects only to IPv4 loopback. Each burst can open thousands of sockets. The measured machine had a file descriptor limit of 524,288. Set `-burst` to a smaller value to exercise a smaller machine.

```sh
go run ./scripts/scalebench -port 18081 -burst 10000 > scale.json
bun scripts/mobile-vitals.mjs http://127.0.0.1:18081 mobile.json
```

For cross-version comparisons, use the same fixture and client binary, stop the disposable server between runs and move its generated `thumbnails` directory aside before restarting. The raw report records the CPU affinity and hashes for the measured binaries. The mobile script creates eight fresh Chromium contexts per build. For the PostgreSQL scenario, also pass `-driver postgres -address <disposable-postgres-URI>` to the seed command and give the server and database the same CPU affinity.

## Earlier sustained SQLite and PostgreSQL comparison


The HTTP benchmark exercises a running server seeded with 10,000 published posts. The original language alternates between English and Turkish. Posts span 31 tags and 127 search groups, and 200 posts have a real image attachment. A separate Python process sends requests over loopback. Each deployment gets its own cgroup with a CPU quota of two cores and a 2 GiB memory limit. The PostgreSQL process shares the application's resource budget.

### Workload

The client probes each route after server startup, warms up for two minutes, then issues 50 reads per second for 15 minutes. Half of the reads request a timeline page, including cursor pagination. Focused posts receive 30%, search receives 10%, and RSS and Markdown each receive 5%. A scoped token edits a post every five seconds, alternating publication and withdrawal. These writes invalidate the public cache. The driver records wall-clock HTTP latency through reading the full response body.

The acceptance limits are zero request errors and p95 below 200 ms for feed, focused post, RSS and Markdown, 400 ms for search, and 500 ms for writes. Raw results include one cold probe per operation. The first search builds the in-memory search index. Cold probes refer to the process state. The operating system's filesystem caches were retained.

### Results

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

### Mobile reader

Eight fresh Chromium contexts alternated English and Turkish on the seeded timeline. Each context used a 390 × 844 viewport, 4× CPU slowdown, 150 ms simulated latency, 1.6 Mbps downstream and 750 Kbps upstream. The input sequence tapped the search field and typed a query. The p75 results were **416 ms LCP**, **32 ms INP**, and **0 CLS**. The page transferred **719 compressed JavaScript bytes** for the reader. These laboratory measurements cover the included Paper theme and this interaction. Real visitors' networks, devices, images and custom themes can produce different results. [Mobile measurements](measurements/mobile.json)

## Earlier storage driver measurements

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

Single-file JSON rewrites its file for each mutation. Directory JSON updates one record at a time. The numbers in this table predate ordered snapshots in the JSON drivers. SQL and MongoDB have persistent ordering indexes.

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
