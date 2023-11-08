package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

type metricsSend struct {
	ID    string  `json:"id"`              // имя метрики
	MType string  `json:"type"`            // параметр, принимающий значение gauge или counter
	Delta int64   `json:"delta,omitempty"` // значение метрики в случае передачи counter
	Value float64 `json:"value,omitempty"` // значение метрики в случае передачи gauge
}

type metricsSendBad struct {
	ID    string `json:"id"`              // имя метрики
	MType string `json:"type"`            // параметр, принимающий значение gauge или counter
	Delta string `json:"delta,omitempty"` // значение метрики в случае передачи counter
	Value string `json:"value,omitempty"` // значение метрики в случае передачи gauge
}

func TestUpdateHandler1(t *testing.T) {

	type want struct {
		code        int
		contentType string
		delta       int64
		value       float64
	}

	tests := []struct {
		name string
		url  string
		body metricsSend
		want want
	}{
		{
			name: "positive test #1",
			url:  "/update/",
			body: metricsSend{ID: "Alloc", MType: "counter", Delta: 5},
			want: want{
				code:        http.StatusOK,
				contentType: "application/json",
				delta:       5,
				value:       -1,
			},
		},
		{
			name: "positive test #2",
			url:  "/update/",
			body: metricsSend{ID: "Alloc", MType: "gauge", Value: 11},
			want: want{
				code:        http.StatusOK,
				contentType: "application/json",
				delta:       -1,
				value:       11,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var metric metricsSend

			mux := chi.NewRouter()
			mux.Use(middleware.Logger)

			mux.Post("/update/", UpdateHandler)

			reqBody, _ := json.Marshal(test.body)

			request := httptest.NewRequest(http.MethodPost, test.url, bytes.NewReader(reqBody))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, request)

			res := w.Result()

			dec := json.NewDecoder(res.Body)
			if err := dec.Decode(&metric); err != nil {
				panic(err)
			}
			err := res.Body.Close()
			if err != nil {
				panic(err)
			}

			assert.Equal(t, res.Header.Get("Content-Type"), test.want.contentType)
			assert.Equal(t, res.StatusCode, test.want.code)

			if metric.MType == "counter" {
				assert.Equal(t, metric.Delta, test.want.delta)
			} else if metric.MType == "gauge" {
				assert.Equal(t, metric.Value, test.want.value)
			} else {
				panic(errors.New("incorrect MType"))
			}
		})
	}
}

func TestUpdateHandler2(t *testing.T) {

	type want struct {
		code        int
		contentType string
		delta       int64
		value       float64
	}

	tests := []struct {
		name string
		url  string
		body metricsSendBad
		want want
	}{
		{
			name: "negative test #3",
			url:  "/update/",
			body: metricsSendBad{ID: "Alloc", MType: "gauge", Value: "55"},
			want: want{
				code:        http.StatusInternalServerError,
				contentType: "application/json",
			},
		},
		{
			name: "negative test #4",
			url:  "/",
			want: want{
				code:        http.StatusNotFound,
				contentType: "application/json",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			mux := chi.NewRouter()
			mux.Use(middleware.Logger)

			mux.Post("/update/", UpdateHandler)

			reqBody, _ := json.Marshal(test.body)

			request := httptest.NewRequest(http.MethodPost, test.url, bytes.NewReader(reqBody))
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, request)

			res := w.Result()

			err := res.Body.Close()
			if err != nil {
				panic(err)
			}

			assert.Equal(t, res.StatusCode, test.want.code)
		})
	}
}

func TestGzipCompression3(t *testing.T) {

	type want struct {
		code        int
		contentType string
		contentEnc  string
		delta       int64
		value       float64
	}

	tests := []struct {
		name string
		url  string
		body metricsSend
		want want
	}{
		{
			name: "positive test #31",
			url:  "/update/",
			body: metricsSend{ID: "Alloc", MType: "counter", Delta: 5},
			want: want{
				code:        http.StatusOK,
				contentType: "application/json",
				contentEnc:  "gzip",
				delta:       5,
				value:       -1,
			},
		},
		{
			name: "positive test #32",
			url:  "/update/",
			body: metricsSend{ID: "Alloc", MType: "gauge", Value: 11},
			want: want{
				code:        http.StatusOK,
				contentType: "application/json",
				contentEnc:  "gzip",
				delta:       -1,
				value:       11,
			},
		},
	}

	mux := chi.NewRouter()
	mux.Use(middleware.Logger)

	mux.Post("/update/", UpdateHandler)

	for _, test := range tests {
		// sends_gzip
		t.Run(test.name, func(t *testing.T) {

			reqBody, _ := json.Marshal(test.body)

			var buf bytes.Buffer
			gzipWriter := gzip.NewWriter(&buf)
			_, err := gzipWriter.Write(reqBody)
			assert.NoError(t, err)
			err = gzipWriter.Close()
			assert.NoError(t, err)

			request := httptest.NewRequest(http.MethodPost, test.url, &buf)
			request.Header.Set("Content-Encoding", "gzip")
			request.Header.Set("Accept-Encoding", "gzip")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, request)

			res := w.Result()

			err = res.Body.Close()
			if err != nil {
				panic(err)
			}

			assert.Equal(t, test.want.code, res.StatusCode)
			assert.Equal(t, test.want.contentEnc, res.Header.Get("Content-Encoding"))
		})
	}
}
