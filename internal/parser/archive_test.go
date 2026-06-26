package parser

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSaveArtifactUnzips(t *testing.T) {
	dest := t.TempDir()
	zipData := makeZip(t, map[string]string{
		"content_list.json": `[{"type":"text"}]`,
		"images/a.jpg":       "imgbytes",
	})
	if err := SaveArtifact(dest, zipData); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "content_list.json"))
	if err != nil || string(got) != `[{"type":"text"}]` {
		t.Fatalf("content_list 未正确落盘: got=%q err=%v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(dest, "images", "a.jpg")); err != nil || string(got) != "imgbytes" {
		t.Fatalf("images 子目录未正确落盘: got=%q err=%v", got, err)
	}
}

func TestSaveArtifactRejectsTraversal(t *testing.T) {
	dest := t.TempDir()
	zipData := makeZip(t, map[string]string{"../escape.txt": "evil"})
	if err := SaveArtifact(dest, zipData); err == nil {
		t.Fatal("应拒绝穿越路径,实际成功了")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "escape.txt")); err == nil {
		t.Fatal("穿越文件被写到了归档目录外")
	}
}

func TestSaveArtifactEmpty(t *testing.T) {
	if err := SaveArtifact(t.TempDir(), nil); err == nil {
		t.Fatal("空产物应报错")
	}
}
