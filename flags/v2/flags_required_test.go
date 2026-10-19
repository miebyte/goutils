package superflags

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestRequiredFlagsUseDeclaredType(t *testing.T) {
	if flagType := os.Getenv("SUPERFLAGS_REQUIRED_TEST_TYPE"); flagType != "" {
		sf = New()
		requiredFlags = nil
		pflag.CommandLine = pflag.NewFlagSet("required", pflag.ContinueOnError)
		switch flagType {
		case "int":
			IntRequired("value", "")
		case "float64":
			Float64Required("value", "")
		case "bool":
			BoolRequired("value", "")
		case "duration":
			DurationRequired("value", "")
		case "string":
			StringRequired("value", "")
		}
		value := os.Getenv("SUPERFLAGS_REQUIRED_TEST_VALUE")
		if os.Getenv("SUPERFLAGS_REQUIRED_TEST_SOURCE") == "config" {
			sf.ReplaceConfig(map[string]json.RawMessage{"value": json.RawMessage(value)})
		} else if err := pflag.CommandLine.Set("value", value); err != nil {
			t.Fatal(err)
		}
		checkFlagKey()
		return
	}

	for _, tc := range []struct {
		name     string
		flagType string
		source   string
		value    string
		missing  bool
	}{
		{"cli int zero", "int", "cli", "0", true},
		{"cli float zero", "float64", "cli", "0.0", true},
		{"cli bool false", "bool", "cli", "false", true},
		{"cli duration zero", "duration", "cli", "0s", true},
		{"cli empty string", "string", "cli", "", true},
		{"cli int nonzero", "int", "cli", "3", false},
		{"cli float nonzero", "float64", "cli", "0.5", false},
		{"cli bool true", "bool", "cli", "true", false},
		{"cli duration nonzero", "duration", "cli", "2s", false},
		{"cli string zero", "string", "cli", "0", false},
		{"cli string false", "string", "cli", "false", false},
		{"config int zero", "int", "config", "0", true},
		{"config float zero", "float64", "config", "0.0", true},
		{"config bool false", "bool", "config", "false", true},
		{"config duration zero", "duration", "config", `"0s"`, true},
		{"config duration nonzero", "duration", "config", `"2s"`, false},
		{"config int nonzero", "int", "config", "3", false},
		{"config float nonzero", "float64", "config", "0.5", false},
		{"config bool true", "bool", "config", "true", false},
		{"config string zero", "string", "config", `"0"`, false},
		{"config numeric string zero", "string", "config", "0", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestRequiredFlagsUseDeclaredType$")
			cmd.Env = append(os.Environ(),
				"SUPERFLAGS_REQUIRED_TEST_TYPE="+tc.flagType,
				"SUPERFLAGS_REQUIRED_TEST_SOURCE="+tc.source,
				"SUPERFLAGS_REQUIRED_TEST_VALUE="+tc.value,
			)
			output, err := cmd.CombinedOutput()
			if tc.missing {
				if err == nil || !strings.Contains(string(output), "Missing key: value") {
					t.Fatalf("zero value was not rejected: err=%v, output=%s", err, output)
				}
			} else if err != nil {
				t.Fatalf("nonzero value was rejected: err=%v, output=%s", err, output)
			}
		})
	}
}
