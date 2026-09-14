# localrepos 模块

`localrepos` 将外部数据源周期性加载为进程内只读快照，适合字典、规则、映射等可容忍短暂陈旧的数据。它属于 Infrastructure，不是 DDD Repository 的默认替代品。

```go
type Item struct {
    ID string
}

func (i Item) GetKey() string { return i.ID }

func load(ctx context.Context) (iter.Seq[Item], error) {
    return slices.Values(items), nil
}

repos := localrepos.NewLocalRepos(load,
    localrepos.WithRefreshInterval(5*time.Minute),
)
repos.Start(ctx)
defer repos.Close()
```

## 语义

- 元素实现 `HashData.GetKey() string`。
- DataStore 返回完整序列；刷新成功后替换内存映射。
- `Get` 未找到时返回类型零值，不返回 `(value, ok)`；调用方必须能区分零值，或在 Adapter 外层封装显式未找到语义。
- `AllValues/AllKeys/AllItems` 返回当前快照内容，顺序不要作为业务契约。
- `Start` 会先同步刷新一次，再启动后台 ticker；`Close` 只应在成功调用 `Start` 后执行。

## 约束

- DataStore 必须响应 Context。后台循环本身不会因 `ctx.Done()` 自动停止，调用方仍必须执行 `Close()`。
- 只缓存可整体重建的数据；不能依赖它保存唯一业务真相或未持久化写入。
- 刷新间隔按一致性和数据源压力选择，不要用极短周期替代事件通知。
