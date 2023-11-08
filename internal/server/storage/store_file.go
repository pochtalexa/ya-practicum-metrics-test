package storage

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"os"
)

type StoreFile struct {
	file    *os.File
	encoder *json.Encoder
}

type RestoreFile struct {
	file    *os.File
	decoder *json.Decoder
}

func NewStoreFile(fileName string) (*StoreFile, error) {
	err := os.Truncate(fileName, 0)
	if err != nil {
		log.Info().Err(err).Msg("can not Truncate file")
	}

	file, err := os.OpenFile(fileName, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}

	return &StoreFile{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

func NewRestoreFile(fileName string) (*RestoreFile, error) {
	file, err := os.OpenFile(fileName, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}

	return &RestoreFile{
		file:    file,
		decoder: json.NewDecoder(file),
	}, nil
}

func (s *StoreFile) WriteMetrics(metric Store, _ error) error {
	return s.encoder.Encode(&metric)
}

func (s *StoreFile) Close() error {
	return s.file.Close()
}

func (s *RestoreFile) ReadMetrics(metric *Store) error {
	return s.decoder.Decode(&metric)
}

func (s *RestoreFile) Close() error {
	return s.file.Close()
}
