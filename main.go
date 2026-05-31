package main

import (
	"ffiibtc/internal/bootstrap"
	"ffiibtc/internal/config"
	"ffiibtc/internal/firefly"
	"ffiibtc/internal/handlers"
	"ffiibtc/internal/router"
	"net/http"
	"time"

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

	httpClient := &http.Client{Timeout: time.Duration(config.FireflyAppTimeout) * time.Second}
	ffClient := firefly.NewClient(cfg, httpClient)

	cls, err := bootstrap.LoadOrTrain(config.ModelFile, ffClient, l)
	if err != nil {
		l.Logf("FATAL initializing classifier: %v", err)
	}

	srv := handlers.NewServer(cls)
	_ = srv

	r := router.NewRouter()
	r.AddRoute("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if err := r.Run(8080); err != nil {
		l.Logf("FATAL server error: %v", err)
	}
}
