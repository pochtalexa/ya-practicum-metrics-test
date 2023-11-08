package middlefunc

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/pochtalexa/ya-practicum-metrics/internal/server/flags"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"strings"
)

// проверяем, что клиент готов принимать gzip данные
func getReqContEncoding(r *http.Request) bool {

	encodingSlice := r.Header.Values("Accept-Encoding")
	encodingsStr := strings.Join(encodingSlice, ",")
	encodings := strings.Split(encodingsStr, ",")
	log.Info().Str("encodingsStr", encodingsStr).Msg("getReqContEncoding")

	for _, el := range encodings {
		if el == "gzip" {
			return true
		}
	}

	return false
}

func GzipDecompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentEncoding := r.Header.Get("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, "gzip")

		if sendsGzip {
			gzipReader, err := gzip.NewReader(r.Body)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			r.Body = gzipReader
		}

		if getReqContEncoding(r) {
			w.Header().Set("Content-Encoding", "gzip")
			log.Info().Msg("set Content-Encoding gzip")
		}

		log.Info().Str("r.URL", r.URL.String()).Msg("GzipDecompression")
		log.Info().Msg("GzipDecompression passed")

		next.ServeHTTP(w, r)
	})
}

func checkSign(r *http.Request) (bool, error) {

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return false, fmt.Errorf("io.ReadAll: %w", err)
	}
	log.Info().Str("bodyBytes", string(bodyBytes)).Msg("checkSign")

	reqHeaderHash := r.Header.Get("HashSHA256")
	h := hmac.New(sha256.New, []byte(flags.FlagHashKey))
	h.Write(bodyBytes)
	dst := h.Sum(nil)

	// We need to set the body again because it was drained by io.ReadAll
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	result := reqHeaderHash == hex.EncodeToString(dst)

	log.Info().Str("reqHeaderHash", reqHeaderHash).Msg("checkSign")
	log.Info().Str("hex.EncodeToString(dst)", hex.EncodeToString(dst)).Msg("checkSign")

	return result, nil
}

func CheckReqBodySign(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if flags.UseHashKey {
			if checkResult, err := checkSign(r); err != nil {
				log.Info().Err(err).Msg("CheckReqBodySign error")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)

				return
			} else if !checkResult {
				log.Info().Err(err).Msg("CheckReqBodySign checkResult error")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)

				return
			}
			log.Info().Msg("CheckReqBodySign success")
		}
		next.ServeHTTP(w, r)
	})
}
