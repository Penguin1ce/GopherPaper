// MinerU 在线 API v4 的批量上传异步流程封装。
// 三段式：申请上传链接 → PUT 上传文件 → 轮询批次结果拿产物 zip。
package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
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
		ExtractResult []struct {
			FileName   string `json:"file_name"`
			State      string `json:"state"` // pending/running/done/failed
			FullZipURL string `json:"full_zip_url"`
			ErrMsg     string `json:"err_msg"`
		} `json:"extract_result"`
	} `json:"data"`
}

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
	interval := time.Duration(cfg.PollInterval) * time.Second
	deadline := time.Now().Add(time.Duration(cfg.PollTimeout) * time.Second)
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
