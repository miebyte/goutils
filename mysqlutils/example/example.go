package main

import (
	"fmt"

	"github.com/miebyte/goutils/mysqlutils"
	"gorm.io/plugin/dbresolver"
)

// ExampleUser 示例用户模型
type ExampleUser struct {
	ID   uint `gorm:"primaryKey"`
	Name string
	Age  int
}

// TableName 实现 Table 接口
func (ExampleUser) TableName() string {
	return "users"
}

// App 实现 Table 接口
func (ExampleUser) App() string {
	return "user_service"
}

// ExampleOrder 示例订单模型
type ExampleOrder struct {
	ID     uint `gorm:"primaryKey"`
	UserID uint
	Amount float64
}

// TableName 实现 Table 接口
func (ExampleOrder) TableName() string {
	return "orders"
}

// App 实现 Table 接口
func (ExampleOrder) App() string {
	return "order_service"
}

// ExampleBasicUsage 基础使用示例
func ExampleBasicUsage() {
	// 数据库配置
	configs := mysqlutils.MysqlConfigMap{
		"master": &mysqlutils.MysqlConfig{
			Host:     "localhost",
			Port:     "3306",
			Db:       "app_db",
			User:     "root",
			Password: "password",
			Charset:  "utf8mb4",
			PoolSize: 10,
		},
		"slave1": &mysqlutils.MysqlConfig{
			Host:     "localhost",
			Port:     "3307",
			Db:       "app_db",
			User:     "readonly",
			Password: "password",
			Charset:  "utf8mb4",
			PoolSize: 5,
		},
		"slave2": &mysqlutils.MysqlConfig{
			Host:     "localhost",
			Port:     "3308",
			Db:       "app_db",
			User:     "readonly",
			Password: "password",
			Charset:  "utf8mb4",
			PoolSize: 5,
		},
	}

	// 创建连接池
	pool, err := configs.DialGormPool()
	if err != nil {
		fmt.Printf("创建连接池失败: %v\n", err)
		return
	}

	// 配置读写分离
	routerConfig := mysqlutils.NewDBRouterConfig("master").
		AddResolver("default", mysqlutils.NewDBResolverConfig().
			WithSources("master").
			WithReplicas("slave1", "slave2"))

	db, err := pool.GetDBWithResolver(routerConfig)
	if err != nil {
		fmt.Printf("获取数据库连接失败: %v\n", err)
		return
	}

	// 读操作（自动使用 slave）
	var users []ExampleUser
	result := db.Find(&users)
	fmt.Printf("查询用户数: %d\n", result.RowsAffected)

	// 写操作（自动使用 master）
	user := ExampleUser{Name: "张三", Age: 25}
	db.Create(&user)
	fmt.Printf("创建用户成功，ID: %d\n", user.ID)

	// 手动指定使用主库进行读取
	var userFromMaster ExampleUser
	db.Clauses(dbresolver.Write).First(&userFromMaster, user.ID)
	fmt.Printf("从主库读取用户: %s\n", userFromMaster.Name)
}

// ExampleModelBasedRouting 基于模型的路由示例
func ExampleModelBasedRouting() {
	configs := mysqlutils.MysqlConfigMap{
		"main_db":  &mysqlutils.MysqlConfig{Host: "localhost", Port: "3306", Db: "main"},
		"user_db":  &mysqlutils.MysqlConfig{Host: "localhost", Port: "3307", Db: "users"},
		"order_db": &mysqlutils.MysqlConfig{Host: "localhost", Port: "3308", Db: "orders"},
	}

	// 创建连接池
	pool, err := configs.DialGormPool()
	if err != nil {
		fmt.Printf("创建连接池失败: %v\n", err)
		return
	}

	routerConfig := mysqlutils.NewDBRouterConfig("main_db").
		AddResolver("users", mysqlutils.NewDBResolverConfig().
			WithSources("user_db").
			WithTables(ExampleUser{})).
		AddResolver("orders", mysqlutils.NewDBResolverConfig().
			WithSources("order_db").
			WithTables(ExampleOrder{}))

	db, err := pool.GetDBWithResolver(routerConfig)
	if err != nil {
		fmt.Printf("获取数据库连接失败: %v\n", err)
		return
	}

	// 使用模型操作（自动路由到对应数据库）
	var user ExampleUser
	db.First(&user, 1) // 使用 user_db

	var order ExampleOrder
	db.First(&order, 1) // 使用 order_db

	fmt.Println("基于模型的数据库路由配置成功")
}

