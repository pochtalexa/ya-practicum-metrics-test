package handlers

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/pochtalexa/ya-practicum-metrics/internal/server/models"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pochtalexa/ya-practicum-metrics/internal/server/flags"
	"github.com/pochtalexa/ya-practicum-metrics/internal/server/storage"
)

// структура для хранения сведений об ответе
type responseData struct {
	status       int
	contEncoding string
	size         int
}

func GetStore() storage.Storer {
	if flags.StorePoint.DataBase {
		return storage.DBstorage
	} else {
		return storage.MemStorage
	}
}

// проверяем, что клиент отправил серверу сжатые данные в формате gzip
func reqCheckGzipBody(r *http.Request) (io.ReadCloser, error) {
	contentEncoding := r.Header.Get("Content-Encoding")
	sendsGzip := strings.Contains(contentEncoding, "gzip")

	if sendsGzip {
		gzr, err := gzip.NewReader(r.Body)
		if err != nil {
			return r.Body, err
		}
		defer gzr.Close()
		// добавляем к телу запроса обертку gzip
		r.Body = gzr
	}

	return r.Body, nil
}

// добавляем кастомную реализацию http.ResponseWriter
type loggingGzipResponseWriter struct {
	// встраиваем оригинальный http.ResponseWriter
	http.ResponseWriter
	responseData *responseData
	resCompress  bool // требуется ли сжимать ответ
}

func (r *loggingGzipResponseWriter) Write(b []byte) (int, error) {
	var (
		size int
		err  error
		gzw  *gzip.Writer
	)

	if r.resCompress {
		gzw, err = gzip.NewWriterLevel(r.ResponseWriter, gzip.BestSpeed)
		if err != nil {
			return -1, err
		}
		defer gzw.Close()

		r.responseData.contEncoding = "gzip" // сохраняем значение contEncoding

		size, err = gzw.Write(b)

	} else {
		// записываем ответ, используя оригинальный http.ResponseWriter
		size, err = r.ResponseWriter.Write(b)
	}

	r.responseData.size += size // захватываем размер

	return size, err
}

func (r *loggingGzipResponseWriter) WriteHeaderStatus(statusCode int) {
	// записываем код статуса, используя оригинальный http.ResponseWriter
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode // захватываем код статуса
}

func logHTTPResult(start time.Time, lw loggingGzipResponseWriter, r http.Request,
	Req []models.Metrics,
	Res []models.Metrics,
	optErr ...error) {
	err := errors.New("null")
	if len(optErr) > 0 {
		err = optErr[0]
	}

	for _, v := range Req {
		log.Info().
			Str("URI", r.URL.Path).
			Str("Method", r.Method).
			Str("Header-HashSHA256", r.Header.Get("HashSHA256")).
			Str("Req", v.String()).
			Dur("duration", time.Since(start)).
			Msg("request")
	}

	for _, v := range Res {
		log.Info().
			Str("Status", strconv.Itoa(lw.responseData.status)).
			Str("Content-Length", strconv.Itoa(lw.responseData.size)).
			Str("Res", v.String()).
			Err(err).
			Msg("response")
	}
}

func UpdateMetric(reqJSON models.Metrics, repo storage.Storer) error {
	if reqJSON.MType == "gauge" {
		value := reqJSON.Value
		if value == nil {
			return fmt.Errorf("bad gauge value")
		}
		repo.SetGauge(reqJSON.ID, storage.Gauge(*value))
	} else if reqJSON.MType == "counter" {
		value := reqJSON.Delta
		if value == nil {
			return fmt.Errorf("bad counetr delta")
		}
		repo.UpdateCounter(reqJSON.ID, storage.Counter(*value))
	} else {
		return fmt.Errorf("bad metric type: %s", reqJSON.MType)
	}

	return nil
}

// проверяем, что клиент готов принимать gzip данные
func getReqContEncoding(r *http.Request) bool {

	encodingSlice := r.Header.Values("Accept-Encoding")
	encodingsStr := strings.Join(encodingSlice, ",")
	encodings := strings.Split(encodingsStr, ",")

	for _, el := range encodings {
		if el == "gzip" {
			return true
		}
	}

	return false
}

