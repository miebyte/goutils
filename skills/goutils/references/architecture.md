# DDD 项目架构规范

## 总体目录

```text
.
├── main.go                         # 组合根：配置、基础设施、依赖注入、启动
├── api/                            # Interface Layer
│   ├── api.go                      # HTTP/WebSocket 装配
│   ├── common/                     # 协议错误与响应映射
│   ├── middleware/                 # 鉴权、限流、上下文注入
│   └── router/                     # 按资源或访问边界组织路由
├── config/                         # 启动与基础设施配置
├── internal/
│   ├── app/                        # Application Layer
│   │   ├── dto/                    # 用例输入与输出
│   │   ├── assembler/              # Application DTO ↔ Domain
│   │   ├── services/               # Application Service
│   │   └── ports/                  # 可选：事务、事件、消息、文件等技术端口
│   ├── domain/                     # Domain Layer
│   │   └── <bounded-context>/
│   │       ├── entity.go           # 聚合根与实体
│   │       ├── valueobj.go         # 值对象与领域枚举
│   │       ├── repo.go             # 领域仓储接口
│   │       ├── service.go          # 领域服务
│   │       ├── event.go            # 可选：领域事件
│   │       └── errors.go           # 可选：领域错误
│   ├── infra/                      # Infrastructure Layer
│   │   ├── db/
│   │   │   ├── models/             # 持久化模型
│   │   │   ├── assembler/          # Persistence Model ↔ Domain
│   │   │   ├── query/              # 可选：生成查询代码
│   │   │   ├── transaction.go      # 事务实现
│   │   │   └── <context>.go        # 仓储实现
│   │   ├── cache/                  # Redis 缓存、锁、仓储
│   │   ├── queue/                  # 队列适配器
│   │   └── external/               # 邮件、第三方 API 等适配器
│   └── errcode/                    # 稳定的应用错误或协议错误映射
└── cmd/                             # 仅放真正独立的运行模式或生成入口
```

目录是职责边界，不是必须创建的空壳。没有跨资源事务时不建 `ports/`；转换很少时可将无状态函数放在对应 Application 或 Infrastructure 包；只有单一运行模式时直接在 `main.go` 组装。

## 分层职责

### API / Interface

负责把外部协议转换成应用用例调用：

- 注册路由、中间件、HTTP 方法或 WebSocket 事件。
- 绑定 Query、Path、Header、Body，提取调用身份。
- 调用 Application Service。
- 把应用结果和错误转换成协议响应。

不得直接查询数据库、Redis、队列，不承载业务不变量或事务。

### Application

表达“创建订单”“发送通知”“撤销操作”等用例：

- 编排执行顺序和跨领域流程。
- 获取聚合并调用领域行为。
- 定义事务边界。
- 调用技术端口和发布事件。
- 将 Domain 结果转换为 DTO。

Application 可以依赖 Domain 和自己定义的端口，不能依赖具体 GORM Repository、Redis Client、Gin Context 或 SMTP Client。

### Domain

承载核心业务模型：

- 聚合根、实体、值对象。
- 业务不变量和状态转换。
- 领域服务、领域事件、领域错误。
- 表达业务需要的仓储接口。

Domain 默认只依赖标准库和稳定的业务语义。不要导入 `ginutils`、`mysqlutils`、`redisutils`、`emailutils`、`queueutils` 或 GORM。

### Infrastructure

实现外部技术细节：

- GORM Model、SQL、仓储和事务。
- Redis 缓存、锁和队列。
- 邮件、外部 HTTP/RPC、WebSocket 底层适配。
- 指标、日志输出和第三方 SDK。

基础设施错误应在边界转换成 Application 或 Domain 可理解的错误，不要让 `gorm.ErrRecordNotFound`、Redis 命令错误等实现细节扩散到 API。

## 接口所有权

接口由最窄的直接消费者定义：

- Domain 需要持久化聚合：接口放 `internal/domain/<context>/repo.go`。
- Application 需要事务、消息、文件、时钟：接口放使用它的 Application 包；仅多个包共享且语义稳定时放 `internal/app/ports`。
- API 只需要某个用例：可在 Router 附近定义窄化 Application 接口。

仓储命名表达领域语义，例如 `OrderRepository`、`OrderQueryRepository`，禁止无上下文的裸 `Repository`。方法表达业务需要，例如 `FindPendingByCustomer`，不要机械暴露 GORM CRUD。

## 聚合与仓储

- 一个事务默认修改一个聚合；跨聚合一致性优先通过 Application 编排、事件或补偿实现。
- 仓储以聚合为读写单位，不允许外层绕过聚合直接修改其内部实体。
- 读模型可以定义独立 Query Repository，不必强行还原完整聚合。
- Repository 接口不暴露 `*gorm.DB`、SQL 条件或生成 Query 对象。

## 三类模型分离

- Persistence Model：匹配表结构、索引、nullable 和 GORM 标签。
- Domain Model：表达业务状态、行为和不变量。
- Application DTO：表达用例输入输出和序列化契约。

字段相似不代表可以复用同一个结构体。数据库模型变化不应直接改变 API；API 字段变化也不应迫使 Domain 引入协议标签。

## Assembler 方向

转换逻辑归属于跨越边界的外层：

- `internal/app/assembler`：DTO ↔ Domain，只依赖 Application 与 Domain。
- `internal/infra/db/assembler`：Persistence Model ↔ Domain，只依赖 Infrastructure 与 Domain。
- API 专属展示转换可以放 API 层。

不要创建同时导入 DTO、Domain 和 Persistence Model 的全局万能转换包。

## 事务边界

Application 定义事务边界，Infrastructure 提供实现：

1. Application 开始事务。
2. 事务内取得绑定同一事务连接的 Repository。
3. 加载聚合、执行领域行为、保存聚合。
4. 提交后执行非事务副作用，或使用 Outbox 保证可靠发布。

Domain 不接收 `*gorm.DB`，API 不开启事务，Repository 不擅自决定跨用例事务范围。

## goutils 模块在分层中的位置

| 模块 | 默认位置 | 说明 |
|---|---|---|
| `cores`、`flags`、`buildinfo` | `main.go` / bootstrap | 进程启动、配置、服务标识 |
| `ginutils` | API | HTTP 协议适配 |
| `websocketutils` | API 或 Infrastructure adapter | 事件协议和连接管理 |
| `mysqlutils`、`redisutils` | Infrastructure | 数据库、缓存、锁 |
| `queueutils`、`emailutils`、`breaker` | Infrastructure | 队列、邮件和外部调用保护 |
| `localrepos` | Infrastructure | 本地只读快照缓存 |
| `logging`、`prometheusutils`、`masking` | 横切能力 | 在外层使用，不污染 Domain |
| `structutils` | API / config boundary | 输入清洗与校验 |
| `utils`、`debounce` | 按职责使用 | 仅在不会引入技术耦合时进入内层 |
| `snail` | bootstrap | 延迟初始化机制，业务代码通常不直接使用 |

## 简化原则

- 小服务可以减少目录，但不能反转依赖方向。
- 只有一个实现且没有替换、测试或边界价值时，不创建形式化接口。
- 不为一次性需求引入总线、插件系统、全局 Service Locator 或反射式依赖注入。
- 保持已有项目结构；除非用户明确要求，不把正常运行的服务整体迁移为新目录。
