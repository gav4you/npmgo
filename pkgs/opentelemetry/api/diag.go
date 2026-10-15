// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Translated to Go by the npmgo contributors from @opentelemetry/api 1.9.0 (build/src/api/diag.js,
// diag/types.js, diag/ComponentLogger.js, diag/consoleLogger.js, diag/internal/logLevelLogger.js and
// diag/internal/noopLogger.js).

package api

import (
	"os"

	"github.com/gav4you/npmgo/internal/jsvalue"
	"github.com/gav4you/npmgo/internal/nodeutil"
)

// DiagLogLevel is the level of the diagnostic logger (the DiagLogLevel enum; any number is accepted, as in
// JavaScript).
type DiagLogLevel float64

// The DiagLogLevel members.
const (
	DiagLogLevel_NONE    DiagLogLevel = 0
	DiagLogLevel_ERROR   DiagLogLevel = 30
	DiagLogLevel_WARN    DiagLogLevel = 50
	DiagLogLevel_INFO    DiagLogLevel = 60
	DiagLogLevel_DEBUG   DiagLogLevel = 70
	DiagLogLevel_VERBOSE DiagLogLevel = 80
	DiagLogLevel_ALL     DiagLogLevel = 9999
)

// DiagLogger receives the diagnostic messages of the API and the SDK.
type DiagLogger interface {
	Error(message string, args ...any)
	Warn(message string, args ...any)
	Info(message string, args ...any)
	Debug(message string, args ...any)
	Verbose(message string, args ...any)
}

// DiagLoggerOptions are the options of Diag.SetLogger. A nil LogLevel stands for undefined: INFO.
type DiagLoggerOptions struct {
	LogLevel                *DiagLogLevel
	SuppressOverrideMessage bool
}

// ComponentLoggerOptions are the options of Diag.CreateComponentLogger.
type ComponentLoggerOptions struct {
	Namespace string
}

const diagAPIName = "diag"

// DiagAPI is the diagnostic logging API (JavaScript's DiagAPI class; its instance is Diag). Its logging methods pass
// their arguments to the registered logger, if any.
type DiagAPI struct{}

// SetLogger registers logger as the global diagnostic logger behind a filter for the level and reports whether it was
// registered. optionsOrLogLevel is a DiagLogLevel (or another number), a DiagLoggerOptions (or a pointer to one) or
// nothing (INFO). Unless the options suppress it, a logger registered before is told (as is the new one) that it is
// being replaced. Diag itself cannot be its own logger: that is logged as an error and refused.
//
// JavaScript's messages carry stack traces (Error.stack); Go's end where the V8 frames would start: "Error: <message>"
// for the self-logger error and "... from Error" in the override messages.
func (d *DiagAPI) SetLogger(logger DiagLogger, optionsOrLogLevel ...any) bool {
	if l, ok := logger.(*DiagAPI); ok && l == d {
		d.Error("Error: Cannot use diag as the logger for itself. Please use a DiagLogger implementation like ConsoleDiagLogger or a custom implementation")
		return false
	}
	level := DiagLogLevel_INFO
	suppress := false
	var opt any = jsvalue.Undefined{}
	if len(optionsOrLogLevel) > 0 {
		opt = optionsOrLogLevel[0]
	}
	switch o := opt.(type) {
	case DiagLogLevel:
		level = o
	case DiagLoggerOptions:
		if o.LogLevel != nil {
			level = *o.LogLevel
		}
		suppress = o.SuppressOverrideMessage
	case *DiagLoggerOptions:
		if o == nil {
			panic(&jsvalue.TypeError{Message: "Cannot read properties of null (reading 'logLevel')"})
		}
		if o.LogLevel != nil {
			level = *o.LogLevel
		}
		suppress = o.SuppressOverrideMessage
	case nil:
		// null: only undefined selects the default.
		panic(&jsvalue.TypeError{Message: "Cannot read properties of null (reading 'logLevel')"})
	default:
		// A number is a level; any other value is an options object without the options set.
		if jsvalue.IsNumber(opt) {
			level = DiagLogLevel(jsvalue.Float(opt))
		}
	}
	oldLogger, _ := getGlobal(diagAPIName).(DiagLogger)
	newLogger := createLogLevelDiagLogger(level, logger)
	if oldLogger != nil && !jsvalue.IsNull(oldLogger) && !suppress {
		const stack = "Error" // new Error().stack, without its frames
		oldLogger.Warn("Current logger will be overwritten from " + stack)
		newLogger.Warn("Current logger will overwrite one already registered from " + stack)
	}
	return registerGlobal(diagAPIName, newLogger, d, true)
}

