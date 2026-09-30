#!/usr/bin/env python3
"""Count traces per request when spans can't be found by request ID.

usage: count-by-window.py MODE N [GAP]

Sends N requests (one fan-out MODE) with GAP seconds (default 2) of quiet
between them, then counts the distinct traces Tempo holds for the four
services in each request's time window. Used for the Go-uprobe tracer
configuration, where OBI only put the ID on the edge span.
"""
import json, os, sys, time, urllib.error, urllib.parse, urllib.request

TEMPO = os.environ.get("TEMPO_URL", "http://localhost:3200")
GW = os.environ.get("GATEWAY", "http://localhost:8080")
mode, n = sys.argv[1], int(sys.argv[2])
gap = float(sys.argv[3]) if len(sys.argv) > 3 else 2.0

windows = []
for i in range(n):
    t0 = time.time_ns()
    try:
        urllib.request.urlopen(f"{GW}/checkout?mode={mode}&fail=0", timeout=10).read()
    except (urllib.error.HTTPError, TimeoutError, urllib.error.URLError):
        pass  # payments' random failures still produce spans
    windows.append((t0, time.time_ns()))
    time.sleep(gap)
time.sleep(60)  # let Tempo make spans searchable

hist = {}
for t0, t1 in windows:
    q = urllib.parse.urlencode({
        "q": '{ resource.service.name =~ "gateway|orders|payments|inventory" }',
        "start": t0 // 10**9 - 1, "end": t1 // 10**9 + 2, "limit": 200, "spss": 100})
    traces = json.load(urllib.request.urlopen(f"{TEMPO}/api/search?{q}", timeout=30)).get("traces", [])
    # keep traces whose spans start inside this request's window
    ids = {t["traceID"] for t in traces
           if t0 - 5 * 10**7 <= int(t["startTimeUnixNano"]) <= t1 + 5 * 10**7}
    hist[len(ids)] = hist.get(len(ids), 0) + 1
print(f"{mode}: traces per request: {dict(sorted(hist.items()))}")