func UpdateHandlerLong(w http.ResponseWriter, r *http.Request) {
	var (
		valCounter       storage.Counter
		valGauge         storage.Gauge
		ok               bool
		reqJSON, resJSON models.Metrics
	)
	start := time.Now()
	repo := GetStore()

	responseData := &responseData{
		status: 0,
		size:   0,
	}
	lw := loggingGzipResponseWriter{
		ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
		responseData:   responseData,
		resCompress:    false,
	}

	reqJSON.ID = chi.URLParam(r, "metricName")
	reqJSON.MType = chi.URLParam(r, "metricType")

	if reqJSON.MType == "counter" {
		counterVal, err := strconv.ParseInt(chi.URLParam(r, "metricVal"), 10, 64)
		if err != nil {
			lw.WriteHeaderStatus(http.StatusBadRequest)
			logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
			return
		}
		reqJSON.Delta = &counterVal
	} else if reqJSON.MType == "gauge" {
		gaugeVal, err := strconv.ParseFloat(chi.URLParam(r, "metricVal"), 64)
		if err != nil {
			lw.WriteHeaderStatus(http.StatusBadRequest)
			logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
			return
		}
		reqJSON.Value = &gaugeVal
	} else {
		err := fmt.Errorf("can not get val for %v from repo", reqJSON.MType)
		lw.WriteHeaderStatus(http.StatusBadRequest)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	lw.Header().Set("Content-Type", "application/json")
	lw.Header().Set("Date", time.Now().String())

	err := UpdateMetric(reqJSON, repo)
	if err != nil {
		lw.WriteHeaderStatus(http.StatusBadRequest)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	resJSON = reqJSON
	if resJSON.MType == "counter" {
		if valCounter, ok, _ = repo.GetCounter(resJSON.ID); ok {
			valCounterI64 := int64(valCounter)
			resJSON.Delta = &valCounterI64
		}
	} else if resJSON.MType == "gauge" {
		if valGauge, ok, _ = repo.GetGauge(resJSON.ID); ok {
			valGaugeF64 := float64(valGauge)
			resJSON.Value = &valGaugeF64
		}
	} else {
		err := fmt.Errorf("can not get val for %v from repo", reqJSON.ID)
		lw.WriteHeaderStatus(http.StatusBadRequest)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	if ok {
		lw.WriteHeaderStatus(http.StatusOK)
	} else {
		lw.WriteHeaderStatus(http.StatusNotFound)
	}

	if flags.FlagStoreInterval == 0 && flags.StorePoint.File {
		err = repo.StoreMetrics()
		if err != nil {
			lw.WriteHeaderStatus(http.StatusInternalServerError)
			logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
			return
		}
	}

	logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON})
}

func UpdateHandler(w http.ResponseWriter, r *http.Request) {
	var (
		reqJSON, resJSON models.Metrics
		valCounter       storage.Counter
		valGauge         storage.Gauge
		ok               bool
		err              error
	)
	start := time.Now()
	repo := GetStore()

	responseData := &responseData{
		status: 0,
		size:   0,
	}
	lw := loggingGzipResponseWriter{
		ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
		responseData:   responseData,
		resCompress:    false,
	}

	if lw.resCompress = getReqContEncoding(r); lw.resCompress {
		lw.Header().Set("Content-Encoding", "gzip")
	}

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&reqJSON); err != nil {
		lw.WriteHeaderStatus(http.StatusInternalServerError)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	lw.Header().Set("Content-Type", "application/json")
	lw.Header().Set("Date", time.Now().String())

	err = UpdateMetric(reqJSON, repo)
	if err != nil {
		lw.WriteHeaderStatus(http.StatusBadRequest)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	resJSON.ID = reqJSON.ID
	resJSON.MType = reqJSON.MType

	if resJSON.MType == "counter" {
		if valCounter, ok, _ = repo.GetCounter(resJSON.ID); ok {
			valCounterI64 := int64(valCounter)
			resJSON.Delta = &valCounterI64
		}
	} else if resJSON.MType == "gauge" {
		if valGauge, ok, _ = repo.GetGauge(resJSON.ID); ok {
			valGaugeF64 := float64(valGauge)
			resJSON.Value = &valGaugeF64
		}
	} else {
		err := fmt.Errorf("can not get val for %v from repo", resJSON.ID)
		lw.WriteHeaderStatus(http.StatusBadRequest)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	lw.WriteHeaderStatus(http.StatusOK)

	enc := json.NewEncoder(&lw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resJSON); err != nil {
		lw.WriteHeaderStatus(http.StatusBadRequest)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	if flags.FlagStoreInterval == 0 && flags.StorePoint.File {
		err = repo.StoreMetrics()
		if err != nil {
			lw.WriteHeaderStatus(http.StatusInternalServerError)
			logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
			return
		}
	}

	logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON})
}

// UpdatesHandler обновление метрик - при условии получения их в виде массива
func UpdatesHandler(w http.ResponseWriter, r *http.Request) {
	var (
		reqJSON, resJSON []models.Metrics
		err              error
	)

	repo := GetStore()

	type responseBody struct {
		Description string `json:"description"` // имя метрики
	}

	start := time.Now()

	responseData := &responseData{
		status: 0,
		size:   0,
	}
	lw := loggingGzipResponseWriter{
		ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
		responseData:   responseData,
		resCompress:    false,
	}

	if lw.resCompress = getReqContEncoding(r); lw.resCompress {
		lw.Header().Set("Content-Encoding", "gzip")
	}

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&reqJSON); err != nil {
		lw.WriteHeaderStatus(http.StatusInternalServerError)
		logHTTPResult(start, lw, *r, reqJSON, resJSON, err)
		return
	}

	lw.Header().Set("Content-Type", "application/json")
	lw.Header().Set("Date", time.Now().String())

	err = repo.UpdateMetricBatch(reqJSON)
	if err != nil {
		lw.WriteHeaderStatus(http.StatusBadRequest)
		logHTTPResult(start, lw, *r, reqJSON, resJSON, err)
		log.Info().Err(err).Msg("DB UpdateMetricBatch error")
		return
	}

	allMetrics, _ := repo.GetAllMetrics()

	for k, v := range allMetrics.Gauges {
		tempV := float64(v)
		metrics := models.Metrics{
			ID:    k,
			MType: "gauge",
			Value: &tempV,
		}
		resJSON = append(resJSON, metrics)
	}

	for k, v := range allMetrics.Counters {
		tempV := int64(v)
		metrics := models.Metrics{
			ID:    k,
			MType: "counter",
			Delta: &tempV,
		}
		resJSON = append(resJSON, metrics)
	}

	lw.WriteHeaderStatus(http.StatusOK)

	enc := json.NewEncoder(&lw)
	enc.SetIndent("", "  ")
	if err := enc.Encode(resJSON); err != nil {
		lw.WriteHeaderStatus(http.StatusBadRequest)
		logHTTPResult(start, lw, *r, reqJSON, resJSON, err)
		return
	}

	logHTTPResult(start, lw, *r, reqJSON, resJSON)
}

func ValueHandlerLong(w http.ResponseWriter, r *http.Request) {
	var (
		valCounter       storage.Counter
		valGauge         storage.Gauge
		ok               bool
		data             string
		reqJSON, resJSON models.Metrics
	)
	start := time.Now()
	repo := GetStore()

	responseData := &responseData{
		status: 0,
		size:   0,
	}
	lw := loggingGzipResponseWriter{
		ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
		responseData:   responseData,
		resCompress:    false,
	}

	reqJSON.ID = chi.URLParam(r, "metricName")
	reqJSON.MType = chi.URLParam(r, "metricType")

	lw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	lw.Header().Set("Date", time.Now().String())

	resJSON = reqJSON
	if resJSON.MType == "counter" {
		if valCounter, ok, _ = repo.GetCounter(resJSON.ID); ok {
			data = fmt.Sprintf("%d", valCounter)
		}
	} else if resJSON.MType == "gauge" {
		if valGauge, ok, _ = repo.GetGauge(resJSON.ID); ok {
			data = fmt.Sprintf("%.3f", valGauge)
			data = strings.Trim(data, "0")
		}
	} else {
		err := fmt.Errorf("can not get val for %v from repo", reqJSON.ID)
		lw.WriteHeaderStatus(http.StatusBadRequest)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	if ok {
		lw.WriteHeaderStatus(http.StatusOK)
		lw.Write([]byte(data))
	} else {
		lw.WriteHeaderStatus(http.StatusNotFound)
	}

	logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON})

}

