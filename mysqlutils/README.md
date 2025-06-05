# MySQL工具包

这个包提供了MySQL数据库连接的工具函数和结构体，基于GORM库实现，支持单数据库连接和多数据库连接池管理。

## 文件结构

- `mysql.go` - 主要的MySQL连接和池管理功能
- `dbrouter.go` - 数据库路由配置相关的结构体和方法
- `table.go` - 表操作相关工具
- `example.go` - 使用示例代码
- `README.md` - 说明文档

## 功能特性

- 简化MySQL数据库连接配置
- 支持基于GORM的数据库操作
- 提供连接池管理
- 自动集成链路跟踪功能
- 连接参数优化（连接池大小、最大连接数、连接生命周期等）
- 支持读写分离
- 支持表级别的数据库路由

## 使用方法

### 单数据库连接

```go
import (
    "github.com/miebyte/goutils/mysqlutils"
)

func main() {
    // 创建数据库配置
    conf := &mysqlutils.MysqlConfig{
        Host:     "localhost",
        Port:     "3306",
        Db:       "your_database",
        User:     "username",
        Password: "password",
        Charset:  "utf8mb4",
        PoolSize: 10,
    }
    
    // 连接数据库
    db, err := mysqlutils.DialMysqlGorm(conf)
    if err != nil {
        panic(err)
    }
    
    // 使用db进行GORM操作...
}
```

### 多数据库连接池

```go
import (
    "github.com/miebyte/goutils/mysqlutils"
)

func main() {
    // 创建多个数据库配置
    configs := mysqlutils.MysqlConfigMap{
        "default": &mysqlutils.MysqlConfig{
            Host:     "localhost",
            Port:     "3306",
            Db:       "default_db",
            User:     "username",
            Password: "password",
            PoolSize: 10,
        },
        "user_db": &mysqlutils.MysqlConfig{
            Host:     "localhost", 
            Port:     "3306",
            Db:       "user_db",
            User:     "username",
            Password: "password",
            PoolSize: 5,
        },
    }
    
    // 创建连接池
    pool, err := configs.DialGormPool()
    if err != nil {
        panic(err)
    }
    
    // 获取特定数据库连接
    defaultDB, err := pool.GetMysql() // 获取默认连接
    if err != nil {
        panic(err)
    }
    
    userDB, err := pool.GetMysql("user_db") // 获取特定名称的连接
    if err != nil {
        panic(err)
    }
    
    // 使用各个数据库连接进行操作...
}
```

## 读写分离配置

### 基础读写分离

```go
// 数据库连接配置
configs := mysqlutils.MysqlConfigMap{
    "master": &mysqlutils.MysqlConfig{
        Host: "master-host",
        Port: "3306",
        Db:   "app_db",
        User: "root",
        Password: "password",
    },
    "slave1": &mysqlutils.MysqlConfig{
        Host: "slave1-host",
        Port: "3306",
        Db:   "app_db",
        User: "readonly",
        Password: "password",
    },
    "slave2": &mysqlutils.MysqlConfig{
        Host: "slave2-host", 
        Port: "3306",
        Db:   "app_db",
        User: "readonly",
        Password: "password",
    },
}

// 创建连接池
pool, err := configs.DialGormPool()
if err != nil {
    panic(err)
}

// 路由配置
routerConfig := mysqlutils.NewDBRouterConfig("master").
    AddResolver("default", mysqlutils.NewDBResolverConfig().
        WithSources("master").
        WithReplicas("slave1", "slave2"))

db, err := pool.GetDBWithResolver(routerConfig)
if err != nil {
    panic(err)
}

// 读操作自动使用 slave
var users []User
db.Find(&users) // 使用 slave1 或 slave2

// 写操作自动使用 master
db.Create(&User{Name: "张三"}) // 使用 master
```

### 表级别的数据库路由

```go
configs := mysqlutils.MysqlConfigMap{
    "main_db": &mysqlutils.MysqlConfig{Host: "main-host", Port: "3306", Db: "main"},
    "user_db": &mysqlutils.MysqlConfig{Host: "user-host", Port: "3306", Db: "users"},
    "order_db": &mysqlutils.MysqlConfig{Host: "order-host", Port: "3306", Db: "orders"},
    "log_db": &mysqlutils.MysqlConfig{Host: "log-host", Port: "3306", Db: "logs"},
}

// 创建连接池
pool, err := configs.DialGormPool()
if err != nil {
    panic(err)
}

// 定义模型，实现 Table 接口
type User struct {
    ID   uint   `gorm:"primaryKey"`
    Name string
}

func (User) TableName() string {
    return "users"
}

func (User) App() string {
    return "user_service"
}

type Order struct {
    ID     uint `gorm:"primaryKey"`
    UserID uint
    Amount float64
}

func (Order) TableName() string {
    return "orders"
}

func (Order) App() string {
    return "order_service"
}

routerConfig := mysqlutils.NewDBRouterConfig("main_db").
    AddResolver("users", mysqlutils.NewDBResolverConfig().
        WithSources("user_db").
        WithTables(User{})).
    AddResolver("orders", mysqlutils.NewDBResolverConfig().
        WithSources("order_db").
        WithTables(Order{}))

db, err := pool.GetDBWithResolver(routerConfig)

// 不同表使用不同数据库
var users []User
db.Find(&users)           // 使用 user_db

var orders []Order  
db.Find(&orders)         // 使用 order_db  

// 使用原始 SQL 查询其他表（使用默认 main_db）
db.Exec("SELECT * FROM products")
```

