package main

import (
	"errors"
	"flag"
	"github.com/caarlos0/env/v8"
)

type Config struct {
	AddressRun           string `env:"RUN_ADDRESS"`
	AddressAccrualSystem string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	AddressDatabase      string `env:"DATABASE_URI"`
	JWTSecretKey         string `env:"JWT_SECRET_KEY"`
}

func ParseConfig() (Config, error) {
	config := Config{}
	if err := env.Parse(&config); err != nil {
		return Config{}, err
	}

	addressRun := flag.String("a", "", "Server run address")
	addressAccrualSystem := flag.String("r", "", "Accrual system address")
	addressDatabase := flag.String("d", "", "Database DSN URI")
	flag.Parse()

	if *addressRun != "" {
		config.AddressRun = *addressRun
	}
	if *addressAccrualSystem != "" {
		config.AddressAccrualSystem = *addressAccrualSystem
	}
	if *addressDatabase != "" {
		config.AddressDatabase = *addressDatabase
	}

	if config.AddressRun == "" {
		return Config{}, errors.New("missing server run address (-a|RUN_ADDRESS)")
	}
	if config.AddressAccrualSystem == "" {
		return Config{}, errors.New("missing accrual system address (-r|ACCRUAL_SYSTEM_ADDRESS)")
	}
	if config.AddressDatabase == "" {
		return Config{}, errors.New("missing database dsn uri (-d|DATABASE_URI)")
	}

	return config, nil
}
