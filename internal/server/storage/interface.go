package storage

import "github.com/pochtalexa/ya-practicum-metrics/internal/server/models"

type Storer interface {
	GetGauge(name string) (Gauge, bool, error)
	GetGauges() (map[string]Gauge, error)
	SetGauge(name string, value Gauge) error
	GetCounter(name string) (Counter, bool, error)
	GetCounters() (map[string]Counter, error)
	UpdateCounter(name string, value Counter) error
	GetAllMetrics() (Store, error)
	UpdateMetricBatch([]models.Metrics) error
	StoreMetrics() error
	RestoreMetrics() error
}