// Disable removes the global diagnostic logger.
func (d *DiagAPI) Disable() { unregisterGlobal(diagAPIName, d) }

// CreateComponentLogger returns a logger that passes its messages to the global logger with the namespace (default
// "DiagComponentLogger") as the first argument.
func (d *DiagAPI) CreateComponentLogger(options ComponentLoggerOptions) DiagLogger {
	ns := options.Namespace
	if ns == "" {
		ns = "DiagComponentLogger"
	}
	return &diagComponentLogger{namespace: ns}
}

// logger returns the global logger, or nil.
func (d *DiagAPI) logger() DiagLogger {
	l, _ := getGlobal(diagAPIName).(DiagLogger)
	if jsvalue.IsNull(l) {
		return nil
	}
	return l
}

// Verbose logs to the global logger, if any.
func (d *DiagAPI) Verbose(message string, args ...any) {
	if l := d.logger(); l != nil {
		l.Verbose(message, args...)
	}
}

// Debug logs to the global logger, if any.
func (d *DiagAPI) Debug(message string, args ...any) {
	if l := d.logger(); l != nil {
		l.Debug(message, args...)
	}
}

// Info logs to the global logger, if any.
func (d *DiagAPI) Info(message string, args ...any) {
	if l := d.logger(); l != nil {
		l.Info(message, args...)
	}
}

// Warn logs to the global logger, if any.
func (d *DiagAPI) Warn(message string, args ...any) {
	if l := d.logger(); l != nil {
		l.Warn(message, args...)
	}
}

// Error logs to the global logger, if any.
func (d *DiagAPI) Error(message string, args ...any) {
	if l := d.logger(); l != nil {
		l.Error(message, args...)
	}
}

// logFunc is a DiagLogFunction.
type logFunc func(message string, args ...any)

func noopLogFunction(string, ...any) {}

// funcLogger is a DiagLogger made of functions (the object literals of logLevelLogger.js and noopLogger.js).
type funcLogger struct {
	error, warn, info, debug, verbose logFunc
}

func (l *funcLogger) Error(message string, args ...any)   { l.error(message, args...) }
func (l *funcLogger) Warn(message string, args ...any)    { l.warn(message, args...) }
func (l *funcLogger) Info(message string, args ...any)    { l.info(message, args...) }
func (l *funcLogger) Debug(message string, args ...any)   { l.debug(message, args...) }
func (l *funcLogger) Verbose(message string, args ...any) { l.verbose(message, args...) }

// createLogLevelDiagLogger wraps logger so that only messages at maxLevel or more severe reach it. maxLevel is clamped
// to NONE..ALL; a NaN level lets nothing through. A nil logger is an empty object: nothing is logged.
func createLogLevelDiagLogger(maxLevel DiagLogLevel, logger DiagLogger) DiagLogger {
	if maxLevel < DiagLogLevel_NONE {
		maxLevel = DiagLogLevel_NONE
	} else if maxLevel > DiagLogLevel_ALL {
		maxLevel = DiagLogLevel_ALL
	}
	present := logger != nil && !jsvalue.IsNull(logger)
	filter := func(f func() logFunc, level DiagLogLevel) logFunc {
		if present && maxLevel >= level {
			return f()
		}
		return noopLogFunction
	}
	return &funcLogger{
		error:   filter(func() logFunc { return logger.Error }, DiagLogLevel_ERROR),
		warn:    filter(func() logFunc { return logger.Warn }, DiagLogLevel_WARN),
		info:    filter(func() logFunc { return logger.Info }, DiagLogLevel_INFO),
		debug:   filter(func() logFunc { return logger.Debug }, DiagLogLevel_DEBUG),
		verbose: filter(func() logFunc { return logger.Verbose }, DiagLogLevel_VERBOSE),
	}
}

