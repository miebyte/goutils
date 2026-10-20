package flags

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/spf13/pflag"
)

func TestGetStringSlicePreservesFlagCSV(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"backslash tab", []string{`--tags="C:\tmp,x"`}},
		{"backslash newline", []string{`--tags="C:\new,x"`}},
		{"quoted comma", []string{`--tags="a,b",c`}},
		{"quote", []string{`--tags="a""b",c`}},
		{"empty fields", []string{`--tags=,a,`}},
		{"whitespace", []string{`--tags= a `}},
		{"empty", []string{`--tags=`}},
		{"brackets", []string{`--tags=[value]`}},
		{"repeated", []string{`--tags="C:\tmp,x"`, `--tags="a""b",`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flags := New()
			declarations := pflag.NewFlagSet("slices", pflag.ContinueOnError)
			declarations.StringSlice("tags", []string{"default"}, "")
			if err := flags.BindPFlags(declarations); err != nil {
				t.Fatal(err)
			}
			flags.ReplaceConfig(map[string]json.RawMessage{"tags": json.RawMessage(`["local"]`)})
			if err := declarations.Parse(tc.args); err != nil {
				t.Fatal(err)
			}
			want, err := declarations.GetStringSlice("tags")
			if err != nil {
				t.Fatal(err)
			}
			if got := flags.GetStringSlice("tags"); !reflect.DeepEqual(got, want) {
				t.Fatalf("GetStringSlice = %#v, want pflag value %#v", got, want)
			}
			if got := flags.GetStringSlice("tags"); len(got) > 0 {
				got[0] = "changed"
				if got := flags.GetStringSlice("tags"); !reflect.DeepEqual(got, want) {
					t.Fatalf("modifying returned slice changed flag: got %#v, want %#v", got, want)
				}
			}
		})
	}
}

func TestGetStringSlicePreservesConfigAndDefaults(t *testing.T) {
	flags := New()
	defaults := []string{`C:\tmp,x`, "default"}
	flags.SetDefault("tags", defaults)
	if got := flags.GetStringSlice("tags"); !reflect.DeepEqual(got, defaults) {
		t.Fatalf("default slice = %#v, want %#v", got, defaults)
	}
	for _, raw := range []string{`["C:\\tmp,x","local"]`, `"[\"C:\\\\tmp,x\",\"local\"]"`} {
		flags.ReplaceConfig(map[string]json.RawMessage{"tags": json.RawMessage(raw)})
		want := []string{`C:\tmp,x`, "local"}
		if got := flags.GetStringSlice("tags"); !reflect.DeepEqual(got, want) {
			t.Fatalf("config %s = %#v, want %#v", raw, got, want)
		}
	}
}
