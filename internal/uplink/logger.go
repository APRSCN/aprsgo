package uplink

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Logger interface definition
type Logger interface {
	Debug(context.Context, ...interface{})
	Info(context.Context, ...interface{})
	Warn(context.Context, ...interface{})
	Error(context.Context, ...interface{})
}

// ZapLogger construct logger adapter for Ghink OpenAPI SDK with zap
type ZapLogger struct {
	logger *zap.Logger
}

// Debug build Debug level log
func (z *ZapLogger) Debug(ctx context.Context, args ...interface{}) {
	z.logger.Debug(fmt.Sprint(args...))
}

// Info build Info level log
func (z *ZapLogger) Info(ctx context.Context, args ...interface{}) {
	z.logger.Info(fmt.Sprint(args...))
}

// Warn build Warn level log
func (z *ZapLogger) Warn(ctx context.Context, args ...interface{}) {
	z.logger.Warn(fmt.Sprint(args...))
}

// Error build Error level log
func (z *ZapLogger) Error(ctx context.Context, args ...interface{}) {
	z.logger.Error(fmt.Sprint(args...))
}
