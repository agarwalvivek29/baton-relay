// orders fans out to payments and inventory. How it fans out is the point of
// the demo: FANOUT_MODE (or ?mode=) picks one of
//
//	sync  both calls on the request goroutine, one after the other
//	go    one `go func()` per call, joined with a WaitGroup
//	pool  a worker pool started at boot, fed jobs over a channel
//
// In go and pool modes the job carries the request's ctx, so baton still
// forwards the request ID even where eBPF can't tell which request a
// goroutine belongs to.
//
// DROP_CONTEXT=1 (or ?drop_ctx=1) makes the payments call with
// context.Background(): the sharp edge that breaks the chain.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/agarwalvivek29/baton"
)

var (
	logger *slog.Logger
	client *http.Client
)

// job is one downstream call handed to the worker pool. It carries ctx
// explicitly: the worker goroutine was not started by this request.
type job struct {
	ctx  context.Context
	name string
	url  string
	done chan<- result
}

type result struct {
	name   string
	status int
	err    error
}

func main() {
	logger = slog.New(baton.LogHandler(slog.NewJSONHandler(os.Stdout, nil))).With("service", "orders")
	client = &http.Client{Transport: baton.Transport(nil, baton.WithOnMissing(func(r *http.Request) {
		logger.WarnContext(r.Context(), "outbound call has no request ID", "url", r.URL.String())
	}))}
	payments := env("PAYMENTS_URL", "http://localhost:8082")
	inventory := env("INVENTORY_URL", "http://localhost:8083")
	defaultMode := env("FANOUT_MODE", "sync")
	dropDefault := os.Getenv("DROP_CONTEXT") == "1"

	jobs := make(chan job)
	for i := 0; i < 4; i++ {
		go worker(jobs)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := r.URL.Query()
		mode := q.Get("mode")
		if mode == "" {
			mode = defaultMode
		}
		payCtx := ctx
		if dropDefault || q.Get("drop_ctx") == "1" {
			payCtx = context.Background() // the bug: the request ID is gone
		}
		payURL := payments + "/charge?fail=" + q.Get("fail")
		invURL := inventory + "/reserve"
		logger.InfoContext(ctx, "order received", "mode", mode)

		var results []result
		switch mode {
		case "sync":
			results = []result{call(payCtx, "payments", payURL), call(ctx, "inventory", invURL)}
		case "go":
			results = make([]result, 2)
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); results[0] = call(payCtx, "payments", payURL) }()
			go func() { defer wg.Done(); results[1] = call(ctx, "inventory", invURL) }()
			wg.Wait()
		case "pool":
			done := make(chan result, 2)
			jobs <- job{ctx: payCtx, name: "payments", url: payURL, done: done}
			jobs <- job{ctx: ctx, name: "inventory", url: invURL, done: done}
			results = []result{<-done, <-done}
		default:
			http.Error(w, "unknown mode "+mode, http.StatusBadRequest)
			return
		}

		for _, res := range results {
			if res.err != nil || res.status >= 500 {
				logger.ErrorContext(ctx, "order failed", "mode", mode, "failed", res.name, "status", res.status, "err", res.err)
				http.Error(w, res.name+" failed", http.StatusBadGateway)
				return
			}
		}
		logger.InfoContext(ctx, "order placed", "mode", mode)
		fmt.Fprintf(w, "order placed (mode=%s, request_id=%s)\n", mode, baton.FromContext(ctx))
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {})

	logger.Info("listening", "addr", ":8081", "fanout_mode", defaultMode, "drop_context", dropDefault)
	if err := http.ListenAndServe(":8081", baton.Middleware(mux)); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func worker(jobs <-chan job) {
	for j := range jobs {
		j.done <- call(j.ctx, j.name, j.url)
	}
}

func call(ctx context.Context, name, url string) result {
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return result{name: name, err: err}
	}
	resp, err := client.Do(req)
	if err != nil {
		return result{name: name, err: err}
	}
	resp.Body.Close()
	return result{name: name, status: resp.StatusCode}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
