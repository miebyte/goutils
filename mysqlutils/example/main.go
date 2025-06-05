package main

import (
	"context"

	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/flags"
)

func main() {
	flags.Parse()
	ExampleSimpleTableRouting()
	logging.Infoc(context.TODO(), "example done")
}
