// payments is the flaky one. ?fail=1 always fails; otherwise it fails at
// FAIL_RATE (default 0.2).
package main

import (
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/agarwalvivek29/baton"
)

func main() {
	logger := slog.New(baton.LogHandler(slog.NewJSONHandler(os.Stdout, nil))).With("service", "payments")
	failRate := 0.2
	if v, err := strconv.ParseFloat(os.Getenv("FAIL_RATE"), 64); err == nil {
		failRate = v
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /charge", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		time.Sleep(time.Duration(20+rand.Intn(30)) * time.Millisecond)
		if r.URL.Query().Get("fail") == "1" || rand.Float64() < failRate {
			logger.ErrorContext(ctx, "charge declined by card processor", "reason", "processor_timeout")
			http.Error(w, "charge failed", http.StatusInternalServerError)
			return
		}
		logger.InfoContext(ctx, "charge ok")
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {})

	logger.Info("listening", "addr", ":8082", "fail_rate", failRate)
	if err := http.ListenAndServe(":8082", baton.Middleware(mux)); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
