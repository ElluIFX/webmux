package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomPresetNameIsReserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := "[[presets]]\nname = \"custom\"\nlabel = \"Configured command\"\ncmd = [\"/bin/echo\"]\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("custom preset validation = %v, want reserved-name error", err)
	}
}
