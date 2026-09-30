// inventory reserves stock. It always succeeds; it exists so orders has two
// downstream calls to fan out.
package main

import (
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/agarwalvivek29/baton"
)

func main() {
	logger := slog.New(baton.LogHandler(slog.NewJSONHandler(os.Stdout, nil))).With("service", "inventory")

	mux := http.NewServeMux()
	mux.HandleFunc("POST /reserve", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(10+rand.Intn(20)) * time.Millisecond)
		logger.InfoContext(r.Context(), "stock reserved")
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {})

	logger.Info("listening", "addr", ":8083")
	if err := http.ListenAndServe(":8083", baton.Middleware(mux)); err != nil {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}
