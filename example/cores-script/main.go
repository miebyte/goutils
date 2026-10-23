package main

import (
	"context"

	"github.com/miebyte/goutils/cores"
	"github.com/miebyte/goutils/flags/v2"
	"github.com/miebyte/goutils/logging"
	"github.com/miebyte/goutils/share"
)

var message = flags.String("message", "执行一次性任务", "脚本输出内容")

func main() {
	share.SetServiceName("cores-script")
	flags.Parse()
	defer flags.Close()

	srv := cores.NewCores(
		cores.WithNameWorker("script", func(ctx context.Context) error {
			// 在这里调用一次性应用用例；任务结束后 Run 会自动返回。
			logging.Infoc(ctx, "%s", message())
			return nil
		}),
	)

	// Run 是脚本模式，不监听管理端口，也不执行 Consul 注册。
	// 长期运行的 HTTP、gRPC、消费者和定时 Worker 应使用 cores.Start。
	logging.PanicError(cores.Run(srv))
}
