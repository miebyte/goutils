// File:		config.go
// Created by:	Hoven
// Created on:	2025-04-03
//
// This file is part of the Example Project.
//
// (c) 2024 Example Corp. All rights reserved.

package logging

import "gopkg.in/natefinch/lumberjack.v2"

type LogConfig lumberjack.Logger

func (l *LogConfig) Write(p []byte) (n int, err error) {
	return (*lumberjack.Logger)(l).Write(p)
}

func (l *LogConfig) SetDefault() {
	if l.Filename == "" {
		l.Filename = "runlog.log"
	}

	if l.MaxAge == 0 {
		l.MaxAge = 30
	}

	if l.MaxBackups == 0 {
		l.MaxBackups = 3
	}

	if l.MaxSize == 0 {
		l.MaxSize = 3
	}
}
