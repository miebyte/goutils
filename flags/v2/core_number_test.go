package superflags

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestGetPreservesJSONIntegerPrecision(t *testing.T) {
	for _, raw := range []string{"0", "42", "9007199254740993", "-9007199254740993", "9223372036854775807", "-9223372036854775808"} {
		t.Run(raw, func(t *testing.T) {
			want, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			flags := New()
			flags.ReplaceConfig(map[string]json.RawMessage{"value": json.RawMessage(raw)})
			if strconv.IntSize == 64 {
				if got := flags.GetInt("value"); int64(got) != want {
					t.Errorf("GetInt = %d, want %d", got, want)
				}
			}
			if got := flags.GetDuration("value"); got != time.Duration(want) {
				t.Errorf("GetDuration = %d, want %d", got, want)
			}
			if got := flags.GetString("value"); got != raw {
				t.Errorf("GetString = %q, want %q", got, raw)
			}
			if got := isZero(flags.Get("value")); got != (want == 0) {
				t.Errorf("isZero = %t, want %t", got, want == 0)
			}
		})
	}
}

func TestGetNumberConversionCompatibility(t *testing.T) {
	flags := New()
	flags.ReplaceConfig(map[string]json.RawMessage{
		"fraction": json.RawMessage(`1.5`),
		"half":     json.RawMessage(`0.5`),
		"exponent": json.RawMessage(`1e3`),
		"timeout":  json.RawMessage(`"2s"`),
		"quoted":   json.RawMessage(`"001"`),
		"plain":    json.RawMessage(`001`),
		"trailing": json.RawMessage(`9007199254740993 1`),
		"overflow": json.RawMessage(`1e999`),
	})
	if got := flags.GetInt("fraction"); got != 1 {
		t.Errorf("fractional GetInt = %d, want 1", got)
	}
	if !flags.GetBool("half") || flags.GetFloat64("half") != 0.5 {
		t.Error("fractional bool or float conversion changed")
	}
	if got := flags.GetInt("exponent"); got != 1000 {
		t.Errorf("exponent GetInt = %d, want 1000", got)
	}
	if flags.GetString("exponent") != "1000" || flags.GetDuration("exponent") != 1000 {
		t.Error("exponent string or duration conversion changed")
	}
	if got := flags.GetDuration("timeout"); got != 2*time.Second {
		t.Errorf("string GetDuration = %v, want 2s", got)
	}
	for key, want := range map[string]string{
		"quoted": "001", "plain": "001", "trailing": "9007199254740993 1", "overflow": "1e999",
	} {
		if got := flags.GetString(key); got != want {
			t.Errorf("GetString(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestGetNumberPreservesFlagPriorityAndDefaults(t *testing.T) {
	flags := New()
	declarations := pflag.NewFlagSet("numbers", pflag.ContinueOnError)
	declarations.Int("value", 7, "")
	if err := flags.BindPFlags(declarations); err != nil {
		t.Fatal(err)
	}
	flags.SetDefault("value", int64(9007199254740993))
	if got := flags.GetDuration("value"); got != 9007199254740993 {
		t.Fatalf("default duration = %d, want 9007199254740993", got)
	}
	flags.ReplaceConfig(map[string]json.RawMessage{"value": json.RawMessage(`9007199254740995`)})
	if err := declarations.Set("value", "9"); err != nil {
		t.Fatal(err)
	}
	if got := flags.GetInt("value"); got != 9 {
		t.Fatalf("command-line value = %d, want 9", got)
	}
}
