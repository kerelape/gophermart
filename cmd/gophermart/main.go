package main

import (
	"github.com/kerelape/gophermart/internal/gophermart"
	"github.com/pior/runnable"
	"log"
)

func main() {
	config, parseConfigError := ParseConfig()
	if parseConfigError != nil {
		log.Fatal(parseConfigError)
	}

	runnable.Run(
		gophermart.New(
			config.AddressRun,
			config.AddressAccrualSystem,
			config.AddressDatabase,
			config.JWTSecretKey,
		),
	)
}
