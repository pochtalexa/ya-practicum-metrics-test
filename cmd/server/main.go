package main

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/pochtalexa/ya-practicum-metrics/internal/server/flags"
	"github.com/pochtalexa/ya-practicum-metrics/internal/server/handlers"
	"github.com/pochtalexa/ya-practicum-metrics/internal/server/middlefunc"
	"github.com/pochtalexa/ya-practicum-metrics/internal/server/migrations"
	"github.com/pochtalexa/ya-practicum-metrics/internal/server/storage"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// можно не оборачивать в retry
func catchTermination() {
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)
	repo := handlers.GetStore()

	<-shutdownChan
	err := repo.StoreMetrics()
	if err != nil {
		log.Fatal().Err(err).Msg("catchTermination")
	}

	os.Exit(0)
}

func initStoreTimer() {
	repo := handlers.GetStore()

	if flags.FlagStoreInterval > 0 {
		for range time.Tick(time.Second * time.Duration(flags.FlagStoreInterval)) {
			err := repo.StoreMetrics()
			if err != nil {
				log.Info().Err(err).Msg("initStoreTimer StoreMetrics")
			}
		}
	}
}

func restoreMetrics() error {
	repo := handlers.GetStore()

	if flags.FlagRestore {
		err := repo.RestoreMetrics()
		if err != nil {
			return fmt.Errorf("repo.RestoreMetrics: %w", err)
		}
	}

	return nil
}

func run() error {

	mux := chi.NewRouter()
	mux.Use(middlefunc.GzipDecompression)
	mux.Use(middleware.Logger)

	// return all metrics on WEB page
	mux.Get("/", handlers.RootHandler)

	// ping DB
	mux.Get("/ping", handlers.PingHandler)

	// get metrics in array
	//mux.Post("/updates/", handlers.UpdatesHandler)
	mux.Route("/updates", func(r chi.Router) {
		r.Use(middlefunc.CheckReqBodySign)
		r.Post("/", handlers.UpdatesHandler)
	})

	mux.Post("/update/{metricType}/{metricName}/{metricVal}", handlers.UpdateHandlerLong)
	//mux.Post("/update/", handlers.UpdateHandler)
	mux.Route("/update", func(r chi.Router) {
		r.Use(middlefunc.CheckReqBodySign)
		r.Post("/", handlers.UpdateHandler)
	})

	mux.Get("/value/{metricType}/{metricName}", handlers.ValueHandlerLong)
	//mux.Post("/value/", handlers.ValueHandler)
	mux.Route("/value", func(r chi.Router) {
		r.Use(middlefunc.CheckReqBodySign)
		r.Post("/", handlers.ValueHandler)
	})

	log.Info().Str("Running on", flags.FlagRunAddr).Msg("Server started")
	defer log.Info().Msg("Server stopped")

	return http.ListenAndServe(flags.FlagRunAddr, mux)
}

func main() {
	var err error

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	flags.ParseFlags()

	if flags.StorePoint.DataBase {
		err = storage.InitConnDB()
		if err != nil {
			log.Fatal().Err(err).Msg("DB conn error")
		}
		defer storage.DBstorage.DBconn.Close()

		err = migrations.ApplyMigrations()
		if err != nil {
			log.Fatal().Err(err).Msg("migration error")
		}
	}

	go catchTermination()
	restoreMetrics()
	go initStoreTimer()

	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("run mux")
	}
}
