package paper

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveStoredPathRemovesFileAndDir(t *testing.T) {
	oldStorageDir := storageDir
	storageDir = t.TempDir()
	t.Cleanup(func() { storageDir = oldStorageDir })

	pdfPath := filepath.Join(storageDir, "paper.pdf")
	if err := os.WriteFile(pdfPath, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeStoredPath(pdfPath, false); err != nil {
		t.Fatalf("remove file: %v", err)
	}
	if _, err := os.Stat(pdfPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("file should be removed, stat err = %v", err)
	}
	if err := removeStoredPath(pdfPath, false); err != nil {
		t.Fatalf("missing file should be idempotent: %v", err)
	}

	figDir := filepath.Join(storageDir, "figures", "paper-1")
	if err := os.MkdirAll(figDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(figDir, "fig.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeStoredPath(figDir, true); err != nil {
		t.Fatalf("remove dir: %v", err)
	}
	if _, err := os.Stat(figDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("dir should be removed, stat err = %v", err)
	}
}

func TestRemoveStoredPathRejectsOutsideStorageDir(t *testing.T) {
	oldStorageDir := storageDir
	storageDir = t.TempDir()
	t.Cleanup(func() { storageDir = oldStorageDir })

	outsidePath := filepath.Join(t.TempDir(), "outside.pdf")
	if err := os.WriteFile(outsidePath, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := removeStoredPath(outsidePath, false); err == nil {
		t.Fatal("expected outside storage path to be rejected")
	}
	if _, err := os.Stat(outsidePath); err != nil {
		t.Fatalf("outside file should remain untouched: %v", err)
	}
}
