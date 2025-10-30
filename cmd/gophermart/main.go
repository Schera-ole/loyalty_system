package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/Schera-ole/loyalty_system/internal/config"
	"github.com/Schera-ole/loyalty_system/internal/handler"
	"github.com/Schera-ole/loyalty_system/internal/migration"
	"github.com/Schera-ole/loyalty_system/internal/repository"
	"github.com/Schera-ole/loyalty_system/internal/service"
	"go.uber.org/zap"
)

func main() {
	// Initialize config
	systemConfig, err := config.NewSystemConfig()
	if err != nil {
		log.Fatal("Failed to parse configuration: ", err)
	}

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to initialize zap logger: ", err)
	}
	defer logger.Sync()
	logSugar := logger.Sugar()

	// Check migrations
	migCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = migration.RunMigrations(migCtx, systemConfig.DatabaseURI, logSugar)
	if err != nil {
		logSugar.Errorf("%v", err)
	}

	// Initialize database storage
	dbStorage, err := repository.NewDBStorage(systemConfig.DatabaseURI)
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
		"database", systemConfig.DatabaseURI,
	)

	// Start server
	logSugar.Fatal(
		http.ListenAndServe(
			systemConfig.RunAddress,
			handler.Router(logSugar, systemConfig, loyaltyService),
		),
	)
}
