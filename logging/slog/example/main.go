package main

import (
	"context"

	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/logging/level"
	"github.com/miebyte/goutils/logging/slog"
)

func main() {
	// testFileLog()
	testSlog()
}

func testSlog() {
	logger := slog.New()
	logger.Enable(level.LevelDebug)
	ctx := context.Background()

	logger.Infoc(ctx, "this is a message")
	ctx = logging.With(ctx, "group")
	logger.Infoc(ctx, "this is a message")
	logging.Infoc(ctx, "this is a message", "name", "hoven")

	ctx = logging.With(ctx, "group2")
	logger.Infoc(ctx, "this is a message")
	logging.Infoc(ctx, "this is a message", "name", "hoven")
}

func testFileLog() {
	fileLog := &logging.LogConfig{}
	fileLog.SetDefault()

	logger := slog.New()
	logger.SetOutput(fileLog)
	logger.Enable(level.LevelDebug)

	logger.Infoc(context.Background(), "this is a message")
	logger.Debugc(context.Background(), "this is a message")
	logger.Warnc(context.Background(), "this is a message")
	logger.Errorc(context.Background(), "this is a message")

	ctx := logging.With(context.Background(), "group")
	ctx = logging.With(ctx, "handler")
	logger.Infoc(ctx, "this is a ctx message")
	logger.Debugc(ctx, "this is a ctx message")
	logger.Warnc(ctx, "this is a ctx message")
	logger.Errorc(ctx, "this is a ctx message")

	ctx = logging.With(ctx, "name", "hoven")
	logger.Infoc(ctx, "this is a key-value message")
	ctx = logging.With(ctx, "city", "shenzhen")
	logger.Debugc(ctx, "this is a key-value message")
	ctx = logging.With(ctx, "age", 16, "phone", 12345)
	logger.Warnc(ctx, "this is a key-value message")

	ctx = logging.With(ctx, "email", "xxxx", "somethingBad")
	logger.Errorc(ctx, "this is a key-value with bad key message")
}
