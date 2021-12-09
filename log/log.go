package log

import (
	"fmt"
	"runtime"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	ALL = iota + 0
	DEBUG
	INFO
	WARN
	ERROR
)

const (
	_callerInfo = "NoCallerFile"
)

var DefaultDebugLevel = DEBUG

const CallHierarchy int = 1

var _log *zap.Logger

func JSON(v interface{}) string {
	return fmt.Sprintf("%+v", v)
}

func ToJSON(v interface{}) string {
	return fmt.Sprintf("%+v", v)
}

func Info(p string, f ...zapcore.Field) {
	if DefaultDebugLevel >= INFO {
		return
	}

	_, file, line, ok := runtime.Caller(CallHierarchy)
	callerInfo := _callerInfo

	if ok {
		callerInfo = fmt.Sprintf("%s:%d", file, line)
	}

	t := []zapcore.Field{zap.String("caller", callerInfo)}
	t = append(t, f...)
	_log.Info(p, t...)
}

func Warn(p string, f ...zapcore.Field) {
	if DefaultDebugLevel >= WARN {
		return
	}

	_, file, line, ok := runtime.Caller(CallHierarchy)
	callerInfo := _callerInfo

	if ok {
		callerInfo = fmt.Sprintf("%s:%d", file, line)
	}

	t := []zapcore.Field{zap.String("caller", callerInfo)}
	t = append(t, f...)
	_log.Warn(p, t...)
}

func Debug(p string, f ...zapcore.Field) {
	if DefaultDebugLevel >= DEBUG {
		return
	}

	_, file, line, ok := runtime.Caller(CallHierarchy)
	callerInfo := _callerInfo

	if ok {
		callerInfo = fmt.Sprintf("%s:%d", file, line)
	}

	t := []zapcore.Field{zap.String("caller", callerInfo)}
	t = append(t, f...)
	_log.Debug(p, t...)
}

func Error(p string, f ...zapcore.Field) {
	if DefaultDebugLevel >= ERROR {
		return
	}

	_, file, line, ok := runtime.Caller(CallHierarchy)
	callerInfo := _callerInfo

	if ok {
		callerInfo = fmt.Sprintf("%s:%d", file, line)
	}

	t := []zapcore.Field{zap.String("caller", callerInfo)}
	t = append(t, f...)
	_log.Error(p, t...)
}

func Init(servername string) {
	_log, _ = zap.NewProduction()
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:     "msg",
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder, // 小写编码器
		EncodeTime:     zapcore.ISO8601TimeEncoder,    // ISO8601 UTC 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.FullCallerEncoder, // 全路径编码器
	}
	atom := zap.NewAtomicLevelAt(zap.DebugLevel)
	config := zap.Config{
		Level:            atom,                                             // 日志级别
		Development:      true,                                             // 开发模式，堆栈跟踪
		Encoding:         "json",                                           // 输出格式 console 或 json
		EncoderConfig:    encoderConfig,                                    // 编码器配置
		InitialFields:    map[string]interface{}{"servername": servername}, // 初始化字段，如：添加一个服务器名称
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	var e error
	_log, e = config.Build()

	if e != nil {
		panic(fmt.Sprintf("log 初始化失败: %v", e))
	}
}