### 基于模型的路由配置

```go
// 定义模型，实现 Table 接口
type User struct {
    ID   uint   `gorm:"primaryKey"`
    Name string
}

func (User) TableName() string {
    return "users"
}

func (User) App() string {
    return "user_service"
}

type Order struct {
    ID     uint `gorm:"primaryKey"`
    UserID uint
    Amount float64
}

func (Order) TableName() string {
    return "orders"
}

func (Order) App() string {
    return "order_service"
}

// 创建连接池
pool, err := configs.DialGormPool()
if err != nil {
    panic(err)
}

// 使用 Table 接口配置路由
routerConfig := mysqlutils.NewDBRouterConfig("main_db").
    AddResolver("users", mysqlutils.NewDBResolverConfig().
        WithSources("user_db").
        WithTables(User{})).
    AddResolver("orders", mysqlutils.NewDBResolverConfig().
        WithSources("order_db").
        WithTables(Order{}))

db, err := pool.GetDBWithResolver(routerConfig)

// 模型操作自动路由到对应数据库
db.Find(&User{})   // 使用 user_db
db.Find(&Order{})  // 使用 order_db
```

### 复杂读写分离配置

```go
// 定义模型
type User struct {
    ID   uint   `gorm:"primaryKey"`
    Name string
}

func (User) TableName() string {
    return "users"
}

func (User) App() string {
    return "user_service"
}

type Order struct {
    ID     uint `gorm:"primaryKey"`
    UserID uint
    Amount float64
}

func (Order) TableName() string {
    return "orders"
}

func (Order) App() string {
    return "order_service"
}

// 创建连接池
pool, err := configs.DialGormPool()
if err != nil {
    panic(err)
}

routerConfig := mysqlutils.NewDBRouterConfig("master").
    AddResolver("user_service", mysqlutils.NewDBResolverConfig().
        WithSources("user_master").
        WithReplicas("user_slave1", "user_slave2").
        WithTables(User{})).
    AddResolver("order_service", mysqlutils.NewDBResolverConfig().
        WithSources("order_master1", "order_master2").
        WithReplicas("order_slave1", "order_slave2", "order_slave3").
        WithTables(Order{})).
    AddResolver("log_service", mysqlutils.NewDBResolverConfig().
        WithSources("log_master"))

db, err := pool.GetDBWithResolver(routerConfig)

// 手动指定读写模式
import "gorm.io/plugin/dbresolver"

// 强制使用写库
db.Clauses(dbresolver.Write).First(&user)

// 强制使用读库  
db.Clauses(dbresolver.Read).Find(&users)

// 指定特定服务的数据库
db.Clauses(dbresolver.Use("user_service")).Find(&users)
db.Clauses(dbresolver.Use("order_service"), dbresolver.Write).Create(&order)
```

## 事务处理

```go
// 开始事务时指定使用主库
tx := db.Clauses(dbresolver.Write).Begin()

// 在事务中的所有操作都会使用同一个连接
tx.Create(&user)
tx.Create(&order)

tx.Commit()
```

## 注意事项

1. 读写分离基于 SQL 语句类型自动判断
2. 事务中的所有操作会使用同一个数据库连接
3. 可以通过 `dbresolver.Write` 和 `dbresolver.Read` 手动指定读写模式
4. 表名路由优先级高于全局路由配置

## 配置参数说明

`MysqlConfig` 结构体支持以下配置参数：

| 参数 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| Host | string | 数据库主机地址 | - |
| Port | string | 数据库端口号 | - |
| Db | string | 数据库名称 | - |
| User | string | 用户名 | - |
| Password | string | 密码 | - |
| Charset | string | 字符集 | utf8mb4 |
| PoolSize | int | 连接池大小（空闲连接数） | - |

## 内部实现

- 自动设置最大开放连接数为100
- 连接最大生命周期为1小时
- 自动集成链路跟踪功能