func ValueHandler(w http.ResponseWriter, r *http.Request) {
	var (
		valCounter       storage.Counter
		valGauge         storage.Gauge
		ok               bool
		err              error
		reqJSON, resJSON models.Metrics
	)
	start := time.Now()
	repo := GetStore()

	responseData := &responseData{
		status:       0,
		contEncoding: "",
		size:         0,
	}
	lw := loggingGzipResponseWriter{
		ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
		responseData:   responseData,
		resCompress:    false,
	}

	r.Body, err = reqCheckGzipBody(r)
	if err != nil {
		lw.WriteHeaderStatus(http.StatusInternalServerError)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	if lw.resCompress = getReqContEncoding(r); lw.resCompress {
		lw.Header().Set("Content-Encoding", "gzip")
	}

	dec := json.NewDecoder(r.Body)
	if err = dec.Decode(&reqJSON); err != nil {
		lw.WriteHeaderStatus(http.StatusInternalServerError)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	lw.Header().Set("Content-Type", "application/json")
	lw.Header().Set("Date", time.Now().String())

	resJSON.ID = reqJSON.ID
	resJSON.MType = reqJSON.MType

	if resJSON.MType == "counter" {
		if valCounter, ok, _ = repo.GetCounter(resJSON.ID); ok {
			valCounterI64 := int64(valCounter)
			resJSON.Delta = &valCounterI64
		}
	} else if resJSON.MType == "gauge" {
		if valGauge, ok, _ = repo.GetGauge(reqJSON.ID); ok {
			valGaugeF64 := float64(valGauge)
			resJSON.Value = &valGaugeF64
		}
	} else {
		err = fmt.Errorf("can not get val for %v from repo", resJSON.MType)
		lw.WriteHeaderStatus(http.StatusBadRequest)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	if ok {
		lw.WriteHeaderStatus(http.StatusOK)
		enc := json.NewEncoder(&lw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(resJSON); err != nil {
			lw.WriteHeaderStatus(http.StatusBadRequest)
			logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
			return
		}
	} else {
		err = fmt.Errorf("can not get val for <%v>, type <%v> from repo", reqJSON.ID, reqJSON.MType)
		lw.WriteHeaderStatus(http.StatusNotFound)
	}

	logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	var reqJSON, resJSON models.Metrics

	start := time.Now()
	repo := GetStore()

	responseData := &responseData{
		status:       0,
		contEncoding: "",
		size:         0,
	}
	lw := loggingGzipResponseWriter{
		ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
		responseData:   responseData,
		resCompress:    false,
	}

	if lw.resCompress = getReqContEncoding(r); lw.resCompress {
		lw.Header().Set("Content-Encoding", "gzip")
	}

	lw.Header().Set("Content-Type", "text/html")
	lw.Header().Set("Date", time.Now().String())

	WebPage1, _ := gauges2String(repo.GetGauges())
	WebPage2, _ := сounters2String(repo.GetCounters())
	WebPage := fmt.Sprintf(`<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<title>Document</title>
	</head>
	<body>
		<h3>Metric values</h3>
		<h5>gauges</h5>
		<p> %s </p>
		<p> </p>
		<h5>counters</h5>
		<p> %s </p>
	</body>
	</html>`, WebPage1, WebPage2)

	data := []byte(WebPage)

	lw.WriteHeaderStatus(http.StatusOK)

	if _, err := lw.Write(data); err != nil {
		log.Info().Err(err).Msg("RootHandler")
		lw.WriteHeaderStatus(http.StatusInternalServerError)
		return
	}

	logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON})
}

func PingHandler(w http.ResponseWriter, r *http.Request) {
	var reqJSON, resJSON models.Metrics

	db := storage.DBstorage.DBconn
	start := time.Now()

	responseData := &responseData{
		status:       0,
		contEncoding: "",
		size:         0,
	}
	lw := loggingGzipResponseWriter{
		ResponseWriter: w, // встраиваем оригинальный http.ResponseWriter
		responseData:   responseData,
		resCompress:    false,
	}

	err := storage.PingDB(db)
	if err != nil {
		lw.WriteHeaderStatus(http.StatusInternalServerError)
		logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
		return
	}

	lw.WriteHeaderStatus(http.StatusOK)
	logHTTPResult(start, lw, *r, []models.Metrics{reqJSON}, []models.Metrics{resJSON}, err)
}

func сounters2String(mapCounters map[string]storage.Counter, _ error) (string, error) {
	var storeList []string

	for k, v := range mapCounters {
		storeList = append(storeList, k+":"+fmt.Sprintf("%d", v))
	}

	return strings.Join(storeList, ","), nil
}

func gauges2String(mapGauges map[string]storage.Gauge, _ error) (string, error) {
	var storeList []string

	for k, v := range mapGauges {
		storeList = append(storeList, k+":"+fmt.Sprintf("%.3f", v))
	}

	return strings.Join(storeList, ","), nil
}
