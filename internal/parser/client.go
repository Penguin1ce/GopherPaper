// MinerU 在线 API v4 的批量上传异步流程封装。
// 三段式：申请上传链接 → PUT 上传文件 → 轮询批次结果拿产物 zip。
package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"GopherPaper/internal/config"
)

var (
	cfg        config.ParserConfig
	httpClient *http.Client
)

// Init 保存 MinerU API 配置与 HTTP 客户端，须在 Parse 前调用。
func Init(c config.ParserConfig) {
	cfg = c
	httpClient = &http.Client{Timeout: time.Duration(c.Timeout) * time.Second}
}

// batchURLsResp 是申请上传链接接口的响应。
type batchURLsResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		BatchID  string   `json:"batch_id"`
		FileURLs []string `json:"file_urls"`
	} `json:"data"`
}

// batchResultResp 是批次结果轮询接口的响应。
type batchResultResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ExtractResult []extractResultItem `json:"extract_result"`
	} `json:"data"`
}

type extractResultItem struct {
	FileName   string         `json:"file_name"`
	State      string         `json:"state"` // pending/running/done/failed
	FullZipURL string         `json:"full_zip_url"`
	ErrMsg     string         `json:"err_msg"`
	Raw        map[string]any `json:"-"`
}

func (i *extractResultItem) UnmarshalJSON(data []byte) error {
	type alias extractResultItem
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*i = extractResultItem(a)
	i.Raw = raw
	return nil
}

type Progress struct {
	Percent int
	Parsed  int
	Total   int
}

var pageFractionRe = regexp.MustCompile(`(\d+)\s*/\s*(\d+)`)

// requestUpload 申请上传链接，返回 batchID 与单文件的 PUT 地址。
func requestUpload(ctx context.Context, fileName string) (batchID, putURL string, err error) {
	body, _ := json.Marshal(map[string]any{
		"enable_formula": true,
		"enable_table":   true,
		"files": []map[string]any{
			{"name": fileName, "is_ocr": true},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.BaseURL+"/file-urls/batch", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Token)

	var out batchURLsResp
	if err := doJSON(req, &out); err != nil {
		return "", "", err
	}
	if out.Code != 0 {
		return "", "", fmt.Errorf("parser: 申请上传链接失败: %s", out.Msg)
	}
	if out.Data.BatchID == "" || len(out.Data.FileURLs) == 0 {
		return "", "", fmt.Errorf("parser: 上传链接响应为空")
	}
	return out.Data.BatchID, out.Data.FileURLs[0], nil
}

// uploadFile 把文件字节 PUT 到预签名地址，不带鉴权头。
func uploadFile(ctx context.Context, putURL string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("parser: 上传文件失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("parser: 上传文件状态码 %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// pollBatch 轮询批次直到该文件 done/failed，返回产物 zip 地址。
func pollBatch(ctx context.Context, batchID string) (string, error) {
	return pollBatchProgress(ctx, batchID, nil)
}

func pollBatchProgress(ctx context.Context, batchID string, onProgress func(Progress)) (string, error) {
	interval := time.Duration(cfg.PollInterval) * time.Second
	deadline := time.Now().Add(time.Duration(cfg.PollTimeout) * time.Second)
	var last Progress
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.BaseURL+"/extract-results/batch/"+batchID, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Authorization", "Bearer "+cfg.Token)

		var out batchResultResp
		if err := doJSON(req, &out); err != nil {
			return "", err
		}
		if out.Code != 0 {
			return "", fmt.Errorf("parser: 查询批次结果失败: %s", out.Msg)
		}
		for _, r := range out.Data.ExtractResult {
			if onProgress != nil {
				if p, ok := extractProgress(r.Raw); ok && p != last {
					onProgress(p)
					last = p
				}
			}
			switch r.State {
			case "done":
				if r.FullZipURL == "" {
					return "", fmt.Errorf("parser: 解析完成但缺少产物地址")
				}
				return r.FullZipURL, nil
			case "failed":
				return "", fmt.Errorf("parser: MinerU 解析失败: %s", r.ErrMsg)
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("parser: 轮询解析结果超时")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}
	}
}

// extractProgress reads page-level progress from MinerU responses across versions.
func extractProgress(raw map[string]any) (Progress, bool) {
	if raw == nil {
		return Progress{}, false
	}
	p := progressFromMap(raw)
	for _, key := range []string{"progress", "parse_progress", "extract_progress", "page_progress", "pages_progress", "processing_progress", "task_progress"} {
		switch v := raw[key].(type) {
		case map[string]any:
			p = fillProgress(p, progressFromMap(v))
		default:
			if parsed, total, ok := pageFraction(v); ok {
				p = fillProgress(p, Progress{Parsed: parsed, Total: total})
			}
		}
	}
	p = normalizeProgress(p)
	return p, p.Percent > 0 || p.Parsed > 0 || p.Total > 0
}

func progressFromMap(m map[string]any) Progress {
	p := Progress{
		Percent: firstPercent(m, "percent", "percentage", "progress", "parse_progress", "extract_progress", "extract_progress_percent", "rate", "ratio"),
		Parsed:  firstInt(m, "parsed_pages", "parse_pages", "extracted_pages", "processed_pages", "completed_pages", "current_page", "page", "done", "finished"),
		Total:   firstInt(m, "total_pages", "total_page", "page_count", "pages", "total"),
	}
	for _, key := range []string{"page_progress", "pages_progress", "parsed_pages", "parse_pages", "extracted_pages", "processed_pages"} {
		if parsed, total, ok := pageFraction(m[key]); ok {
			p = fillProgress(p, Progress{Parsed: parsed, Total: total})
		}
	}
	return p
}

func fillProgress(dst, src Progress) Progress {
	if dst.Percent == 0 && src.Percent > 0 {
		dst.Percent = src.Percent
	}
	if dst.Parsed == 0 && src.Parsed > 0 {
		dst.Parsed = src.Parsed
	}
	if dst.Total == 0 && src.Total > 0 {
		dst.Total = src.Total
	}
	return dst
}

func normalizeProgress(p Progress) Progress {
	if p.Parsed < 0 {
		p.Parsed = 0
	}
	if p.Total < 0 {
		p.Total = 0
	}
	if p.Total > 0 && p.Parsed > p.Total {
		p.Parsed = p.Total
	}
	if p.Percent == 0 && p.Total > 0 && p.Parsed > 0 {
		p.Percent = p.Parsed * 100 / p.Total
	}
	if p.Percent < 0 {
		p.Percent = 0
	}
	if p.Percent > 100 {
		p.Percent = 100
	}
	return p
}

func firstInt(m map[string]any, keys ...string) int {
	for _, key := range keys {
		if v := anyInt(m[key]); v > 0 {
			return v
		}
	}
	return 0
}

func firstPercent(m map[string]any, keys ...string) int {
	for _, key := range keys {
		if v := anyPercent(m[key], key == "ratio" || key == "rate"); v > 0 {
			return v
		}
	}
	return 0
}

func anyInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int8:
		return int(n)
	case int16:
		return int(n)
	case int32:
		return int(n)
	case int64:
		return int(n)
	case uint:
		return int(n)
	case uint8:
		return int(n)
	case uint16:
		return int(n)
	case uint32:
		return int(n)
	case uint64:
		if n > uint64(^uint(0)>>1) {
			return 0
		}
		return int(n)
	case float32:
		return int(n)
	case float64:
		return int(n)
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i)
		}
		f, _ := n.Float64()
		return int(f)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err == nil {
			return int(f)
		}
	}
	return 0
}