// createNoopDiagLogger is createNoopDiagLogger: a logger whose functions do nothing.
func createNoopDiagLogger() DiagLogger {
	return &funcLogger{noopLogFunction, noopLogFunction, noopLogFunction, noopLogFunction, noopLogFunction}
}

// diagComponentLogger is DiagComponentLogger.
type diagComponentLogger struct {
	namespace string
}

func (c *diagComponentLogger) proxy(f func(DiagLogger) logFunc, message string, args []any) {
	logger := Diag.logger()
	if logger == nil {
		return
	}
	f(logger)(c.namespace, append([]any{message}, args...)...)
}

func (c *diagComponentLogger) Debug(message string, args ...any) {
	c.proxy(func(l DiagLogger) logFunc { return l.Debug }, message, args)
}

func (c *diagComponentLogger) Error(message string, args ...any) {
	c.proxy(func(l DiagLogger) logFunc { return l.Error }, message, args)
}

func (c *diagComponentLogger) Info(message string, args ...any) {
	c.proxy(func(l DiagLogger) logFunc { return l.Info }, message, args)
}

func (c *diagComponentLogger) Warn(message string, args ...any) {
	c.proxy(func(l DiagLogger) logFunc { return l.Warn }, message, args)
}

func (c *diagComponentLogger) Verbose(message string, args ...any) {
	c.proxy(func(l DiagLogger) logFunc { return l.Verbose }, message, args)
}

// ConsoleMethods is the console that DiagConsoleLogger writes to: the console methods it calls, each nil where the
// console has no such function (then console.log is used, and nothing is written when that is nil too).
type ConsoleMethods struct {
	Error, Warn, Info, Debug, Trace, Log func(args ...any)
}

// Console stands for JavaScript's global console object. The default writes as Node's console does without colours:
// util.format(...args) and a newline, to standard output for log, info and debug and to standard error for error, warn
// and trace ("Trace: " first; Node also prints a stack trace there, which Go does not). Replace it, or its methods,
// before logging from several goroutines.
var Console = &ConsoleMethods{
	Error: func(args ...any) { writeConsole(os.Stderr, "", args) },
	Warn:  func(args ...any) { writeConsole(os.Stderr, "", args) },
	Info:  func(args ...any) { writeConsole(os.Stdout, "", args) },
	Debug: func(args ...any) { writeConsole(os.Stdout, "", args) },
	Trace: func(args ...any) { writeConsole(os.Stderr, "Trace: ", args) },
	Log:   func(args ...any) { writeConsole(os.Stdout, "", args) },
}

func writeConsole(w *os.File, prefix string, args []any) {
	w.WriteString(prefix + nodeutil.Format(args...) + "\n")
}

// DiagConsoleLogger is a DiagLogger that writes to Console: Error to console.error, Warn to console.warn, Info to
// console.info, Debug to console.debug and Verbose to console.trace, each falling back to console.log when the
// console lacks the function. The method is looked up on every call.
type DiagConsoleLogger struct{}

// NewDiagConsoleLogger is new DiagConsoleLogger().
func NewDiagConsoleLogger() *DiagConsoleLogger { return &DiagConsoleLogger{} }

func consoleCall(pick func(*ConsoleMethods) func(...any), message string, args []any) {
	c := Console
	if c == nil {
		return
	}
	f := pick(c)
	if f == nil {
		f = c.Log
	}
	if f != nil {
		f(append([]any{message}, args...)...)
	}
}

func (l *DiagConsoleLogger) Error(message string, args ...any) {
	consoleCall(func(c *ConsoleMethods) func(...any) { return c.Error }, message, args)
}

func (l *DiagConsoleLogger) Warn(message string, args ...any) {
	consoleCall(func(c *ConsoleMethods) func(...any) { return c.Warn }, message, args)
}

func (l *DiagConsoleLogger) Info(message string, args ...any) {
	consoleCall(func(c *ConsoleMethods) func(...any) { return c.Info }, message, args)
}

func (l *DiagConsoleLogger) Debug(message string, args ...any) {
	consoleCall(func(c *ConsoleMethods) func(...any) { return c.Debug }, message, args)
}

func (l *DiagConsoleLogger) Verbose(message string, args ...any) {
	consoleCall(func(c *ConsoleMethods) func(...any) { return c.Trace }, message, args)
}
