package main

import (
	"log"

	"go.uber.org/zap"
)

func initLogger() *zap.Logger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("cannot initialize logger: %v", err)
	}
	return logger
}

func main() {
	logger := initLogger()
	defer logger.Sync()
}
