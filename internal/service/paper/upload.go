package paper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	paperdao "GopherPaper/internal/dao/paper"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/errs"
)

// Upload 校验并落盘 PDF,建论文记录并投递解析任务,返回论文记录。
// HTTP 上传与小云雀的 download_paper 工具共用此入口,均落盘建记录并进解析流水线。
func Upload(ctx context.Context, ownerID, fileName string, data []byte) (*model.Paper, error) {
	if !strings.EqualFold(filepath.Ext(fileName), ".pdf") {
		return nil, errs.ErrInvalidFile
	}
	if len(data) == 0 {
		return nil, errs.ErrInvalidFile
	}

	fileURI := filepath.Join(storageDir, uuid.NewString()+".pdf")
	if err := os.WriteFile(fileURI, data, 0o644); err != nil {
		return nil, fmt.Errorf("service/paper: 落盘失败: %w", err)
	}

	p := &model.Paper{
		OwnerID:  ownerID,
		Title:    strings.TrimSuffix(fileName, filepath.Ext(fileName)),
		FileName: fileName,
		FileURI:  fileURI,
		Size:     int64(len(data)),
	}
	if err := paperdao.Create(ctx, p); err != nil {
		_ = os.Remove(fileURI)
		return nil, err
	}

	if err := publishParse(ctx, p); err != nil {
		// 投递失败兜底后台解析,保证流程不中断。
		zlog.Error("投递解析队列失败,改后台解析", "paper_id", p.ID, "err", err)
		go runPipeline(context.WithoutCancel(ctx), taskOf(p))
	}
	return p, nil
}

// parseTask 是投递给解析消费者的任务负载。
type parseTask struct {
	PaperID      string `json:"paper_id"`
	OwnerID      string `json:"owner_id"`
	FileURI      string `json:"file_uri"`
	FileName     string `json:"file_name"`
	Mode         string `json:"mode,omitempty"`
	RebuildGraph bool   `json:"rebuild_graph,omitempty"`
}

const parseTaskModeMinerUArchive = "mineru_archive"

func taskOf(p *model.Paper) parseTask {
	return parseTask{PaperID: p.ID, OwnerID: p.OwnerID, FileURI: p.FileURI, FileName: p.FileName}
}

// publishParse 把解析任务投递到队列。
func publishParse(ctx context.Context, p *model.Paper) error {
	body, err := json.Marshal(taskOf(p))
	if err != nil {
		return fmt.Errorf("service/paper: 序列化解析任务失败: %w", err)
	}
	return mqClient.Publish(ctx, parseQueue, body)
}
