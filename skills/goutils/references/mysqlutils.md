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

## Repository 边界

```go
type orderRepository struct {
    db *gorm.DB
}

func (r *orderRepository) FindByID(ctx context.Context, id order.ID) (*order.Order, error) {
    var model OrderModel
    err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error
    // 将未找到和基础设施错误映射后，再转换为 Domain
    return toDomain(model), err
}
```

- 所有查询使用 `WithContext(ctx)`。
- Model ↔ Domain 转换放 Infrastructure Assembler。
- 不把 `*gorm.DB`、GORM Model 或 SQL 条件暴露给 Application/Domain。
- 事务内的 Repository 必须绑定同一个事务 `*gorm.DB`。

## 读写路由

使用 `MysqlPool.GetDBWithResolver` 前，确保：

- `DefaultDB` 存在。
- `Sources`、`Replicas` 中的连接名都已初始化。
- `Tables` 实现 GORM `schema.Tabler`。

只有确有多库或读写分离需求时启用 dbresolver；单库服务直接注入单一 `*gorm.DB` 更简单。

当前 `MysqlPool.Close()` 是空实现，不要假设它会释放连接；需要显式关闭时，在 bootstrap 管理各 `*gorm.DB` 对应的 `sql.DB` 生命周期。
