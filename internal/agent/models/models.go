package models

import "encoding/json"

// Metric структура для обработки тела POST запроса в формате JSON
type Metric struct {
	ID    string   `json:"id"`              // имя метрики
	MType string   `json:"type"`            // параметр, принимающий значение gauge или counter
	Delta *int64   `json:"delta,omitempty"` // значение метрики в случае передачи counter
	Value *float64 `json:"value,omitempty"` // значение метрики в случае передачи gauge
}

func (m *Metric) String() string {
	jsonRes, _ := json.Marshal(m)
	return string(jsonRes)
}
