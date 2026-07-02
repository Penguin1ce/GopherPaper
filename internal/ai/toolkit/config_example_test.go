package toolkit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigExampleNamesBuiltinTools(t *testing.T) {
	path := filepath.Join("..", "..", "..", "config", "config.example.toml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, `openalex_api_key = ""`) {
		t.Fatal("config.example.toml should include openalex_api_key")
	}
	if !strings.Contains(content, `gopher_flow_skills = ["skills/gopher-flow"]`) {
		t.Fatal("config.example.toml should include gopher_flow_skills")
	}
	for name, display := range builtinToolDisplayNames {
		want := fmt.Sprintf(`%s = "%s"`, name, display)
		if !strings.Contains(content, want) {
			t.Fatalf("config.example.toml should name %s as %q", name, display)
		}
	}
}
