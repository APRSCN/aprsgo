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
	if config.C.GetBool("debug") {
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
	logFile := config.C.GetString("log.file")
	if logFile != "" {
		fileWriter := &lumberjack.Logger{
			Filename:   logFile,
			MaxSize:    config.C.GetInt("log.maxSize"),
			MaxBackups: config.C.GetInt("log.maxBackups"),
			MaxAge:     config.C.GetInt("log.maxAge"),
			Compress:   config.C.GetBool("log.compress"),
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

	// Add config change notifier
	config.OnChange = append(config.OnChange, func() {
		L.Info("Config file changed")
	})

	L.Debug("Logger initialized")
}
