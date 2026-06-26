package toolkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigExampleNamesDeleteMyPaperTool(t *testing.T) {
	path := filepath.Join("..", "..", "..", "config", "config.example.toml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config example: %v", err)
	}
	if !strings.Contains(string(b), `delete_my_paper = "删除我的论文"`) {
		t.Fatal("config.example.toml should name delete_my_paper")
	}
}
