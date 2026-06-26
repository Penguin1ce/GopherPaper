package zlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const dayLayout = "2006-01-02"

// rotateWriter 按日期切分日志文件,单文件超额同日滚动,并按数量清理历史文件。
// 文件名:prefix-2006-01-02.log,同日超额滚动为 prefix-2006-01-02.1.log、.2.log ……
type rotateWriter struct {
	mu         sync.Mutex
	dir        string
	prefix     string // 文件名前缀,如 app
	ext        string // 扩展名,含点,如 .log
	maxSize    int64  // 单文件字节上限,<=0 不限
	maxBackups int    // 保留文件数,<=0 不清理
	day        string // 当前文件所属日期
	seq        int    // 当日序号,0 为当天首个文件
	size       int64  // 当前文件已写字节
	f          *os.File
	now        func() time.Time // 取当前时间,默认 time.Now,测试可注入
}

// newRotateWriter 从 file 解析目录/前缀/扩展名,建目录并打开当天文件。
func newRotateWriter(file string, maxSizeMB, maxBackups int) (*rotateWriter, error) {
	dir := filepath.Dir(file)
	base := filepath.Base(file)
	ext := filepath.Ext(base)
	if ext == "" {
		ext = ".log"
	}
	prefix := strings.TrimSuffix(base, filepath.Ext(base))
	if prefix == "" {
		prefix = "app"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	w := &rotateWriter{
		dir:        dir,
		prefix:     prefix,
		ext:        ext,
		maxSize:    int64(maxSizeMB) * 1024 * 1024,
		maxBackups: maxBackups,
		now:        time.Now,
	}
	if err := w.openFor(w.now()); err != nil {
		return nil, err
	}
	return w, nil
}

// Write 实现 io.Writer:跨天或超额前先滚动文件,再写入。
func (w *rotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.now()
	if now.Format(dayLayout) != w.day {
		if err := w.openFor(now); err != nil {
			return 0, err
		}
	} else if w.maxSize > 0 && w.size > 0 && w.size+int64(len(p)) > w.maxSize {
		if err := w.rollSeq(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// openFor 打开 now 当天的日志文件,跨天/启动时续用当天已存在的最新序号。
func (w *rotateWriter) openFor(now time.Time) error {
	day := now.Format(dayLayout)
	w.day = day
	w.seq = w.latestSeq(day)
	return w.open()
}

// rollSeq 同日内序号加一,切到下一个文件。
func (w *rotateWriter) rollSeq() error {
	w.seq++
	return w.open()
}

// open 关闭旧文件并按当前 day/seq 打开新文件(追加),回填已写大小并触发清理。
func (w *rotateWriter) open() error {
	if w.f != nil {
		_ = w.f.Close()
		w.f = nil
	}
	name := w.fileName(w.day, w.seq)
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	w.f = f
	w.size = info.Size()
	w.cleanup()
	return nil
}

// fileName 拼出某日某序号的文件路径,seq 0 不带序号后缀。
func (w *rotateWriter) fileName(day string, seq int) string {
	if seq <= 0 {
		return filepath.Join(w.dir, w.prefix+"-"+day+w.ext)
	}
	return filepath.Join(w.dir, fmt.Sprintf("%s-%s.%d%s", w.prefix, day, seq, w.ext))
}

// latestSeq 返回某日已存在文件的最大序号,无则 0,供跨天/重启续写当日最新文件。
func (w *rotateWriter) latestSeq(day string) int {
	seq := 0
	for {
		if _, err := os.Stat(w.fileName(day, seq+1)); err != nil {
			return seq
		}
		seq++
	}
}

// cleanup 历史文件数超过 maxBackups 时,按修改时间删掉最旧的若干个。
func (w *rotateWriter) cleanup() {
	if w.maxBackups <= 0 {
		return
	}
	matches, err := filepath.Glob(filepath.Join(w.dir, w.prefix+"-*"+w.ext))
	if err != nil || len(matches) <= w.maxBackups {
		return
	}
	type entry struct {
		path string
		mod  time.Time
	}
	entries := make([]entry, 0, len(matches))
	for _, p := range matches {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		entries = append(entries, entry{path: p, mod: info.ModTime()})
	}
	// 新→旧排序,保留前 maxBackups 个,删除其余。
	sort.Slice(entries, func(i, j int) bool { return entries[i].mod.After(entries[j].mod) })
	for _, e := range entries[min(w.maxBackups, len(entries)):] {
		_ = os.Remove(e.path)
	}
}
