# What eBPF does with one request, measured

**The claim:** eBPF may split one request into several traces, but the request ID still links every one of them back to the request.

Everything below was measured on this setup, not assumed:

| | |
|---|---|
| Agent | OpenTelemetry eBPF Instrumentation (OBI) `v0.13.0` |
| Kernel | `6.12.76-linuxkit`, aarch64 (Docker Desktop, engine 29.4.1, Apple Silicon Mac) |
| Services | Go 1.26, `net/http` + [baton](https://github.com/agarwalvivek29/baton), no tracing code |
| Date | 2026-09-30 |

The request goes `gateway → orders → {payments, inventory}`. That's **7 HTTP hops**: 4 server spans and 3 client spans. `orders` makes its two downstream calls in one of three ways (`FANOUT_MODE` or `?mode=`):

- `sync`: one call after the other, on the request goroutine.
- `go`: one `go func()` per call, joined with a `WaitGroup`.
- `pool`: jobs sent over a channel to a worker pool started at boot. The job carries the request's `ctx`.

## Result 1: with OBI's Go tracers, a worker pool splits the trace

These are OBI's Go-specific uprobes (`discovery.skip_go_specific_tracers: false`), with `context_propagation: headers`. They follow goroutine creation (`runtime.newproc1`) and inject `traceparent` from inside the Go runtime.

| Mode | Traces per request (10 requests each) |
|---|---|
| `sync` | **1** (10/10) |
| `go` | **1** (10/10). OBI follows a plain `go func()`. |
| `pool` | **3** (10/10): gateway→orders, orders→payments, orders→inventory |

OBI links a goroutine to a request by walking the goroutine's creator chain. A pool worker was created at boot, not by the request, so the link is lost. (The channel send shows up as a span link, not as a parent.) This matches OBI's source: `bpf/gotracer/go_runtime.c`, `go_common.h` (`find_parent_goroutine`, up to 6 levels).

**The catch:** in this mode OBI put the `X-Request-ID` attribute **only on the edge span**, the gateway server span. The coverage run gave 0 of 20 requests with the ID on every hop. So the ID couldn't reach the other two traces, and that's why the demo doesn't use this mode.

## Result 2: with OBI's generic tracer, the ID is on every hop

This is the demo's default (`deploy/obi.yml`): the generic kernel HTTP tracer (`skip_go_specific_tracers: true`) with header capture enabled.

| Mode | Requests with the ID on **all 7 hops** | Traces per request (20 requests each) |
|---|---|---|
| `sync` | **20/20** | 1 ×11, 2 ×2, 3 ×6, 4 ×1 |
| `go` | **20/20** | 1 ×16, 3 ×4 |
| `pool` | **20/20** | 1 ×9, 2 ×2, 3 ×9 |
| `pool`, ID minted by baton at the edge (no client header) | **20/20** | 1 ×11, 2 ×8, 3 ×1 |

Here even `sync` gets split. The cause on this kernel isn't goroutines. OBI's generic tracer injects `traceparent` through an `sk_msg` hook. Kernels with the FIONREAD sockhash bug (6.6.128+, 6.12.75+, 6.18.14+, 6.19+; this is 6.12.76) can't use that hook safely, so OBI turns it off and logs an ERROR (`pkg/internal/ebpf/tpinjector/fionread_check.go`). Without it, OBI links hops heuristically.

**The request ID, on the other hand, never missed:** 80 of 80 requests had it on every hop.

## Result 3: one click finds all of them

A failing `pool` request whose payments call failed, `pool-g1-8`, was split into 3 traces. In Grafana:

1. Loki: `{service=~".+"} |= "pool-g1-8"` returns six lines from four services. The payments `ERROR` line is among them.
2. Open the line, then **Links → "Find every trace for this request"**.
3. Tempo runs the TraceQL search `{ span."http.request.header.x-request-id" = "pool-g1-8" || span."http.response.header.x-request-id" = "pool-g1-8" }` and returns **3 traces**: `orders POST /charge`, plus two `gateway GET /checkout` fragments.

eBPF's view is 3 traces; the request ID's view is 1 request.

## Sharp edge: dropped context

`?drop_ctx=1` makes `orders` call payments with `context.Background()`. For request `drop-1`:

- orders logs `WARN outbound call has no request ID url=http://payments:8082/charge…`. That comes from baton's `WithOnMissing` hook.
- Tempo: the search by `drop-1` finds every hop except `payments SERVER`.
- Loki: payments' error line is under a **different** request ID, the one baton minted there, so the join is visibly broken at that hop.

## Reproduce

```bash
cd deploy && docker compose up -d --build
cd .. && N=20 RUN=r1 ./scripts/fanout.sh     # IDs sync-r1-1..20, go-r1-*, pool-r1-*
sleep 60                                     # Tempo needs ~1 min before spans are searchable
for m in sync go pool; do python3 scripts/count-traces.py $m-r1 20; done

# Result 1 (Go tracers): restart OBI with OTEL_EBPF_SKIP_GO_SPECIFIC_TRACERS=false, then
for m in sync go pool; do python3 scripts/count-by-window.py $m 10; done
```

## Open questions

- **A kernel without the FIONREAD bug** (or a future OBI fix) should keep `traceparent` injection on for the generic tracer. That would give 1 trace for `sync`, and would show whether `pool` still splits. Not measured yet.
- **Go-tracer header capture beyond the edge span:** OBI's source and its `TestSuiteGoBodyExtraction` suggest this is supported, but it didn't happen here. Worth an upstream issue.
- `errgroup`: not measured. It's built on `go`, so it would probably behave like `go`.
