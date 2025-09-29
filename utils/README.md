# utils 工具集

面向日常后端开发的一组轻量通用工具，覆盖字节与字符串处理、函数式集合操作、时间工具、随机数据、泛型容器、指针便捷函数、滑动窗口统计等。

## 目录结构

- `bytes.go`: MD5 与高性能 `AppendAny` 拼接
- `collection/`: 泛型集合，`Set`、`DefaultDict`、`DefaultDictSlice`
- `env.go`: 环境变量读取 `GetEnvByDefualt`
- `function.go`: Map/Reduce/Filter/Any/All/Find/GroupBy/Zip/Partition 及迭代版
- `lang/`: `Repr` 与占位类型 `Placeholder`
- `map.go`: `MapKeys`、`MapValues`
- `number.go`: 约束接口 `Numerical`
- `ptrx/`: 指针取/还原工具（值 <-> 指针、切片 <-> 指针切片）
- `random.go`: 随机数/字符串/字节与辅助函数
- `rollingwindow/`: 泛型滑动窗口与桶（详见子包 README）
- `slice.go`: Pairwise/Convert/SliceConvert/SliceToMap
- `stack.go`: 泛型栈 `Stack`
- `time.go`: 各时间粒度起止、工作日判断、原子时长 `AtomicDuration`

## 功能概览

- 字节与字符串（`bytes.go`）
  - `Md5(src any) []byte`、`ShortMd5(src any) []byte`
  - `AppendAny(dst []byte, v any) []byte`: 零拷贝风格将多类型高效追加到字节切片

- 函数式集合（`function.go`）
  - `Map`、`Reduce`、`Filter`、`Any`、`All`、`Find`、`GroupBy`、`Zip`、`Partition`
  - 迭代器版本：`MapIter`、`FilterIter`、`ZipIter`（基于 `iter`）

- 切片工具（`slice.go`）
  - `Pairwise`：相邻配对序列
  - `Convert`、`SliceConvert`（基于 `Converter` 接口）
  - `SliceToMap`（基于 `MapKeyer` 接口）

- 映射工具（`map.go`）
  - `MapKeys`、`MapValues`（基于 `maps`、`slices.Collect`）

- 时间工具（`time.go`）
  - 起止时间：`BeginOfMinute/Hour/Day/Week/Month/Year` 与对应 `EndOf...`
  - 判断/计算：`IsLeapYear`、`BetweenSeconds`、`DayOfYear`、`IsWeekend`
  - 便捷：`StartEndDay/Week/Month`、相对时间 `Now`、`Since`
  - 并发原子：`AtomicDuration`（`CompareAndSwap`、`Load`、`Set`）

- 随机工具（`random.go`）
  - 基本：`RandBool`、`RandInt`、`RandFloat`、`RandBytes`
  - 切片/唯一：`RandIntSlice`、`RandUniqueIntSlice`、`RandFloats`
  - 字符串：`RandString`、`RandUpper`、`RandLower`、`RandNumeral`、`RandNumeralOrLetter`、`RandSymbolChar`、`RandStringSlice`、`RandStringWithLetter`
  - 采样：`RandFromGivenSlice`、`RandSliceFromGivenSlice`
  - 辅助：`RoundToFloat`

- 指针工具（`ptrx/`）
  - 取指针：`Int/Int8/.../Float64/Bool/String`
  - 指针取值：`IntValue/.../Float64Value/BoolValue/StringValue`
  - 切片转换：`IntSlice` -> `[]*int`、`IntValueSlice` -> `[]int` 等全套类型覆盖

- 集合（`collection/`）
  - `Set[T]`：`Add/Remove/Contains/Count/Keys/Clear/Iter`
  - `DefaultDict[K,V]`：带工厂默认值的字典
  - `DefaultDictSlice[K,E,V []E]`：键 -> 动态切片的默认字典

- 语言辅助（`lang/`）
  - `Repr(any) string`：多类型友好字符串化
  - `Placeholder`、`PlaceholderType`、`AnyType`

- 泛型与约束（`number.go`）
  - `Numerical`：整数/浮点联合约束，用于滑动窗口与桶等场景

- 数据结构（`stack.go`）
  - `Stack[T]`：`Push`、`Pop`

- 滑动窗口（`rollingwindow/`）
  - `RollingWindow[T,B]`、`BucketInterface[T]`、`Bucket[T]`、`IgnoreCurrentBucket`
  - 用于 QPS、错误率、延迟统计等。详见子包 `rollingwindow/README.md`

## 并发与性能注意

- 除非特别声明，组件默认非并发安全；并发场景请在调用侧加锁或分片。
- `random.go` 内部使用包级 `rand.Rand`，并发下请自行同步或为每个 goroutine 构造独立生成器；`RandBytes` 使用 `crypto/rand` 为并发安全。
- `AppendAny`、`Repr` 等 API 旨在减少临时分配，尽量重用传入缓冲区。
- `rollingwindow` 使用固定数组与读写锁，`Add` 与 `Reduce` 为 O(1)/O(n) 量级（详见子包文档）。
