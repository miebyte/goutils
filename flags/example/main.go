package main

import (
	"fmt"

	"github.com/miebyte/goutils/flags"
)

var (
	boolGetter   = flags.Bool("bool", false, "bool config")
	stringGetter = flags.String("string", "thisisdefault", "str config")
	sliceGetter  = flags.StringSlice("slice", []string{"test1", "test2"}, "slice config")
)

func main() {
	flags.Parse()

	fmt.Println(boolGetter())
	fmt.Println(stringGetter())
	fmt.Println(sliceGetter())

}
