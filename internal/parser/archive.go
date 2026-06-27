package parser

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SaveArtifact 把 MinerU 产物 zip 解压落盘到 destDir,保留 zip 内相对路径
// (content_list_v2.json、layout.json 等与 images/)。
// 供 worker 在解析成功后归档,为后续离线重建索引/重解析铺垫——改了分块或解析逻辑
// 时可直接读归档的 content_list_v2.json 重跑,不必重打分钟级的 MinerU 在线 API。
func SaveArtifact(destDir string, zipData []byte) error {
	if len(zipData) == 0 {
		return fmt.Errorf("parser: 产物为空,无可归档")
	}
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("parser: 打开产物失败: %w", err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("parser: 建归档目录失败: %w", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		target, err := safeJoin(destDir, f.Name)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("parser: 建归档子目录失败: %w", err)
		}
		if err := writeZipEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

// safeJoin 拼接并校验目标在 destDir 内,挡 zip-slip 路径穿越。
func safeJoin(base, name string) (string, error) {
	target := filepath.Join(base, name)
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("parser: 拒绝写入归档目录外路径: %s", name)
	}
	return target, nil
}

// writeZipEntry 流式写出单个 zip 条目,避免大图整段读进内存。
func writeZipEntry(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("parser: 读取产物条目失败: %w", err)
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("parser: 写归档文件失败: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("parser: 写归档内容失败: %w", err)
	}
	return nil
}
