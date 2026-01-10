package logger

import (
	"encoding/json"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

var FoundLogLevel = os.Getenv("LOG_LEVEL")

func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	default:
		return "info"
	}
}

func ParseLogLevel(level string) LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return DebugLevel
	case "info":
		return InfoLevel
	case "warn", "warning":
		return WarnLevel
	case "error":
		return ErrorLevel
	default:
		return InfoLevel
	}
}

func toZapLevel(level LogLevel) zapcore.Level {
	switch level {
	case DebugLevel:
		return zapcore.DebugLevel
	case InfoLevel:
		return zapcore.InfoLevel
	case WarnLevel:
		return zapcore.WarnLevel
	case ErrorLevel:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

var globalLogger *zap.SugaredLogger

type Logger struct {
	sugar *zap.SugaredLogger
}

func Init(minLevel LogLevel) error {
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "service",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(toZapLevel(minLevel)),
		Development:      false,
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build(zap.AddCallerSkip(1))
	if err != nil {
		return err
	}

	globalLogger = logger.Sugar()
	return nil
}

func New(service string, minLevel LogLevel) *Logger {
	if globalLogger == nil {
		if err := Init(minLevel); err != nil {
			panic(err)
		}
	}

	return &Logger{
		sugar: globalLogger.Named(service),
	}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.sugar.Debugf(format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.sugar.Infof(format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.sugar.Warnf(format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.sugar.Errorf(format, args...)
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.sugar.Fatalf(format, args...)
}

func (l *Logger) WithService(service string) *Logger {
	return &Logger{
		sugar: l.sugar.Named(service),
	}
}

func Debug(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debugf(format, args...)
	}
}

func Info(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Infof(format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warnf(format, args...)
	}
}

func Error(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Errorf(format, args...)
	}
}

func Fatal(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Fatalf(format, args...)
	}
}

func Sync() error {
	if globalLogger == nil {
		return nil
	}
	// Ignore sync errors for stdout/stderr - they don't support fsync
	_ = globalLogger.Sync()
	return nil
}

type CronLoggerAdapter struct {
	logger  *Logger
	verbose bool
}

func (c *CronLoggerAdapter) Printf(format string, args ...interface{}) {
	if c.verbose {
		c.logger.Debug(format, args...)
	} else {
		c.logger.Info(format, args...)
	}
}

func (c *CronLoggerAdapter) Info(msg string, keysAndValues ...interface{}) {
	c.logger.Info("%s %v", msg, keysAndValues)
}

func (c *CronLoggerAdapter) Error(err error, msg string, keysAndValues ...interface{}) {
	c.logger.Error("%s: %v %v", msg, err, keysAndValues)
}

func (l *Logger) ToCronLogger(verbose bool) *CronLoggerAdapter {
	return &CronLoggerAdapter{
		logger:  l,
		verbose: verbose,
	}
}

func LogObject(obj interface{}, logger *Logger) {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		logger.Error("Failed to marshal object: %v", err)
		return
	}
	logger.Info("Object: %s", string(jsonBytes))
}
