package websocketutils

import "fmt"

func callHandlerSafely(handler EventHandler, ctx *Context) {
	defer func() {
		if r := recover(); r != nil {
			Logger().Errorf("event handler panic event=%s conn=%s err=%s", ctx.Event(), ctx.Conn().ID(), fmt.Sprint(r))
		}
	}()
	handler(ctx)
}
