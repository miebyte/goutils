package structutils

import (
	"github.com/go-playground/mold/v4"
	"github.com/go-playground/mold/v4/modifiers"
)

var modify = modifiers.New()

func RegisterModifier(tag string, fn mold.Func) {
	modify.Register(tag, fn)
}

func RegisterModifierAlias(alias, tag string) {
	modify.RegisterAlias(alias, tag)
}

func Modifier() *mold.Transformer {
	return modify
}
