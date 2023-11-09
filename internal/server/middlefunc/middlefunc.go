package middlefunc

import (
	"bytes"
	"compress/gzip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"strings"
	"test.test/internal/server/flags"
)

// проверяем, что клиент отправил серверу сжатые данные в формате gzip
func checkGzipEncoding(r *http.Request) bool {

	encodingSlice := r.Header.Values("Content-Encoding")
	encodingsStr := strings.Join(encodingSlice, ",")
	encodings := strings.Split(encodingsStr, ",")

	log.Info().Str("encodingsStr", encodingsStr).Msg("checkGzipEncoding")

	for _, el := range encodings {
		if el == "gzip" {
			return true
		}
	}

	return false
}

func GzipDecompression(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		if checkGzipEncoding(r) {
			gzipReader, err := gzip.NewReader(r.Body)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			r.Body = gzipReader
			defer gzipReader.Close()
		}

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
		reqHeaderHash := r.Header.Get("HashSHA256")
		if flags.UseHashKey && reqHeaderHash != "" {
			if checkResult, err := checkSign(r); err != nil {
				log.Info().Err(err).Msg("CheckReqBodySign error")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				io.Copy(w, r.Body)

				return
			} else if !checkResult {
				log.Info().Err(err).Msg("CheckReqBodySign checkResult error")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				io.Copy(w, r.Body)

				return
			}
			log.Info().Msg("CheckReqBodySign success")
		}
		next.ServeHTTP(w, r)
	})
}
