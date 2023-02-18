package log

import (
	"go.uber.org/zap"
)

var Logger *zap.Logger

func InitProductionLogger() {
	Logger, _ = zap.NewProduction()
}

func InitDevelopmentLogger() {
	Logger, _ = zap.NewDevelopment()
}
