---
name: goutils
description: 使用 github.com/miebyte/goutils 创建、扩展或审查采用领域驱动设计（DDD）的 Go 服务。适用于服务启动、配置、HTTP API、日志、MySQL、Redis、队列、邮件、熔断、指标、WebSocket 和常用工具接入；不适用于未使用 goutils 且不需要本规范的通用 Go 项目。
---

# goutils Go 服务开发规范

以 DDD 分层和依赖方向为首要约束，以 `github.com/miebyte/goutils` 提供启动、配置、日志和基础设施能力。先理解业务边界，再决定目录和抽象；不要为了“看起来像 DDD”创建空接口、贫血领域对象或无职责的层。

## 默认依赖方向

```text
API / Interface → Application → Domain ← Infrastructure
                         ↑
                 main 负责依赖组装
```

- Domain 不依赖 API、Application、数据库模型、Gin、GORM、Redis 客户端或其他基础设施实现。
- API 只处理协议绑定、身份提取、应用调用、错误映射和响应，不直接访问数据库、Redis 或消息队列。
- Application 编排用例、事务和跨领域流程；业务不变量保留在 Domain。
- Infrastructure 实现仓储、缓存、队列、邮件和外部系统适配器。
- `main.go` 是组合根：解析配置、创建基础设施、注入应用服务并启动进程。

## 工作方式

1. 先检查目标项目的 `go.mod`、`main.go`、现有目录、生成命令和锁定的 `goutils` 版本。
2. 用业务语言确认限界上下文、聚合边界、不变量和用例，再创建或修改目录。
3. 接口由最窄的直接消费者定义；领域仓储接口放所属上下文，技术端口放 Application。
4. 数据库 Model、Domain Entity/Aggregate、Application DTO 分离；DTO ↔ Domain 转换放 Application 边界，Model ↔ Domain 转换放 Infrastructure 边界，不建跨层的集中转换包。
5. 必选配置在 `Validate()` 中校验，必选依赖在构造函数或组合根校验一次；不要在每个运行期方法重复判空。
6. 持续传递 `context.Context`；长期 Worker 必须响应 `ctx.Done()`。
7. 以目标项目实际 API 为准，不从旧版 `go-utils` 猜测名称。当前库使用 `flags`、`ginutils`、`cores`，不是 `superflags`、`ginlibs`。
8. 只修改当前任务需要的代码；运行格式化、相关测试、生成命令和差异检查。

## MySQL 持久化硬性要求

- 使用 MySQL/GORM 时，必须定义独立的持久化 Model，并使用 `gorm.io/gen` 生成类型安全的 `query`；业务仓储必须实际使用生成的 Query，不能仅生成后闲置。
- Model 与生成的 Query 均放在 Infrastructure；Application/Domain 不依赖 GORM、数据库 Model 或 Query。
- 必须提供可重复执行的生成入口，锁定生成器依赖版本，保留生成文件并禁止手工修改；Model 变更后重新生成并验证。
- 普通业务 CRUD 不使用手写 SQL 或原生 GORM 链式查询替代 Gen。版本化迁移及 Gen 无法表达的数据库专用操作可保留 SQL，并说明原因。具体实现读取 [references/mysqlutils.md](references/mysqlutils.md)。
- 仓储按业务上下文拆分实现，由 `RepositoryFactory` 汇总并提供 `WithTransaction(ctx, fn(Repositories))`；Model ↔ Domain 转换留在 Infrastructure，DTO ↔ Domain 转换留在 Application，避免跨边界直接转换或在业务流程中散落字段搬运。表结构用 `models.AllModels()` 在组合根执行 `AutoMigrate`，删除列/表等破坏性变更单独提供版本化 SQL。

## 按任务懒加载参考资料

只读取当前任务需要的文件，不要一次性加载全部 reference。

### DDD 与开发流程

- 设计目录、限界上下文、聚合、仓储、边界转换或依赖方向：读取 [references/architecture.md](references/architecture.md)。
- 新增用例、实现仓储、处理事务、接入 API/Worker 或审查现有代码：读取 [references/development-workflows.md](references/development-workflows.md)。

### 服务与接口模块

- 服务生命周期、HTTP 挂载、Worker、优雅退出：读取 [references/cores.md](references/cores.md)。
- Flag、JSON 配置、`Struct`、`Validate()`、`Reload()`、配置监听或多运行模式：读取 [references/flags.md](references/flags.md)。
- Gin Engine、路由分组、请求绑定、参数清洗/校验、统一响应：读取 [references/ginutils.md](references/ginutils.md)。
- WebSocket 命名空间、房间、事件和连接生命周期：读取 [references/websocketutils.md](references/websocketutils.md)。

### 基础设施模块

- MySQL、GORM、Model、Gen Query、连接池或读写路由：读取 [references/mysqlutils.md](references/mysqlutils.md)。
- Redis、连接池、锁、值转换或函数缓存：读取 [references/redisutils.md](references/redisutils.md)。
- 内存队列、优先队列或 Redis 队列：读取 [references/queueutils.md](references/queueutils.md)。
- SMTP 邮件发送：读取 [references/emailutils.md](references/emailutils.md)。
- 外部调用熔断、降级或错误可接受策略：读取 [references/breaker.md](references/breaker.md)。
- 本地只读数据快照和周期刷新：读取 [references/localrepos.md](references/localrepos.md)。

### 横切与通用模块

- 上下文字段、结构化日志、格式化器或 Hook：读取 [references/logging.md](references/logging.md)。
- Prometheus Handler 和内置指标：读取 [references/prometheusutils.md](references/prometheusutils.md)。
- 手机号、身份证或自定义文本脱敏：读取 [references/masking.md](references/masking.md)。
- 请求清洗、校验器和中文错误翻译：读取 [references/structutils.md](references/structutils.md)。
- 防抖执行：读取 [references/debounce.md](references/debounce.md)。
- 延迟初始化注册：读取 [references/snail.md](references/snail.md)。
- 构建版本和服务名注入：读取 [references/buildinfo.md](references/buildinfo.md)。
- 集合、切片、随机、时间、指针、反射等纯工具：读取 [references/utils.md](references/utils.md)。
