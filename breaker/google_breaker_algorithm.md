## GoogleBreaker 算法说明

本算法实现参考 Google SRE Book 中的客户端限流（Client-Side Throttling）思想，通过“滑动窗口统计 + 动态阈值 + 概率丢弃”在过载时自适应限流，避免级联故障与抖动。

### 参数
- **时间窗口 window**: 10s
- **桶数量 buckets**: 40（每桶 250ms）
- **阈值因子 k/minK**: 1.5 / 1.1（动态权重上下限）
- **保护项 protection**: 5（低 QPS 保护）
- **强制通过间隔 forcePassDuration**: 1s（恢复探测）

### 统计口径（滑动窗口）
窗口内按桶聚合以下指标：
- `Success`、`Failure`、`Sum`

汇总得到：
- `accepts`: 所有桶的 `Success` 之和
- `total`: 所有桶的 `Sum` 之和
- `failingBuckets`: 最近一段时间“仅失败无成功”的连续桶计数（遇到成功清零）
- `workingBuckets`: 最近一段时间“仅成功无失败”的连续桶计数（遇到失败清零）

### 判定流程（accept）
1) 动态权重：失败桶越多，权重越趋向保守

    w = k - (k - minK) * (failingBuckets / buckets)

2) 加权接受量：

    weightedAccepts = max(w, minK) * accepts

3) 基础丢弃率（含保护项）：

    dropRatio = ((total - protection) - weightedAccepts) / (total + 1)

   若 `dropRatio <= 0`，直接放行。

4) 强制通过（恢复探测）：若距离 `lastPass` 超过 1s，则本次直接放行并更新时间。

5) 近期工作调节：

    dropRatio *= (buckets - workingBuckets) / buckets

   “连续仅成功”越多，乘子越小，丢弃率越低。

6) 概率丢弃：以 `dropRatio` 的概率拒绝请求（返回 `ErrServiceUnavailable`）；否则放行并更新时间。

### 行为特性
- **自适应限流**：失败增多或总量上升时提高丢弃概率；恢复后快速回落。
- **平滑与反抖**：概率丢弃避免“全放/全拒”；滑动窗口天然平滑。
- **低流量保护**：`protection` 在低 QPS 场景避免被少量失败误伤。
- **快速恢复探测**：`forcePassDuration` 确保至少每秒探测一次服务健康。

### 调参建议
- **k/minK**：放大或收敛对成功数的“容忍度”，k 越大越保守；minK 拉高最低容忍基线。
- **protection**：低 QPS 增大保护项更稳；高 QPS 可适度降低以更敏感。
- **window/buckets**：窗口越大越平滑但反应更慢；桶越多精度更高但开销略增。
- **forcePassDuration**：探测频率；过小会增加穿透，过大恢复响应变慢。

### 与代码对应关系
- 判定逻辑位于 `googlebreaker.accept()`：权重计算、丢弃率、强制通过、工作调节与概率丢弃。
- 打点流转：
  - `allow()`/`doReq()` 调用 `accept()`；拒绝时计 `drop`。
  - 放行后根据执行结果计 `success` 或 `failure`，形成统计闭环。