func anyPercent(v any, ratioKey bool) int {
	switch n := v.(type) {
	case int:
		return percentNumber(float64(n), ratioKey)
	case int8:
		return percentNumber(float64(n), ratioKey)
	case int16:
		return percentNumber(float64(n), ratioKey)
	case int32:
		return percentNumber(float64(n), ratioKey)
	case int64:
		return percentNumber(float64(n), ratioKey)
	case uint:
		return percentNumber(float64(n), ratioKey)
	case uint8:
		return percentNumber(float64(n), ratioKey)
	case uint16:
		return percentNumber(float64(n), ratioKey)
	case uint32:
		return percentNumber(float64(n), ratioKey)
	case uint64:
		return percentNumber(float64(n), ratioKey)
	case float32:
		return percentNumber(float64(n), ratioKey)
	case float64:
		return percentNumber(n, ratioKey)
	case json.Number:
		f, err := n.Float64()
		if err != nil {
			return 0
		}
		return percentNumber(f, ratioKey)
	case string:
		return percentString(n, ratioKey)
	}
	return 0
}

func percentNumber(v float64, ratioKey bool) int {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if ratioKey {
		if v > 0 && v <= 1 {
			v *= 100
		}
	} else if v > 0 && v < 1 {
		v *= 100
	}
	return int(math.Round(v))
}

func percentString(v string, ratioKey bool) int {
	s := strings.TrimSpace(v)
	if s == "" {
		return 0
	}
	hasPercent := strings.HasSuffix(s, "%")
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return percentNumber(f, ratioKey && !hasPercent)
}

func pageFraction(v any) (parsed, total int, ok bool) {
	s, ok := v.(string)
	if !ok {
		return 0, 0, false
	}
	match := pageFractionRe.FindStringSubmatch(s)
	if len(match) != 3 {
		return 0, 0, false
	}
	parsed, err1 := strconv.Atoi(match[1])
	total, err2 := strconv.Atoi(match[2])
	if err1 != nil || err2 != nil || total <= 0 {
		return 0, 0, false
	}
	if parsed > total {
		parsed = total
	}
	return parsed, total, true
}

// downloadZip 拉取产物 zip 字节。
func downloadZip(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("parser: 下载产物失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("parser: 下载产物状态码 %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// doJSON 执行请求并把响应体解码到 v。
func doJSON(req *http.Request, v any) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("parser: 请求 MinerU 失败: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("parser: MinerU 状态码 %d: %s", resp.StatusCode, string(b))
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("parser: 解析 MinerU 响应失败: %w", err)
	}
	return nil
}

// baseName 取文件名,用于上传请求。
func baseName(fileURI string) string {
	return filepath.Base(fileURI)
}
