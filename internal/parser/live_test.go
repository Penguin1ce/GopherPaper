//go:build mineru_live

package parser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"GopherPaper/internal/config"
)

func TestLiveMinerUSmoke(t *testing.T) {
	if os.Getenv("MINERU_LIVE") != "1" {
		t.Skip("set MINERU_LIVE=1 to call MinerU")
	}
	cfg, err := loadLiveConfig()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(cfg.Parser.Token) == "" || strings.Contains(cfg.Parser.Token, "your-mineru-api-token") {
		t.Skip("mineru token is empty")
	}

	Init(cfg.Parser)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Parser.PollTimeout+cfg.Parser.Timeout+60)*time.Second)
	defer cancel()

	pdf := filepath.Join(t.TempDir(), "mineru-smoke.pdf")
	if err := os.WriteFile(pdf, smokePDF(), 0o644); err != nil {
		t.Fatal(err)
	}

	batchID, putURL, err := requestUpload(ctx, filepath.Base(pdf))
	if err != nil {
		t.Fatalf("request upload: %v", err)
	}
	data, err := os.ReadFile(pdf)
	if err != nil {
		t.Fatal(err)
	}
	if err := uploadFile(ctx, putURL, data); err != nil {
		t.Fatalf("upload file: %v", err)
	}
	zipURL, err := pollBatch(ctx, batchID)
	if err != nil {
		t.Fatalf("poll batch: %v", err)
	}
	zipData, err := downloadZip(ctx, zipURL)
	if err != nil {
		t.Fatalf("download zip: %v", err)
	}

	names, rawBlocks, err := inspectZip(zipData)
	if err != nil {
		t.Fatalf("inspect zip: %v", err)
	}
	blocks, images, detailRefs, err := readArtifacts(zipData)
	if err != nil {
		t.Fatalf("read artifacts: %v", err)
	}
	doc := mapBlocks(blocks, images)
	doc.References = mergeReferences(doc.References, detailRefs)

	t.Logf("zip files: %s", strings.Join(names, ", "))
	t.Logf("raw content_list blocks=%d first=%s", len(rawBlocks), summarizeBlocks(rawBlocks, 8))
	t.Logf("parsed sections=%d paragraphs=%d figures=%d references=%d detail_refs=%d pages=%d images=%d",
		len(doc.Sections), len(doc.Paragraphs), len(doc.Figures), len(doc.References), len(detailRefs), doc.PageCount, len(images))
}

func loadLiveConfig() (*config.Config, error) {
	candidates := []string{}
	if p := os.Getenv("GOPHERPAPER_CONFIG"); p != "" {
		candidates = append(candidates, p)
	}
	candidates = append(candidates, "config/config.toml", "../../config/config.toml")
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return config.Load(p)
		}
	}
	return nil, fmt.Errorf("config/config.toml not found")
}

func inspectZip(zipData []byte) ([]string, []map[string]any, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, nil, err
	}
	names := make([]string, 0, len(zr.File))
	var blocks []map[string]any
	for _, f := range zr.File {
		names = append(names, f.Name)
		if strings.HasSuffix(f.Name, "content_list.json") {
			raw, err := readZipFile(f)
			if err != nil {
				return nil, nil, err
			}
			if err := json.Unmarshal(raw, &blocks); err != nil {
				return nil, nil, err
			}
		}
	}
	sort.Strings(names)
	return names, blocks, nil
}

func summarizeBlocks(blocks []map[string]any, limit int) string {
	if len(blocks) < limit {
		limit = len(blocks)
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%d:type=%v keys=%v", i, blocks[i]["type"], sortedKeys(blocks[i])))
	}
	return strings.Join(parts, " | ")
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func smokePDF() []byte {
	img := bytes.Repeat([]byte{0x90, 0x10, 0x10}, 40*30)
	stream := "q\n120 0 0 90 72 560 cm\n/Im1 Do\nQ\n" +
		"BT\n/F1 18 Tf\n72 760 Td\n(Introduction) Tj\n" +
		"/F1 12 Tf\n0 -28 Td\n(This is a MinerU smoke test document.) Tj\n" +
		"0 -190 Td\n(Figure 1: Test image caption.) Tj\nET\n"
	objects := [][]byte{
		[]byte("<< /Type /Catalog /Pages 2 0 R >>\n"),
		[]byte("<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n"),
		[]byte("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> /XObject << /Im1 6 0 R >> >> /Contents 4 0 R >>\n"),
		[]byte(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream\n", len(stream), stream)),
		[]byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\n"),
		append(append([]byte(fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 40 /Height 30 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length %d >>\nstream\n", len(img))), img...), []byte("\nendstream\n")...),
	}

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, obj := range objects {
		offsets[i+1] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n", i+1)
		b.Write(obj)
		b.WriteString("endobj\n")
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(objects)+1)
	b.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&b, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}
