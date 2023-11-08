package main

import (
	"fmt"
	"github.com/pochtalexa/ya-practicum-metrics/internal/agent/flags"
	"github.com/pochtalexa/ya-practicum-metrics/internal/agent/metrics"
	"github.com/pochtalexa/ya-practicum-metrics/internal/agent/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

func InitMultiLogger() *os.File {
	fileLogger, err := os.OpenFile(
		"client.log",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0664,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("InitMultiLogger")
	}

	writers := io.MultiWriter(os.Stdout, fileLogger)
	log.Logger = log.Output(writers)

	log.Info().Msg("MultiWriter logger initiated")

	return fileLogger
}

func main() {
	var (
		CashMetrics     metrics.CashMetrics
		runtimeStorage  = metrics.NewRuntimeMetrics()
		gopsutilStorage = metrics.NewGopsutilMetrics()
		wg              sync.WaitGroup
		err             error
	)

	tr := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    30 * time.Second,
		DisableCompression: false,
	}

	multiLogger := InitMultiLogger()
	defer multiLogger.Close()

	flags.ParseFlags()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	httpClient := http.Client{Transport: tr}

	chCashMetrics := make(chan models.Metric, 100)
	chCashMetricsResult := make(chan error, flags.FlagWorkers)
	chgopsutilUpdateMetricsResult := make(chan error, 1)

	// создаем пул воркеров
	for i := 0; i < flags.FlagWorkers; i++ {
		workerID := i
		go func() {
			metrics.SendMetricWorker(workerID, chCashMetrics, chCashMetricsResult, httpClient, flags.FlagRunAddr)
		}()
	}

	// горутина принимаем ошибки от SendMetricWorker и gopsutilStorage.UpdateMetrics
	go func() {
		var err error
		for {
			select {
			case err = <-chCashMetricsResult:
				log.Info().Err(err).Msg("SendMetricWorker error")
			case err = <-chgopsutilUpdateMetricsResult:
				log.Info().Err(err).Msg("gopsutilStorage.UpdateMetrics error")
			}
		}
	}()

	// горутина: runtimeStorage.UpdateMetrics сбора метрик с заданным интервалом
	wg.Add(1)
	go func() {
		log.Info().Msg("runtimeStorage.UpdateMetrics started")

		for range time.Tick(flags.PollInterval) {
			runtimeStorage.UpdateMetrics()
			log.Info().Msg("runtimeStorage Metrics updated")
		}
		wg.Done()
	}()

	// горутина: gopsutilStorage.UpdateMetrics() сбора метрик с заданным интервалом
	wg.Add(1)
	go func() {
		log.Info().Msg("gopsutilStorage.UpdateMetrics started")

		for range time.Tick(flags.PollInterval) {
			err := gopsutilStorage.UpdateMetrics()
			if err != nil {
				chgopsutilUpdateMetricsResult <- err
			} else {
				log.Info().Msg("gopsutilStorage Metrics updated")
			}
		}
		wg.Done()
	}()

	// горутина: CollectMetrics подготовка кеша для отправки
	// передача кеша в горутину SendMetricBatch и канал chCashMetrics
	wg.Add(1)
	go func() {
		var wgCollect sync.WaitGroup
		log.Info().Msg("CollectMetrics started")

		for range time.Tick(flags.ReportInterval) {
			CashMetrics, err = metrics.CollectMetrics(runtimeStorage, gopsutilStorage)
			if err != nil {
				log.Fatal().Err(err).Msg("CollectMetrics")
			}
			runtimeStorage.PollCountDrop()
			log.Info().Msg("CollectMetrics done")

			for _, v := range CashMetrics.CashMetrics {
				// if the channel is full, the default case will be executed
				select {
				case chCashMetrics <- v:
					continue
				default:
					log.Info().
						Err(fmt.Errorf("chCashMetrics is full")).
						Str("CashMetric", v.String()).
						Msg("can not pass metric to channel")
				}
			}

			wgCollect.Add(1)
			go func() {
				err = metrics.SendMetricBatch(CashMetrics, httpClient, flags.FlagRunAddr)
				if err != nil {
					log.Info().Err(err).Msg("SendMetricBatch send error")
				}
				log.Info().Msg("SendMetricBatch done")
				wgCollect.Done()
			}()

			wgCollect.Wait()
		}

		wg.Done()
	}()

	wg.Wait()
}
