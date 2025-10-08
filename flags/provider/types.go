package provider

type Event struct {
	Key    string
	Path   string
	Config map[string]any
	Err    error
}
