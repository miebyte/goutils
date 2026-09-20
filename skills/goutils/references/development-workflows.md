# DDD 开发与审查流程

## 新增业务能力

1. 用业务语言写清用例、参与者、输入输出和失败条件。
2. 确认所属限界上下文以及需要修改的聚合。
3. 先在 Domain 中表达不变量、状态转换和值对象。
4. 在 Domain 的 `repo.go` 定义完成用例所需的最窄仓储接口。
5. 在 Application 中编排聚合加载、领域行为、事务和技术端口。
6. 在 Infrastructure 中实现仓储、缓存、队列或外部适配器。
7. 在 API/Worker 中完成协议绑定和 Application 调用。
8. 在 `main.go` 注入具体实现。

不要从数据库表或 HTTP 路径反推领域模型；先确认业务边界。

## 修改已有服务

- 保持当前启动方式和目录，除非任务明确要求迁移。
- 先搜索现有聚合、用例、Repository 和错误映射，避免创建第二套路径。
- 新接口放在直接消费者一侧；不要为了复用把接口提前提升到公共包。
- 只删除本次修改导致的无效导入、变量或分支；原有死代码仅提醒，不顺手清理。

## Domain 实现

- 构造函数建立合法初始状态。
- 聚合方法使用业务动词，并在方法内部维护不变量。
- 值对象在创建时完成格式和范围校验。
- 跨实体但仍属同一领域规则时使用领域服务。
- 领域错误保持稳定，可由 Application/API 映射；不要直接返回 GORM、Redis、Gin 错误。
- 不把纯 Getter/Setter 集合伪装成领域模型。

## Repository 实现

接口示例：

```go
type OrderRepository interface {
    FindByID(ctx context.Context, id OrderID) (*Order, error)
    Save(ctx context.Context, order *Order) error
}
```

规则：

- 接口放 `internal/domain/order/repo.go`，实现放 `internal/infra/db`。
- 所有 I/O 方法接收 `context.Context`。
- 持久化 Model 和 Domain Aggregate 通过 DB Assembler 或集中转换包（`internal/converter`）转换。
- 明确区分“未找到”、冲突、非法状态和基础设施故障。
- 不让 Application 拼 SQL、GORM Scope 或 Redis Key。
- MySQL/GORM 仓储必须使用 `gorm.io/gen` 生成的 Query；先补齐独立表 Model 和生成入口，再实现仓储调用。Model 变更后重新生成，验证生成结果可重复，并覆盖事务、nullable、零值更新及冲突语义。
- 查询投影与聚合写仓储职责明显不同，可拆成 `OrderQueryRepository`。

## 事务

事务由 Application 用例决定，不由 Handler、聚合或单个 Repository 私自开启。

```go
type UnitOfWork interface {
    WithinTransaction(ctx context.Context, fn func(Repositories) error) error
}
```

同一事务中的 Repository 必须绑定同一事务连接。外部邮件、网络调用等不可回滚副作用默认放在提交后；需要原子可靠性时使用 Outbox，不要持有数据库事务等待慢网络。

## API 接入

- Router 只注入所需 Application 接口。
- Handler 绑定并校验请求，传递 `c.Request.Context()`，调用 Application，映射错误并返回 DTO。
- 不把 `*gin.Context` 传入 Application 或 Domain。
- 不在 Handler 直接使用 MySQL、Redis、队列或事务。
- 最终路由要同时计算 `cores.WithHttpHandler` 挂载路径、Gin Group 前缀和资源相对路径。
- 使用 `ginutils` 时读取 [ginutils.md](ginutils.md)。

## Worker 接入

- Worker 是外部触发适配器，调用 Application Service，不重新实现业务用例。
- 长期循环必须监听 `ctx.Done()` 并返回。
- 每次消息、任务或批次都传递 Context 到仓储和外部调用。
- 由 `cores.WithWorker` 或 `cores.WithNameWorker` 注册；纯 Worker 服务用 `cores.Run`。
- 重试必须有上限、退避和幂等策略，不能无限重试不可恢复错误。

## 外部系统适配

- 在 Application 定义业务语义端口，例如 `NotificationSender`，Infrastructure 用 `emailutils` 实现。
- 外部错误在 Adapter 边界转换；不要把 SMTP、Redis、GORM 或 WebSocket 实现类型传给 Domain。
- 外部调用需要熔断时在 Infrastructure Adapter 使用 `breaker`，不要让领域实体感知熔断器。
- 配置和 Client 在组合根创建并注入，不在每次用例执行时重新初始化连接池。

## 配置与依赖只校验一次

- 配置结构在 `SetDefault()` 后通过 `Validate()` 校验。
- 必选 Repository、Client、Publisher 在构造函数或组合根校验一次。
- 初始化完成后，运行期方法默认对象有效，不重复判断 receiver、配置和必选依赖是否为 `nil`。
- 可选依赖必须有显式开关、清晰的 nil 语义或 No-op 实现，不能静默跳过本应必选的能力。
- 请求字段、数据库结果、外部响应和类型断言仍需正常校验。

不要引入反射式递归 nil 检查框架来掩盖组装错误。

## Context 与日志

- API 使用请求 Context；Worker 使用 `cores` 传入的 Context。
- 不用 `context.Background()` 替换已有请求 Context，除非明确需要与请求取消解耦并记录原因。
- 日志优先使用 `logging.Infoc/Debugc/Warnc/Errorc` 或结构化 `*w/*s` 方法保留上下文字段。
- 不记录密码、Token、Secret、完整身份证号或其他敏感数据；必要时使用 `masking`。

## 测试策略

### Domain

- 正常状态转换。
- 每条业务不变量和非法转换。
- 值对象边界。
- 领域服务组合规则。

### Application

- 使用 Fake/Mock Repository 和端口验证用例顺序。
- 事务成功、回滚和提交后副作用。
- Domain/Infrastructure 错误映射。

### Infrastructure

- Model ↔ Domain 转换。
- 查询条件、未找到和唯一冲突。
- Redis 序列化、锁所有权、TTL、队列顺序。
- 外部 Adapter 的超时、取消和错误转换。

### API/Worker

- 请求绑定与校验。
- HTTP 状态和业务码映射。
- Context 传递。
- Worker 收到取消后退出。

## 完成前验证

按改动范围执行：

```bash
gofmt -w <changed-go-files>
go test ./path/to/changed/package/...
go test ./...
go vet ./...
git diff --check
```

如果默认 Go 缓存不可写，使用项目专属临时目录：

```bash
mkdir -p /tmp/miebyte-goutils-gocache /tmp/miebyte-goutils-gotmp
GOCACHE=/tmp/miebyte-goutils-gocache \
GOTMPDIR=/tmp/miebyte-goutils-gotmp \
go test ./...
```

区分“相关测试通过”和“全量测试通过”；不要用无关失败掩盖本次改动结果。
