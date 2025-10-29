package config

import (
	"flag"
	"os"
)

type SystemConfig struct {
	RunAddress     string
	DatabaseURI    string
	AccrualAddress string
	JwtSecretKey   string
	JwtAlgorithm   string
}

func NewSystemConfig() (*SystemConfig, error) {
	config := &SystemConfig{
		RunAddress:     "localhost:8080",
		DatabaseURI:    "postgresql://xxx:xxx@localhost:5432/loyalty_system?sslmode=disable",
		AccrualAddress: "localhost:8088",
		JwtSecretKey:   "random_secret_key",
		JwtAlgorithm:   "HS256",
	}

	address := flag.String("a", config.RunAddress, "address")
	database := flag.String("d", config.DatabaseURI, "database uri")
	accural := flag.String("r", config.AccrualAddress, "accural system address")

	envVars := map[string]*string{
		"RUN_ADDRESS":            address,
		"DATABASE_URI":           database,
		"ACCRUAL_SYSTEM_ADDRESS": accural,
	}

	for envVar, flag := range envVars {
		if envValue := os.Getenv(envVar); envValue != "" {
			*flag = envValue
		}
	}
	config.RunAddress = *address
	config.DatabaseURI = *database
	config.AccrualAddress = *accural

	return config, nil
}