// ExampleComplexConfiguration 复杂配置示例
func ExampleComplexConfiguration() {
	configs := mysqlutils.MysqlConfigMap{
		"user_master":  &mysqlutils.MysqlConfig{Host: "user-master", Port: "3306", Db: "users"},
		"user_slave1":  &mysqlutils.MysqlConfig{Host: "user-slave1", Port: "3306", Db: "users"},
		"user_slave2":  &mysqlutils.MysqlConfig{Host: "user-slave2", Port: "3306", Db: "users"},
		"order_master": &mysqlutils.MysqlConfig{Host: "order-master", Port: "3306", Db: "orders"},
		"order_slave1": &mysqlutils.MysqlConfig{Host: "order-slave1", Port: "3306", Db: "orders"},
		"order_slave2": &mysqlutils.MysqlConfig{Host: "order-slave2", Port: "3306", Db: "orders"},
		"log_master":   &mysqlutils.MysqlConfig{Host: "log-master", Port: "3306", Db: "logs"},
	}

	// 创建连接池
	pool, err := configs.DialGormPool()
	if err != nil {
		fmt.Printf("创建连接池失败: %v\n", err)
		return
	}

	// 复杂的读写分离配置
	routerConfig := mysqlutils.NewDBRouterConfig("user_master").
		AddResolver("user_service", mysqlutils.NewDBResolverConfig().
			WithSources("user_master").
			WithReplicas("user_slave1", "user_slave2").
			WithTables(ExampleUser{})).
		AddResolver("order_service", mysqlutils.NewDBResolverConfig().
			WithSources("order_master").
			WithReplicas("order_slave1", "order_slave2").
			WithTables(ExampleOrder{})).
		AddResolver("log_service", mysqlutils.NewDBResolverConfig().
			WithSources("log_master"))

	db, err := pool.GetDBWithResolver(routerConfig)
	if err != nil {
		fmt.Printf("获取数据库连接失败: %v\n", err)
		return
	}

	// 指定使用特定服务的数据库
	var users []ExampleUser
	db.Clauses(dbresolver.Use("user_service")).Find(&users) // 使用 user_service 的读库

	var orders []ExampleOrder
	db.Clauses(dbresolver.Use("order_service"), dbresolver.Write).Find(&orders) // 使用 order_service 的写库

	// 使用事务
	tx := db.Clauses(dbresolver.Use("user_service"), dbresolver.Write).Begin()
	tx.Create(&ExampleUser{Name: "事务用户", Age: 30})
	tx.Commit()

	fmt.Println("复杂数据库路由配置完成")
}

// ExampleTransactionUsage 事务使用示例
func ExampleTransactionUsage() {
	configs := mysqlutils.MysqlConfigMap{
		"master": &mysqlutils.MysqlConfig{Host: "localhost", Port: "3306", Db: "app"},
		"slave":  &mysqlutils.MysqlConfig{Host: "localhost", Port: "3307", Db: "app"},
	}

	// 创建连接池
	pool, err := configs.DialGormPool()
	if err != nil {
		fmt.Printf("创建连接池失败: %v\n", err)
		return
	}

	routerConfig := mysqlutils.NewDBRouterConfig("master").
		AddResolver("default", mysqlutils.NewDBResolverConfig().
			WithSources("master").
			WithReplicas("slave"))

	db, err := pool.GetDBWithResolver(routerConfig)
	if err != nil {
		fmt.Printf("获取数据库连接失败: %v\n", err)
		return
	}

	// 开始事务（指定使用主库）
	tx := db.Clauses(dbresolver.Write).Begin()

	// 在事务中执行多个操作
	user := ExampleUser{Name: "事务用户", Age: 25}
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		fmt.Printf("创建用户失败: %v\n", err)
		return
	}

	order := ExampleOrder{UserID: user.ID, Amount: 100.50}
	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		fmt.Printf("创建订单失败: %v\n", err)
		return
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		fmt.Printf("提交事务失败: %v\n", err)
		return
	}

	fmt.Printf("事务执行成功，用户ID: %d，订单ID: %d\n", user.ID, order.ID)
}

