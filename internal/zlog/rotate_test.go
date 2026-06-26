package zlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRotateWriter_SizeRollAndCleanup 验证单文件超额同日滚动到序号后缀,并按 maxBackups 清理最旧文件。
func TestRotateWriter_SizeRollAndCleanup(t *testing.T) {
	dir := t.TempDir()
	w, err := newRotateWriter(filepath.Join(dir, "app.log"), 0, 2)
	if err != nil {
		t.Fatalf("建 writer 失败 %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Fatalf("关闭 writer 失败 %v", err)
		}
	}()
	w.maxSize = 100 // 直接设小阈值,免造 100MB

	// 写 5 段各 60 字节,超 100 字节即滚动,应产生 app-<day>.log / .1 / .2 ...
	for range 5 {
		if _, err := w.Write([]byte(repeat('x', 60))); err != nil {
			t.Fatalf("写入失败 %v", err)
		}
	}

	day := time.Now().Format(dayLayout)
	matches, _ := filepath.Glob(filepath.Join(dir, "app-"+day+"*.log"))
	if len(matches) > 2 {
		t.Fatalf("历史文件数应被清理到 2,实际 %d: %v", len(matches), matches)
	}
	if len(matches) == 0 {
		t.Fatal("未产生任何日志文件")
	}
	// 最新文件应存在且非空
	if _, err := os.Stat(w.fileName(day, w.seq)); err != nil {
		t.Fatalf("当前文件应存在 %v", err)
	}
}

// TestRotateWriter_DaySplit 验证跨天写入会切到新日期文件。
func TestRotateWriter_DaySplit(t *testing.T) {
	dir := t.TempDir()
	w, err := newRotateWriter(filepath.Join(dir, "app.log"), 100, 30)
	if err != nil {
		t.Fatalf("建 writer 失败 %v", err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			t.Fatalf("关闭 writer 失败 %v", err)
		}
	}()
	// 注入时钟:第一天写一条。
	d1 := time.Date(2026, 6, 22, 10, 0, 0, 0, time.Local)
	w.now = func() time.Time { return d1 }
	if _, err := w.Write([]byte("day1\n")); err != nil {
		t.Fatalf("写入失败 %v", err)
	}
	first := w.fileName(w.day, w.seq)

	// 时钟跨到第二天,再写一条应切到新日期文件。
	d2 := d1.AddDate(0, 0, 1)
	w.now = func() time.Time { return d2 }
	if _, err := w.Write([]byte("day2\n")); err != nil {
		t.Fatalf("写入失败 %v", err)
	}
	second := w.fileName(w.day, w.seq)

	if first == second {
		t.Fatalf("跨天应切到不同文件,均为 %s", first)
	}
	if w.day != d2.Format(dayLayout) {
		t.Fatalf("当前 day 应为 %s,实际 %s", d2.Format(dayLayout), w.day)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatalf("第一天文件应存在 %v", err)
	}
}

func repeat(b byte, n int) string {
	s := make([]byte, n)
	for i := range s {
		s[i] = b
	}
	return string(s)
}
