# Performance verification

## v0.1.1 verification

On November 26, 2026, we repeated the HTTP and mobile workloads on SQLite, PostgreSQL 16.16, redb 5.0.0 and LMDB. The full storage benchmark covered all nine drivers. Each of the four HTTP deployments had its own database and media files. The measured Linux/amd64 application used Go 1.27.4, Rust 1.100.0 and the runtime sources at `071d5ea`. The raw report includes executable hashes, every burst and browser sample, per-route percentiles and cgroup counters. [Raw HTTP and mobile results](measurements/v0.1.1.json) · [Storage output](measurements/storage-v0.1.1.txt)

### 10,000 posts and 5,000 photographs

Each 20-post page contains ten separately addressed photographs. We rotated the four engines across three runs each, restarted the server and emptied its thumbnail directory before every run. The following values are median run-level p95 latencies in milliseconds, measured through the last response byte. All requests succeeded. The warm timeline burst serves one cached HTML page per reader. Each two-photo visit requests the page and two 480 px JPEGs in sequence.

| Scenario (p95 ms) | SQLite | PostgreSQL | redb | LMDB |
| --- | ---: | ---: | ---: | ---: |
| Cold timeline, 128 simultaneous readers | 29.70 | 28.28 | 29.63 | 29.10 |
| First 10 concurrent thumbnails | 59.43 | 71.05 | 57.27 | 60.33 |
| Mixed routes, 4,096 requests / 64 workers | 5.86 | 10.26 | 9.95 | 9.04 |
| Common-word search, 512 requests / 32 workers | 10.58 | 10.21 | 4.79 | 4.27 |
| Deep cursor, 512 requests / 32 workers | 7.71 | 7.99 | 3.71 | 3.89 |
| Warm timeline, 10,000 simultaneous readers | 322.28 | 227.32 | 524.32 | 485.94 |
| 10,000 visits with two photos each | 1,506.10 | 1,457.18 | 1,475.32 | 1,473.87 |

| Per burst, median | SQLite | PostgreSQL | redb | LMDB |
| --- | ---: | ---: | ---: | ---: |
| Two-photo visit p99 (s) | 1.53 | 1.48 | 1.49 | 1.49 |
| Wall time for all 30,000 requests (s) | 1.56 | 1.51 | 1.52 | 1.51 |
| Encoded bodies transferred (GB) | 1.13 | 1.13 | 1.13 | 1.13 |
| Server peak RSS across all seven scenarios (MiB) | 560 | 524 | 515 | 490 |

These runs used HTTP/1.1 over loopback on an Intel Core Ultra 9 285H. The server and PostgreSQL shared logical CPUs 2 and 4. The client used CPUs 6–11, matching the earlier image-heavy test. The OS filesystem cache was retained. RSS covers the Go process and its in-process database and excludes the separate PostgreSQL process. The raw file also includes three runs per engine on low-power CPUs 14 and 15. Those samples were collected during other local measurements and are reported separately. An internet-facing VPS serving 1.13 GB across a 100 Mbit/s uplink needs about 90 seconds for the bodies alone.

Compared with the previous measurements on the same CPU assignments, warm timeline p95 increased from 187 to 322 ms on SQLite and from 166 to 227 ms on PostgreSQL. Two-photo visit p95 was 1.51 s on SQLite in both sets and rose from 1.37 to 1.46 s on PostgreSQL. The warm timeline serves cached HTML. Its timing includes connection setup, socket transfer and client scheduling, so this scenario cannot isolate database latency.

### Fifteen minutes of reads and publication changes

After a two-minute warm-up, each server handled 50 reads per second and one publication change every five seconds on a 10,000-post fixture with 200 small images. Across all four engines, 180,000 measured reads and 720 writes completed with zero errors. All four met the existing p95 limits. Each cell below shows p95 / p99 in milliseconds.

| Operation | SQLite | PostgreSQL | redb | LMDB |
| --- | ---: | ---: | ---: | ---: |
| Timeline | 0.52 / 1.32 | 1.06 / 2.26 | 0.65 / 1.29 | 0.56 / 1.26 |
| Focused post | 2.14 / 2.64 | 2.59 / 3.39 | 1.70 / 2.29 | 1.82 / 2.02 |
| Search | 5.66 / 6.88 | 7.41 / 9.52 | 4.40 / 5.27 | 4.15 / 4.92 |
| RSS | 0.98 / 1.29 | 1.20 / 2.40 | 0.84 / 1.22 | 0.83 / 1.22 |
| Markdown | 0.65 / 0.92 | 1.13 / 1.94 | 0.64 / 0.90 | 0.56 / 0.89 |
| Write | 4.87 / 5.64 | 4.82 / 5.53 | 3.80 / 4.63 | 2.92 / 3.43 |

| Server or deployment | SQLite | PostgreSQL | redb | LMDB |
| --- | ---: | ---: | ---: | ---: |
| Cold search (ms) | 67.36 | 196.59 | 70.08 | 76.81 |
| Peak sampled cgroup memory (MiB) | 41.3 | 83.3 | 43.6 | 34.4 |
| OOM kills | 0 | 0 | 0 | 0 |