// ExampleTableInterface 使用 Table 接口的示例
func ExampleTableInterface() {
	configs := mysqlutils.MysqlConfigMap{
		"user_master":  &mysqlutils.MysqlConfig{Host: "user-master", Port: "3306", Db: "users"},
		"user_slave":   &mysqlutils.MysqlConfig{Host: "user-slave", Port: "3306", Db: "users"},
		"order_master": &mysqlutils.MysqlConfig{Host: "order-master", Port: "3306", Db: "orders"},
		"order_slave":  &mysqlutils.MysqlConfig{Host: "order-slave", Port: "3306", Db: "orders"},
	}

	// 创建连接池
	pool, err := configs.DialGormPool()
	if err != nil {
		fmt.Printf("创建连接池失败: %v\n", err)
		return
	}

	// 使用 Table 接口配置路由
	userTable := ExampleUser{}
	orderTable := ExampleOrder{}

	routerConfig := mysqlutils.NewDBRouterConfig("user_master").
		AddResolver("user_service", mysqlutils.NewDBResolverConfig().
			WithSources("user_master").
			WithReplicas("user_slave").
			WithTables(userTable)).
		AddResolver("order_service", mysqlutils.NewDBResolverConfig().
			WithSources("order_master").
			WithReplicas("order_slave").
			WithTables(orderTable))

	db, err := pool.GetDBWithResolver(routerConfig)
	if err != nil {
		fmt.Printf("获取数据库连接失败: %v\n", err)
		return
	}

	// 现在可以使用数据库操作
	var users []ExampleUser
	db.Find(&users) // 自动路由到 user_service

	var orders []ExampleOrder
	db.Find(&orders) // 自动路由到 order_service

	fmt.Printf("使用 Table 接口配置路由成功，查询到 %d 个用户，%d 个订单\n", len(users), len(orders))
}

// ExampleSimpleTableRouting 简单表级路由示例
func ExampleSimpleTableRouting() {
	configs := mysqlutils.MysqlConfigMap{
		"db1": &mysqlutils.MysqlConfig{
			Host:     "localhost",
			Port:     "3307",
			Db:       "db1",
			User:     "root",
			Password: "123456789",
			Charset:  "utf8mb4",
			PoolSize: 10,
		},
		"db2": &mysqlutils.MysqlConfig{
			Host:     "localhost",
			Port:     "3308",
			Db:       "db2",
			User:     "root",
			Password: "123456789",
			Charset:  "utf8mb4",
			PoolSize: 10,
		},
		"db3": &mysqlutils.MysqlConfig{
			Host:     "localhost",
			Port:     "3309",
			Db:       "db3",
			User:     "root",
			Password: "123456789",
			Charset:  "utf8mb4",
			PoolSize: 10,
		},
	}

	pool, err := configs.DialGormPool()
	if err != nil {
		fmt.Printf("创建连接池失败: %v\n", err)
		return
	}

	// 配置表级路由：
	// ExampleUser 表使用 db1 (端口3307)
	// ExampleOrder 表使用 db2 (端口3308)
	// 其他表使用默认的 db3 (端口3309)
	routerConfig := mysqlutils.NewDBRouterConfig("db3").
		AddResolver("users", mysqlutils.NewDBResolverConfig().
			WithSources("db1").
			WithTables(ExampleUser{})).
		AddResolver("orders", mysqlutils.NewDBResolverConfig().
			WithSources("db2"))
		// WithTables(ExampleOrder{}))

	db, err := pool.GetDBWithResolver(routerConfig)
	if err != nil {
		fmt.Printf("获取数据库连接失败: %v\n", err)
		return
	}

	// 操作 users 表 - 自动路由到 db1 (端口3307)
	user := ExampleUser{Name: "张三", Age: 25}
	if err := db.Create(&user).Error; err != nil {
		fmt.Printf("创建用户失败: %v\n", err)
	} else {
		fmt.Printf("在 db1 (端口3307) 创建用户成功，ID: %d\n", user.ID)
	}

	var users []ExampleUser
	db.Find(&users)
	fmt.Printf("从 db1 (端口3307) 查询到 %d 个用户\n", len(users))

	// 操作 orders 表 - 自动路由到 db2 (端口3308)
	order := ExampleOrder{UserID: user.ID, Amount: 199.99}
	if err := db.Create(&order).Error; err != nil {
		fmt.Printf("创建订单失败: %v\n", err)
	} else {
		fmt.Printf("在 db2 (端口3308) 创建订单成功，ID: %d\n", order.ID)
	}

	var orders []ExampleOrder
	db.Find(&orders)
	fmt.Printf("从 db2 (端口3308) 查询到 %d 个订单\n", len(orders))

	// 操作其他表 - 使用默认的 db3 (端口3309)
	result := db.Exec("CREATE TABLE IF NOT EXISTS products (id INT PRIMARY KEY, name VARCHAR(255))")
	if result.Error != nil {
		fmt.Printf("在 db3 创建表失败: %v\n", result.Error)
	} else {
		fmt.Println("在 db3 (端口3309) 创建 products 表成功")
	}

	// 查找不存在的数据
	notExistsUser := &ExampleUser{}
	if err := db.Clauses(dbresolver.Use("orders")).Where("id = ?", 1).First(notExistsUser).Error; err != nil {
		fmt.Printf("从 db1 (端口3307)查询不存在的用户. %v\n", err)
	}

	fmt.Println("表级路由配置完成！")
	fmt.Println("- users 表 → db1 (端口3307)")
	fmt.Println("- orders 表 → db2 (端口3308)")
	fmt.Println("- 其他表 → db3 (端口3309)")
}
