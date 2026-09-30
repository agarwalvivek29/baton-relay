// gateway is the edge. It has no inbound request ID, so baton creates one here.
//
//	GET /checkout[?fail=1][&mode=sync|go|pool][&drop_ctx=1] → orders
package main

import (
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/agarwalvivek29/baton"
)

func main() {
	logger := slog.New(baton.LogHandler(slog.NewJSONHandler(os.Stdout, nil))).With("service", "gateway")
	client := &http.Client{Transport: baton.Transport(nil, baton.WithOnMissing(func(r *http.Request) {
		logger.WarnContext(r.Context(), "outbound call has no request ID", "url", r.URL.String())
	}))}
	orders := env("ORDERS_URL", "http://localhost:8081")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /checkout", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logger.InfoContext(ctx, "checkout started")

		req, _ := http.NewRequestWithContext(ctx, "POST", orders+"/orders?"+r.URL.RawQuery, nil)
		resp, err := client.Do(req)
		if err != nil {
			logger.ErrorContext(ctx, "orders unreachable", "err", err)
			http.Error(w, "orders unreachable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode >= 500 {
			logger.ErrorContext(ctx, "checkout failed", "status", resp.StatusCode)
		} else {
			logger.InfoContext(ctx, "checkout done", "status", resp.StatusCode)
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {})

	logger.Info("listening", "addr", ":8080")
	if err := http.ListenAndServe(":8080", baton.Middleware(mux)); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