Each deployment ran in its own 2 GiB cgroup with two logical CPUs and a 200% CPU quota. PostgreSQL shared its application’s cgroup. Memory was sampled once per second and includes charged file cache. The four deployments ran concurrently on distinct CPU pairs with different base capabilities. Use this table to assess the workload budgets. The burst table above uses the same cores for every engine and is better suited to direct comparisons.

### Mobile reader

Each cell shows p75 for eight fresh Chromium contexts, alternating English and Turkish with the Paper theme. The emulated viewport was 390 × 844 at 2× pixel density, with 4× CPU slowdown, 150 ms latency and 1.6 Mbit/s downstream. Interactions tapped the search field and typed a query. JS figures count encoded script response bodies observed by the harness.

| Fixture and metric | SQLite | PostgreSQL | redb | LMDB |
| --- | ---: | ---: | ---: | ---: |
| Image-heavy LCP (ms) | 1728 | 1724 | 1728 | 1724 |
| Image-heavy INP (ms) | 32 | 32 | 32 | 32 |
| Image-heavy CLS | 0 | 0 | 0 | 0 |
| Text-heavy LCP (ms) | 456 | 460 | 460 | 460 |
| Text-heavy INP (ms) | 32 | 32 | 32 | 32 |
| Text-heavy CLS | 0 | 0 | 0 | 0 |
| Maximum reader JS (bytes) | 4142 | 4142 | 4142 | 4142 |

### Nine storage engines

The Go benchmark populated 500 records and ran 100 iterations per operation, repeated three times. Values are the median of the three mean times, in microseconds per operation. Five file-backed stores ran in temporary directories. PostgreSQL 18, MySQL 8.4, MariaDB 11.8 and MongoDB 8 ran as separate services on the same GitHub-hosted Linux runner. The workflow also exercised the storage and publishing contracts. [GitHub Actions run](https://github.com/sametcn99/th0ughts/actions/runs/45851673928)

| Driver | Read 20 rows (µs) | Update one row (µs) |
| --- | ---: | ---: |
| SQLite | 29.25 | 111.30 |
| PostgreSQL | 89.00 | 175.45 |
| redb | 14.11 | 129.97 |
| LMDB | 17.28 | 160.72 |
| MySQL | 200.96 | 142.44 |
| MariaDB | 198.65 | 140.46 |
| MongoDB | 207.85 | 135.14 |
| Single JSON | 0.29 | 766.72 |
| Directory JSON | 0.38 | 489.45 |

The JSON engines serve repeated page reads from ordered in-memory snapshots. Single-file JSON rewrites its database on each mutation. Different filesystems, durability settings and network distances change the cost of a write. The reproduction commands below cover the image-heavy, sustained, mobile and storage workloads.

## Earlier image-heavy feed and concurrent readers

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
export GOFLAGS=-tags=nativekv
go run ./scripts/benchseed -data "$work/data" -driver redb -posts 10000 \
  -image scripts/fixtures/great-wave.jpg -image-every 2
GOMAXPROCS=2 bin/th0ughts -data "$work/data" -listen 127.0.0.1:18081 serve
```

Run the client in a separate shell. The client connects only to IPv4 loopback. Each burst can open thousands of sockets. The measured machine had a file descriptor limit of 524,288. Set `-burst` to a smaller value to exercise a smaller machine.

```sh
go run ./scripts/scalebench -port 18081 -burst 10000 > scale.json
bun scripts/mobile-vitals.mjs http://127.0.0.1:18081 mobile.json
```

For cross-version comparisons, use the same fixture and client binary, stop the disposable server between runs and move its generated `thumbnails` directory aside before restarting. The raw report records the CPU affinity and hashes for the measured binaries. The mobile script creates eight fresh Chromium contexts per build. Use a new data directory for each driver and select it with the seed command’s `-driver` flag. For the PostgreSQL scenario, also pass `-driver postgres -address <disposable-postgres-URI>` to the seed command and give the server and database the same CPU affinity.

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
export GOFLAGS=-tags=nativekv
go run ./scripts/benchseed -data "$work/data" -driver lmdb
bin/th0ughts -data "$work/data" -listen 127.0.0.1:8080 serve
```

Run the client in a second shell, using the temporary directory created above.

```sh
python3 scripts/benchmark.py http://127.0.0.1:8080 "$work/data/bench-token" --output lmdb.json
bun scripts/mobile-vitals.mjs http://127.0.0.1:8080 mobile.json
```

For PostgreSQL, use a fresh directory and pass `-driver postgres -address <disposable-postgres-URI>` to the seed command. The server reopens the encrypted locator in that directory. On Linux with cgroup v2 and systemd, set `CPUQuota=200%` and `MemoryMax=2G` on a dedicated slice. Start the server and its database in that slice, and start the workload client outside the slice. Read `cpu.max`, `memory.max`, `memory.current`, and `memory.events` from the slice during the run.

To reproduce the storage measurements, set the external database addresses with `TEST_postgres`, `TEST_mysql`, `TEST_mariadb`, and `TEST_mongodb` in the test environment, build the native archive with `bash scripts/build-native.sh`, then run `go test -tags nativekv ./storage -run '^$' -bench BenchmarkStore -benchtime=100x -count=3`. The five file-backed engines use temporary directories. The GitHub Actions workflow `storage-performance.yml` provisions all four external database services and saves the output of the same benchmark.
