package uplink

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// ZapLogger adapts the project's zap logger to the aprsutils.Logger interface
// expected by the APRS client (passed via client.WithLogger).
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
