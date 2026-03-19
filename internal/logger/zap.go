package logger

import (
	"os"

	"github.com/APRSCN/aprsgo/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var L *zap.Logger

func InitLogger() {
	// Set log level
	var level zapcore.Level
	if config.Debug {
		level = zap.DebugLevel
	} else {
		level = zap.InfoLevel
	}

	// Create config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	cores := make([]zapcore.Core, 0)

	stdoutCore := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.Lock(os.Stdout),
		level,
	)
	cores = append(cores, stdoutCore)

	// Output to log file
	if config.Get().Log.File != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   config.Get().Log.File,
			MaxSize:    config.Get().Log.MaxSize,
			MaxBackups: config.Get().Log.MaxBackups,
			MaxAge:     config.Get().Log.MaxAge,
			Compress:   config.Get().Log.Compress,
			LocalTime:  true,
		}

		fileCore := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(fileWriter),
			level,
		)
		cores = append(cores, fileCore)
	}

	// Create zap core
	core := zapcore.NewTee(cores...)

	// Create zap logger
	L = zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel))

	L.Debug("Logger initialized")
}
