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
		EncodeLevel:    zapcore.LowercaseLevelEncoder, // ???????????????
		EncodeTime:     zapcore.ISO8601TimeEncoder,    // ISO8601 UTC ????????????
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.FullCallerEncoder, // ??????????????????
	}
	atom := zap.NewAtomicLevelAt(zap.DebugLevel)
	config := zap.Config{
		Level:            atom,                                             // ????????????
		Development:      true,                                             // ???????????????????????????
		Encoding:         "json",                                           // ???????????? console ??? json
		EncoderConfig:    encoderConfig,                                    // ???????????????
		InitialFields:    map[string]interface{}{"servername": servername}, // ???????????????????????????????????????????????????
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	var e error
	_log, e = config.Build()

	if e != nil {
		panic(fmt.Sprintf("log ???????????????: %v", e))
	}
}
