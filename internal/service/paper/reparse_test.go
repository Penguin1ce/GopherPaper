package paper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasMinerUArchiveAcceptsPrefixedContentList(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "334b7dd4_content_list_v2.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	ok, err := hasMinerUArchive(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("带前缀的 content_list_v2.json 应识别为 MinerU 归档")
	}
}

func TestHasMinerUArchiveMissing(t *testing.T) {
	ok, err := hasMinerUArchive(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("空目录不应识别为 MinerU 归档")
	}
}
