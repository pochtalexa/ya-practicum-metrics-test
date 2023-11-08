package flags

import (
	"flag"
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type StoragePoint struct {
	Memory   bool
	File     bool
	DataBase bool
}

var (
	FlagRunAddr       string
	FlagStoreInterval int
	FlagFileStorePath string
	FlagRestore       bool
	FlagDBConn        string
	StorePoint        StoragePoint
	FlagHashKey       string
	UseHashKey        bool
	err               error
)

func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func ParseFlags() {
	defaultFileStorePath := "/tmp/metrics-db.json"
	if opSyst := runtime.GOOS; strings.Contains(opSyst, "windows") {
		defaultFileStorePath = "c:/tmp/metrics-db.json"
	}

	defaultDBConn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		`localhost`, `5432`, `praktikum`, `praktikum`, `praktikum`)

	//defaultHashKey := "0123456789ABCDEF"
	defaultHashKey := ""

	flag.StringVar(&FlagRunAddr, "a", ":8080", "addr to run on")
	flag.IntVar(&FlagStoreInterval, "i", 300, "save to file interval (sec)")
	flag.StringVar(&FlagFileStorePath, "f", defaultFileStorePath, "file to save")
	flag.BoolVar(&FlagRestore, "r", true, "load metrics on start from file")
	flag.StringVar(&FlagDBConn, "d", defaultDBConn, "db conn string")
	flag.StringVar(&FlagHashKey, "k", defaultHashKey, "hashKey")
	flag.Parse()

	if envVar := os.Getenv("ADDRESS"); envVar != "" {
		FlagRunAddr = envVar
	}

	if envVar := os.Getenv("STORE_INTERVAL"); envVar != "" {
		FlagStoreInterval, err = strconv.Atoi(envVar)
		if err != nil {
			log.Fatal().Err(err).Msg("FlagStoreInterval")
		}
	}

	if envVar := os.Getenv("FILE_STORAGE_PATH"); envVar != "" {
		FlagFileStorePath = envVar
	}

	if envVar := os.Getenv("RESTORE"); envVar != "" {
		FlagRestore, err = strconv.ParseBool(envVar)
		if err != nil {
			log.Fatal().Err(err).Msg("FlagRestore")
		}
	}

	if envVar := os.Getenv("DATABASE_DSN"); envVar != "" {
		FlagDBConn = envVar
		if err != nil {
			log.Fatal().Err(err).Msg("FlagDBConn")
		}
	}

	if isFlagPassed(FlagDBConn) || os.Getenv("DATABASE_DSN") != "" {
		StorePoint.DataBase = true
	} else if isFlagPassed(FlagFileStorePath) || FlagFileStorePath != "" {
		StorePoint.File = true
	} else {
		StorePoint.Memory = true
	}

	if envHashKey := os.Getenv("KEY"); envHashKey != "" {
		FlagHashKey = envHashKey
	}

	if !isFlagPassed(FlagHashKey) && os.Getenv("KEY") == "" {
		UseHashKey = false
	} else {
		UseHashKey = true
	}
	//UseHashKey = true

	log.Info().
		Str("UseHashKey", strconv.FormatBool(UseHashKey)).
		Str("FlagHashKey", FlagHashKey).
		Msg("UseHashKey server")
}
