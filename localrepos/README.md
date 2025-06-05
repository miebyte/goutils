# LocalRepos

LocalRepos 是一个用于本地缓存数据的通用工具包，它基于 Go 泛型实现，能够自动定期刷新缓存数据。

## 功能特点

- 基于 Go 泛型实现，支持任意实现了 `HashData` 接口的数据类型
- 提供内存缓存，减少对数据源的频繁访问
- 支持自动定期刷新数据
- 线程安全，支持并发读写操作
- 简洁易用的 API 设计

## 使用方法

### 基本用法

1. 首先，确保你的数据类型实现了 `HashData` 接口：

```go
// 实现 HashData 接口
type MyData struct {
    ID    string
    Value string
}

// 实现 GetKey 方法
func (d MyData) GetKey() string {
    return d.ID
}
```

2. 创建一个数据源函数，该函数应返回 `iter.Seq[T]` 和 error：

```go
func myDataStore(ctx context.Context) (iter.Seq[MyData], error) {
    // 从数据库、API 或其他数据源获取数据
    // 返回数据序列
    return slices.Values([]MyData{
        {ID: "1", Value: "value1"},
        {ID: "2", Value: "value2"},
    }), nil
}
```

3. 创建并启动 LocalRepos 实例：

```go
func main() {
    ctx := context.Background()
    
    // 创建 LocalRepos 实例
    repos := localrepos.NewLocalRepos(myDataStore, 
        localrepos.WithRefreshInterval(5 * time.Minute))
    
    // 启动定期刷新
    repos.Start(ctx)
    defer repos.Close()
    
    // 使用 repos 获取数据
    item := repos.Get("1")
    allItems := repos.AllValues()
    
    // 更多操作...
}
```

### 配置选项

- `WithRefreshInterval` - 设置缓存刷新间隔时间（默认为 5 分钟）：

```go
repos := localrepos.NewLocalRepos(myDataStore, 
    localrepos.WithRefreshInterval(10 * time.Second))
```

## API 参考

### 类型

- `HashData` - 数据必须实现的接口
- `DataStore[T HashData]` - 数据源函数类型
- `ReposKVEntry[T HashData]` - 键值对结构
- `LocalRepos[T HashData]` - 主要的仓库类型

### 方法

- `NewLocalRepos[T HashData](dataStore DataStore[T], opts ...ReposOptionsFunc) *LocalRepos[T]` - 创建新的本地仓库
- `Start(ctx context.Context)` - 启动定期刷新
- `Get(id string) T` - 根据键获取值
- `AllValues() []T` - 获取所有值
- `AllKeys() []string` - 获取所有键
- `AllItems() []*ReposKVEntry[T]` - 获取所有键值对
- `Len() int` - 获取条目数量
- `Close() error` - 停止定期刷新

## 示例

完整示例代码：

```go
package main

import (
    "context"
    "fmt"
    "iter"
    "slices"
    "time"
    
    "github.com/miebyte/goutils/localrepos"
)

// 用户数据
type User struct {
    ID   string
    Name string
}

// 实现 HashData 接口
func (u User) GetKey() string {
    return u.ID
}

// 模拟数据源
func userDataStore(ctx context.Context) (iter.Seq[User], error) {
    users := []User{
        {ID: "1", Name: "张三"},
        {ID: "2", Name: "李四"},
        {ID: "3", Name: "王五"},
    }
    return slices.Values(users), nil
}

func main() {
    ctx := context.Background()
    
    // 创建仓库并设置1分钟刷新
    repos := localrepos.NewLocalRepos(userDataStore, 
        localrepos.WithRefreshInterval(1 * time.Minute))
    
    // 启动
    repos.Start(ctx)
    defer repos.Close()
    
    // 获取单个用户
    user := repos.Get("1")
    fmt.Printf("用户: %s\n", user.Name)
    
    // 获取所有用户
    allUsers := repos.AllValues()
    fmt.Printf("总用户数: %d\n", len(allUsers))
    
    // 等待程序结束
    time.Sleep(time.Minute)
}
```