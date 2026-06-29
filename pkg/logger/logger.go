package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config 日志配置
type Config struct {
	Filename     string // 日志文件路径
	Level        string // 日志级别
	MaxSizeMB    int    // 单个文件最大MB
	MaxBackups   int    // 最大备份数
	RetentionDays int   // 保留天数
	Compress     bool   // 是否压缩旧日志
	Stdout       bool   // 是否输出到控制台
}

// DefaultConfig 默认配置
var DefaultConfig = Config{
	Filename:      "logs/nmsappsrv.log",
	Level:         "info",
	MaxSizeMB:     100,
	MaxBackups:    10,
	RetentionDays: 30,
	Compress:      true,
	Stdout:        true,
}

var (
	globalLogger *zap.SugaredLogger
	globalAtomicLevel zap.AtomicLevel
	once sync.Once
)

// Init 初始化全局日志
func Init(cfg ...Config) {
	once.Do(func() {
		c := DefaultConfig
		if len(cfg) > 0 {
			c = cfg[0]
		}
		globalLogger = buildLogger(c)
	})
}

// buildLogger 构建logger
func buildLogger(cfg Config) *zap.SugaredLogger {
	globalAtomicLevel = zap.NewAtomicLevel()
	setLevel(cfg.Level)

	encoderCfg := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		TimeKey:        "ts",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     utcTimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	var cores []zapcore.Core

	// 文件输出
	if cfg.Filename != "" {
		dir := filepath.Dir(cfg.Filename)
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create log dir %s: %v\n", dir, err)
		}
		fileWriter := &lumberjack.Logger{
			Filename:   cfg.Filename,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.RetentionDays,
			Compress:   cfg.Compress,
		}
		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			zapcore.AddSync(fileWriter),
			globalAtomicLevel,
		))
	}

	// 控制台输出
	if cfg.Stdout {
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		cores = append(cores, zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderCfg),
			zapcore.AddSync(os.Stdout),
			globalAtomicLevel,
		))
	}

	if len(cores) == 0 {
		cores = append(cores, zapcore.NewCore(
			zapcore.NewConsoleEncoder(encoderCfg),
			zapcore.AddSync(os.Stdout),
			globalAtomicLevel,
		))
	}

	core := zapcore.NewTee(cores...)
	l := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	return l.Sugar()
}

func utcTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.UTC().Format("2006-01-02T15:04:05.000Z"))
}

func setLevel(level string) {
	switch strings.ToLower(level) {
	case "debug":
		globalAtomicLevel.SetLevel(zap.DebugLevel)
	case "info":
		globalAtomicLevel.SetLevel(zap.InfoLevel)
	case "warn":
		globalAtomicLevel.SetLevel(zap.WarnLevel)
	case "error":
		globalAtomicLevel.SetLevel(zap.ErrorLevel)
	case "dpanic":
		globalAtomicLevel.SetLevel(zap.DPanicLevel)
	case "panic":
		globalAtomicLevel.SetLevel(zap.PanicLevel)
	case "fatal":
		globalAtomicLevel.SetLevel(zap.FatalLevel)
	default:
		globalAtomicLevel.SetLevel(zap.InfoLevel)
	}
}

// Sync 刷新日志缓冲
func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
}

// Cleanup 清理并关闭
func Cleanup() {
	Sync()
}

func getLogger() *zap.SugaredLogger {
	if globalLogger == nil {
		Init()
	}
	return globalLogger
}

// 全局日志函数
func Debug(args ...interface{})        { getLogger().Debug(args...) }
func Info(args ...interface{})         { getLogger().Info(args...) }
func Warn(args ...interface{})         { getLogger().Warn(args...) }
func Error(args ...interface{})        { getLogger().Error(args...) }
func DPanic(args ...interface{})       { getLogger().DPanic(args...) }
func Panic(args ...interface{})        { getLogger().Panic(args...) }
func Fatal(args ...interface{})        { getLogger().Fatal(args...) }
func Debugf(template string, args ...interface{}) { getLogger().Debugf(template, args...) }
func Infof(template string, args ...interface{})  { getLogger().Infof(template, args...) }
func Warnf(template string, args ...interface{})  { getLogger().Warnf(template, args...) }
func Errorf(template string, args ...interface{}) { getLogger().Errorf(template, args...) }
func DPanicf(template string, args ...interface{}) { getLogger().DPanicf(template, args...) }
func Panicf(template string, args ...interface{})  { getLogger().Panicf(template, args...) }
func Fatalf(template string, args ...interface{})  { getLogger().Fatalf(template, args...) }
func Debugw(msg string, keysAndValues ...interface{}) { getLogger().Debugw(msg, keysAndValues...) }
func Infow(msg string, keysAndValues ...interface{})  { getLogger().Infow(msg, keysAndValues...) }
func Warnw(msg string, keysAndValues ...interface{})  { getLogger().Warnw(msg, keysAndValues...) }
func Errorw(msg string, keysAndValues ...interface{}) { getLogger().Errorw(msg, keysAndValues...) }
