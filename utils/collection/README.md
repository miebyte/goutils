# Collection Package

这个包提供了 Go 语言的通用集合数据结构。

## Set 集合

`Set` 是一个类型安全的泛型集合实现，基于 Go 的 map 类型构建。

### 特性

- **类型安全**: 使用 Go 泛型确保类型安全
- **高效操作**: 基于 map 实现，O(1) 时间复杂度
- **简洁 API**: 提供直观的集合操作方法
- **内存优化**: 使用占位符类型节省内存

### 主要方法

- `NewSet[T comparable]() *Set[T]` - 创建新的空集合
- `Add(items ...T)` - 添加一个或多个元素
- `Remove(item T)` - 移除指定元素
- `Contains(item T) bool` - 检查元素是否存在
- `Count() int` - 获取集合大小
- `Keys() []T` - 获取所有元素的切片
- `Clear()` - 清空集合

### 使用示例

```go
package main

import (
    "fmt"
    "github.com/miebyte/goutils/utils/collection"
)

func main() {
    // 创建字符串集合
    set := collection.NewSet[string]()

    // 添加元素
    set.Add("apple", "banana", "cherry")

    // 检查元素
    fmt.Println(set.Contains("apple"))  // true
    fmt.Println(set.Contains("grape"))  // false

    // 获取大小
    fmt.Println(set.Count())  // 3

    // 获取所有元素
    keys := set.Keys()
    fmt.Println(keys)  // [apple banana cherry]

    // 移除元素
    set.Remove("banana")
    fmt.Println(set.Count())  // 2

    // 清空集合
    set.Clear()
    fmt.Println(set.Count())  // 0
}
```

### 注意事项

- 集合不是线程安全的，在并发环境中使用时需要适当的同步机制
- 元素类型必须实现 `comparable` 接口
- 集合内部使用 map 实现，元素顺序不保证
*/
