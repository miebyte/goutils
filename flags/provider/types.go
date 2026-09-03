package provider

// Event 表示一次配置变更。Key 为空表示整份配置替换，非空表示只更新对应配置项。
type Event struct {
	Key    string
	Path   string
	Config map[string]any
	Err    error
}
