# utils 模块

`utils` 及其子包提供纯函数、泛型集合、时间、随机、指针、反射、滑动窗口和并发容器。先使用标准库；只有现有 helper 明显更简洁并且语义匹配时才引入。

## 常用能力

- `Map`、`Reduce`、`Filter`、`Any`、`All`、`Find`、`GroupBy`、`Partition`。
- `MapIter`、`FilterIter`、`ZipIter` 等惰性迭代器。
- `AsSlice` 的链式 `Map/Reduce/GroupBy/Zip`；转换不会复制底层数组。
- `Dedup`、`Contains`、`Reverse`、`MapKeys`、`MapValues`。
- `BeginOf*/EndOf*`、`StartEnd*`、`AtomicDuration`。
- `Md5`、`ShortMd5`、`AppendAny`。
- `utils/collection`、`utils/ptrx`、`utils/reflectx`、`utils/rollingwindow`、`utils/syncx`。

## 语义注意

- `Pairwise([a,b,c,d])` 产生 `(a,b)`、`(c,d)`，不是滑动相邻对 `(a,b)`、`(b,c)`、`(c,d)`。
- `AsSlice` 不复制，后续原地修改可能影响原切片。
- `GetEnvByDefualt` 的函数名包含既有拼写 `Defualt`，不要自行假设存在 `GetEnvByDefault`。
- `Md5` 不适合密码存储、签名或安全 Token；只用于非安全哈希/缓存键等场景。
- 普通 `math/rand` helper 不用于安全值；安全随机字节使用 `RandBytes`，并检查其可能返回 nil。
- 随机范围通常是 `[min,max)`；编码前查看具体函数注释和边界测试。
- 集合 helper 不应掩盖复杂业务循环；当显式循环更清楚时优先循环。
