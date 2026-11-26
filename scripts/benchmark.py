"""Run the reference HTTP workload from a separate process using the standard library."""
import argparse
import concurrent.futures
import http.client
import json
import math
import pathlib
import threading
import time
import urllib.parse
from collections import defaultdict

p = argparse.ArgumentParser()
p.add_argument("url")
p.add_argument("token_file", type=pathlib.Path)
p.add_argument("--duration", type=int, default=900)
p.add_argument("--warmup", type=int, default=120)
p.add_argument("--output", type=pathlib.Path, required=True)
args = p.parse_args()
base = urllib.parse.urlparse(args.url)
token = args.token_file.read_text().strip()
local = threading.local()
samples = defaultdict(list)
errors = defaultdict(int)
cold = {}
cache_counts = defaultdict(int)
draft = None

def request(kind, path, body=None):
    global draft
    if not hasattr(local, "conn"):
        local.conn = http.client.HTTPConnection(base.hostname, base.port, timeout=8)
    conn = local.conn
    start = time.monotonic()
    try:
        headers = {}
        if body is not None:
            headers = {"Authorization": "Bearer " + token, "Content-Type": "application/json"}
        conn.request("POST" if body is not None else "GET", path, json.dumps(body) if body is not None else None, headers)
        result = conn.getresponse()
        raw = result.read()
        if result.status != 200:
            raise ValueError(result.status)
        if kind == "write":
            draft = json.loads(raw)
        return kind, (time.monotonic() - start) * 1000, True, result.getheader("X-Cache", "BYPASS")
    except (OSError, ValueError, http.client.HTTPException):
        conn.close()
        return kind, (time.monotonic() - start) * 1000, False, "ERROR"

def route(i):
    lang = ("en", "tr")[i % 2]
    if i % 20 < 10:
        page = "" if i % 4 < 2 else "&cursor=MTcwMDAwMDAwOTk4MC9zZWVkLTA5OTgw"
        return "feed", "/?lang=" + lang + page
    if i % 20 < 16:
        return "post", f"/?lang={lang}&post=seed-{i%10000:05d}"
    if i % 20 < 18:
        return "search", f"/?lang={lang}&q=Group%20{i%127}"
    return ("rss", "/feed.xml?lang=" + lang) if i % 2 == 0 else ("markdown", "/feed.md?lang=" + lang)

def record(result):
    kind, ms, ok, cache = result
    cache_counts[cache] += 1
    samples[kind].append(ms)
    if not ok:
        errors[kind] += 1

for i in [0,10,16,18,19]:
    k, path = route(i)
    _, ms, ok, cache = request(k, path)
    cold[k] = {"ms": round(ms,2), "ok": ok}
with concurrent.futures.ThreadPoolExecutor(max_workers=40) as pool:
    for duration, measure in [(args.warmup, False), (args.duration, True)]:
        pending = []
        start = time.monotonic()
        for i in range(duration * 50):
            time.sleep(max(0, start + i / 50 - time.monotonic()))
            pending.append(pool.submit(request, *route(i)))
            if i % 250 == 0:
                body = draft or {"original":"en","text":{"en":{"body":"Benchmark draft","published":False}}}
                body["text"]["en"]["published"] = i % 500 == 0
                pending.append(pool.submit(request, "write", "/api/v1/save_post", body))
            if len(pending) >= 100:
                for future in pending:
                    result = future.result()
                    if measure:
                        record(result)
                pending.clear()
        for future in pending:
            result = future.result()
            if measure:
                record(result)
report = {"elapsed_seconds":round(time.monotonic()-start,2),"cache":dict(cache_counts),"rows":10000,"read_rps":50,"seconds":args.duration,"warmup":args.warmup,"cold":cold,"errors":dict(errors),"latency":{}}
for kind, times in samples.items():
    times.sort()
    report["latency"][kind] = {"count":len(times), **{f"p{pct}_ms":round(times[math.ceil(pct/100*len(times))-1],2) for pct in (95,99)}}
args.output.write_text(json.dumps(report,indent=2)+"\n")
print(json.dumps(report))
raise SystemExit(bool(errors) or any(v["p95_ms"] > {"search":400,"write":500}.get(k,200) for k,v in report["latency"].items()))
