package cores

import (
	"context"
	"time"

	"github.com/miebyte/goutils/consulutils"
	"github.com/miebyte/goutils/internal/innerlog"
	"github.com/miebyte/goutils/share"
)

// WithConsulRegister 显式启用 Consul 注册，仅在 Start 的服务模式下生效。
// 服务名来自进程配置，业务端口和健康检查端口分别从已绑定的 listener 获取。
// 注册配置或网络请求失败只记录日志，不影响业务服务的运行。
func WithConsulRegister(configs ...consulutils.RegistrationConfig) ServiceOption {
	return func(cs *CoresService) {
		if len(configs) > 1 {
			cs.consulConfig = nil
			innerlog.Logger.Warnc(cs.ctx, "Consul registration disabled: expected at most one configuration")
			return
		}

		var config consulutils.RegistrationConfig
		if len(configs) == 1 {
			config = configs[0]
		}
		cs.consulConfig = &config
	}
}

// startConsulRegistration 为注册单独管理取消和等待，不占用业务 Worker 的退出预算。
func (c *CoresService) startConsulRegistration() func() {
	// 如果不在服务模式，或者没有配置 Consul 注册时不允许注册
	if !c.serviceMode || c.consulConfig == nil {
		return nil
	}

	// 没有启动业务监听器，则不允许注册
	if c.businessListener == nil {
		innerlog.Logger.Warnc(c.ctx, "Consul registration disabled: no business listener")
		return nil
	}

	// 解析业务监听器的端口
	// _, portText, err := net.SplitHostPort(c.businessListener.Addr().String())
	// if err != nil {
	// 	innerlog.Logger.Warnc(c.ctx, "Consul registration disabled: invalid business listener address: %v", err)
	// 	return nil
	// }
	// port, err := strconv.Atoi(portText)
	// if err != nil {
	// 	innerlog.Logger.Warnc(c.ctx, "Consul registration disabled: invalid business listener port: %v", err)
	// 	return nil
	// }

	registration, err := consulutils.NewRegistration(
		share.GetServiceName(),
		c.businessListener.Addr().String(),
		c.internalHTTP.listener.Addr().String(),
		*c.consulConfig,
	)
	if err != nil {
		innerlog.Logger.Warnc(c.ctx, "Consul registration disabled: %v", err)
		return nil
	}

	ctx, cancel := context.WithCancel(c.ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		registration.Run(ctx)
	}()

	return func() {
		// 先等待注册循环结束，再注销，避免后台重试把实例重新加回来。
		// 已发送请求的远端结果可能不确定，异常残留由健康检查和自动注销兜底。
		cancel()
		<-done

		// 服务上下文可能已取消，清理请求必须使用独立的有限超时上下文。
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		err := registration.Deregister(ctx)
		if err != nil {
			innerlog.Logger.Warnc(c.ctx, "Consul deregistration failed: %v", err)
		} else {
			innerlog.Logger.Infoc(c.ctx, "Consul deregistration successful")
		}
	}
}
