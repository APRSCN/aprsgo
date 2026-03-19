package uplink

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// Logger interface definition
type Logger interface {
	Debug(context.Context, ...any)
	Info(context.Context, ...any)
	Warn(context.Context, ...any)
	Error(context.Context, ...any)
}

// ZapLogger construct logger adapter for Ghink OpenAPI SDK with zap
type ZapLogger struct {
	logger *zap.Logger
}

// Debug build Debug level log
func (z *ZapLogger) Debug(ctx context.Context, args ...any) {
	z.logger.Debug(fmt.Sprint(args...))
}

// Info build Info level log
func (z *ZapLogger) Info(ctx context.Context, args ...any) {
	z.logger.Info(fmt.Sprint(args...))
}

// Warn build Warn level log
func (z *ZapLogger) Warn(ctx context.Context, args ...any) {
	z.logger.Warn(fmt.Sprint(args...))
}

// Error build Error level log
func (z *ZapLogger) Error(ctx context.Context, args ...any) {
	z.logger.Error(fmt.Sprint(args...))
}
