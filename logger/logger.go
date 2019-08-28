package logger

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a logger with the supplied configuration.
func NewLogger(configJSON string, levelOverride string, opts ...zap.Option) *zap.SugaredLogger {
	var loggingCfg zap.Config
	if err := json.Unmarshal([]byte(configJSON), &loggingCfg); err != nil {
		return newFallbackLogger(err, levelOverride, opts...)
	}
	if len(levelOverride) > 0 {
		if level, err := levelFromString(levelOverride); err == nil {
			loggingCfg.Level = zap.NewAtomicLevelAt(*level)
		}
	}

	logger, err := loggingCfg.Build(opts...)
	if err != nil {
		return newFallbackLogger(err, levelOverride, opts...)
	}

	logger.Info("Successfully created the logger.", zap.String("jsonconfig", configJSON))
	logger.Sugar().Infof("Logging level set to %v", loggingCfg.Level)

	return logger.Sugar()
}

func levelFromString(level string) (*zapcore.Level, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("invalid logging level: %v", level)
	}
	return &zapLevel, nil
}

func newFallbackLogger(cause error, levelOverride string, opts ...zap.Option) *zap.SugaredLogger {
	loggingCfg := zap.NewProductionConfig()
	if len(levelOverride) > 0 {
		if level, err := levelFromString(levelOverride); err == nil {
			loggingCfg.Level = zap.NewAtomicLevelAt(*level)
		}
	}

	logger, err := loggingCfg.Build(opts...)
	if err != nil {
		panic(err)
	}
	logger = logger.Named("fallback-logger")

	logger.Warn("Failed to create logger, using fallback:", zap.Error(cause))
	logger.Sugar().Infof("Logging level set to %v", loggingCfg.Level)

	return logger.Sugar()
}
