// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package log provide different level of log message.
// it's wrapper based on std package log
package log

import (
	"fmt"
	_log "log"
	"os"
	"strings"
)

// log level
const (
	TRACE = iota + 1
	INFO
	WARNING
	ERROR
)

// Color numbers for stdout
const (
	black = (iota + 30)
	red
	green
	yellow
	blue
	magenta
	cyan
	white
)

type logger struct {
	level int
	_log.Logger
}

var def logger

func init() {

	def.level = WARNING
	def.SetOutput(os.Stdout)
	def.SetFlags(_log.LstdFlags)
}

// SetLevel configure log level
func SetLevel(logLevel string) bool {
	var level int

	switch logLevel {
	case "warning":
		level = WARNING

	case "error":
		level = ERROR

	case "trace":
		level = TRACE

	case "info":
		level = INFO
	default:
		return false
	}

	if level >= TRACE && level <= ERROR {
		def.level = level
	}
	return true
}

// Error output level of ERROR message
func Error(format string, v ...interface{}) {
	var str strings.Builder

	if def.level > ERROR {
		return
	}
	str.WriteString(fmt.Sprintf("\x1b[0;%dm%s \x1b[0m", red, "Error: "))
	str.WriteString(fmt.Sprintf(format, v...))
	def.Output(2, str.String())
}

// Warning output level of WARNING message
func Warning(format string, v ...interface{}) {
	var str strings.Builder

	if def.level > WARNING {
		return
	}

	str.WriteString(fmt.Sprintf("\x1b[0;%dm%s \x1b[0m", yellow, "Warning: "))
	str.WriteString(fmt.Sprintf(format, v...))
	def.Output(2, str.String())
}

// Info output level of INFO message
func Info(format string, v ...interface{}) {
	var str strings.Builder

	if def.level > INFO {
		return
	}

	str.WriteString(fmt.Sprintf(format, v...))
	def.Output(2, str.String())
}

// Trace output level of TRACE message
func Trace(format string, v ...interface{}) {
	var str strings.Builder

	if def.level > TRACE {
		return
	}

	str.WriteString(fmt.Sprintf(format, v...))
	def.Output(2, str.String())
}
