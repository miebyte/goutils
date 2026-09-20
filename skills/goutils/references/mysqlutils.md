# mysqlutils 模块

`mysqlutils` 基于 GORM 创建 MySQL 连接、连接池和 dbresolver 路由。它属于 Infrastructure；Domain 和 API 不直接依赖它。

## 当前配置结构

```go
type MysqlConfig struct {
    Instance string `json:"instance"` // host:port
    Database string `json:"database"`
    Username string `json:"username"`
    Password string `json:"password"`
    Charset  string `json:"charset"`
    PoolSize int    `json:"pool_size"`
}
```

不要使用旧文档中的 `Host`、`Port`、`Db`、`User` 字段。编码前仍应检查目标项目锁定版本。

```go
var mysqlFlag = flags.Struct(
    "mysql",
    mysqlutils.MysqlConfigMap(nil),
    "mysql configs",
)

configs := make(mysqlutils.MysqlConfigMap)
logging.PanicError(mysqlFlag(&configs))
pool, err := configs.DialGormPool()
logging.PanicError(err)
```

默认连接名是 `default`。连接池在组合根创建并注入 Repository，不在 Handler 或每次用例中重复拨号。

## GORM 配置

- `DialMysqlGorm` 默认开启 `PrepareStmt` 并使用包内 `GormLogger`。
- `DialMysqlGormWithConfig` 会复制传入的 `gorm.Config`，未设置 Logger 时补上默认 Logger。
- 默认最大空闲和最大连接数均使用 `PoolSize`，连接最大生命周期为 1 小时。
- `EnableDebug()` 会输出全部 SQL；只在受控调试环境使用，避免泄漏敏感参数。

当前模块没有通用 `BaseModel`。Persistence Model 必须按真实 Schema 显式定义主键、时间、nullable、索引和列名，不能从另一个 go-utils 版本复制不存在的类型。

## 必须使用 gorm/gen

MySQL/GORM 持久化必须提供独立 Model、`gorm.io/gen` 生成的 `query` 和实际调用 Query 的 Repository。只通过 goutils 获取 `*sql.DB` 后手写业务 SQL 不满足本规范。

- Model 显式声明表名、列名、类型、主键、nullable 和索引；与 Domain、Application DTO 分离。
- 生成入口使用 `gen.NewGenerator`、`ApplyBasic` 和 `Execute`；固定项目内 `gorm.io/gen` 版本。已有 Go Model 时直接从 Model 生成，无需为了生成 Query 连接业务数据库。
- 提供 `go generate` 或 `make generate` 等可重复命令，生成的 `query/*.gen.go` 随源码入库，禁止手工修改；Model 变更后必须重新生成。
- 通过 `query.Use(db)` 注入实例，不依赖全局 Query；事务使用绑定该事务连接的 Query，不能回到非事务实例执行。
- 普通业务 CRUD、条件和排序使用生成的字段与方法，不用原生 GORM 或手写 SQL 绕过。版本化 DDL 迁移、迁移锁等数据库专用操作可保留 SQL 并注明原因；Gen 无法表达的特殊查询须局部说明原因。
- 注意 `Updates(struct)` 跳过零值的行为，明确保存 `false`、`0`、空字符串和 `NULL`；保留行锁、事务隔离、唯一冲突及受影响行数语义。

## Repository 边界

```go
type orderRepository struct {
    q *query.Query
}

func (r *orderRepository) FindByID(ctx context.Context, id order.ID) (*order.Order, error) {
    o := r.q.Order
    model, err := o.WithContext(ctx).Where(o.ID.Eq(string(id))).Take()
    // 将未找到和基础设施错误映射后，再转换为 Domain
    if err != nil {
        return nil, mapDBError(err)
    }
    return toDomain(model), nil
}
```

- 所有查询使用 `WithContext(ctx)`。
- Model ↔ Domain 转换放 Infrastructure Assembler 或集中转换包（`internal/converter`，见 architecture.md 的「Assembler 方向」）。

## 仓储工厂与 AutoMigrate

单库服务推荐用一个 `RepositoryFactory` 汇总仓储，并把事务边界暴露成参数：

```go
type RepositoryFactory struct{ db *gorm.DB }

func NewRepositoryFactory(db *gorm.DB) *RepositoryFactory { return &RepositoryFactory{db: db} }

func (f *RepositoryFactory) User() identity.IUserRepository { return &userRepository{db: f.db} }

func (f *RepositoryFactory) WithTransaction(ctx context.Context, fn func(Repositories) error) error {
    return f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
        return fn(&RepositoryFactory{db: tx})
    }, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
}
```

- 应用层依赖汇总接口（`Repositories`），事务内得到的工厂绑定同一连接，仓储不会各自开事务。
- 需要行锁/隔离级别时在 `WithTransaction` 内声明，不在仓储里偷偷切换。
- 表结构用 `models.AllModels()` 在组合根执行 `AutoMigrate`：Model 即 Schema 的唯一来源；`AutoMigrate` 只增不删，删除列/表等破坏性变更必须单独提供版本化 SQL。
- 需要外键时在 Model 上加关联字段并在 `gorm.Config` 里决定是否 `DisableForeignKeyConstraintWhenMigrating`；不使用外键时由应用层保证引用完整性，并在文档中写明。
- 不把 `*gorm.DB`、GORM Model 或 SQL 条件暴露给 Application/Domain。
- 事务内的 Repository 必须使用同一事务 Query，例如 `q.Transaction(func(tx *query.Query) error { ... })`，或从同一个事务 `*gorm.DB` 创建 `query.Use(tx)`。

## 读写路由

使用 `MysqlPool.GetDBWithResolver` 前，确保：

- `DefaultDB` 存在。
- `Sources`、`Replicas` 中的连接名都已初始化。
- `Tables` 实现 GORM `schema.Tabler`。

只有确有多库或读写分离需求时启用 dbresolver；单库服务直接注入单一 `*gorm.DB` 更简单。

当前 `MysqlPool.Close()` 是空实现，不要假设它会释放连接；需要显式关闭时，在 bootstrap 管理各 `*gorm.DB` 对应的 `sql.DB` 生命周期。
