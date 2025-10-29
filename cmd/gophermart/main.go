package main

import (
	"context"
	"log"
	"net/http"

	"github.com/Schera-ole/loyalty_system/internal/config"
	"github.com/Schera-ole/loyalty_system/internal/handler"
	"github.com/Schera-ole/loyalty_system/internal/repository"
	"github.com/Schera-ole/loyalty_system/internal/service"
	"go.uber.org/zap"
)

func main() {
	systemConfig, err := config.NewSystemConfig()
	if err != nil {
		log.Fatal("Failed to parse configuration: ", err)
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to initialize zap logger: ", err)
	}
	defer logger.Sync()
	logSugar := logger.Sugar()

	// Initialize database storage
	dbStorage, err := repository.NewDBStorage(systemConfig.DatabaseUri)
	if err != nil {
		log.Fatal("Failed to connect to database: ", err)
	}
	defer dbStorage.Close()

	// Initialize service
	loyaltyService := service.NewLoyaltySystemService(dbStorage, logSugar)

	ctx := context.Background()
	if err := dbStorage.Ping(ctx); err != nil {
		log.Fatal("Failed to ping database: ", err)
	}

	logSugar.Infow(
		"Starting server",
		"run address", systemConfig.RunAddress,
		"accural system address", systemConfig.AccrualAddress,
		"database", systemConfig.DatabaseUri,
	)
	logSugar.Fatal(
		http.ListenAndServe(
			systemConfig.RunAddress,
			handler.Router(logSugar, systemConfig, loyaltyService),
		),
	)
}
