package main

import (
	"ffiibtc/internal/config"
	"ffiibtc/internal/router"
	"net/http"

	"github.com/go-pkgz/lgr"
)

func main() {
	l := lgr.New(lgr.Debug, lgr.CallerFunc)
	l.Logf("INFO ffiibtc started")

	cfg, err := config.NewConfig(l)
	if err != nil {
		l.Logf("FATAL getting config: %v", err)
	}
	l.Logf("INFO connected to Firefly at %s", cfg.FFApp)

	r := router.NewRouter()
	r.AddRoute("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if err := r.Run(8080); err != nil {
		l.Logf("FATAL server error: %v", err)
	}
}
