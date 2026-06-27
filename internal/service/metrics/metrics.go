package metrics

import (
	"context"
	"strings"
	"time"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/model"
	"GopherPaper/internal/zlog"
)

const (
	ServiceChat   = "chat"
	ServiceReport = "report"
	ServiceParse  = "parse"
)

func Record(ctx context.Context, serviceType, actorID, paperID, sessionID string, success bool, duration time.Duration, err error) {
	if dao.DB == nil {
		return
	}
	msg := ""
	if err != nil {
		msg = err.Error()
		if len(msg) > 1024 {
			msg = msg[:1024]
		}
	}
	rec := &model.ServiceCallLog{
		ServiceType:  serviceType,
		ActorID:      actorID,
		PaperID:      paperID,
		SessionID:    sessionID,
		Success:      success,
		DurationMS:   duration.Milliseconds(),
		ErrorMessage: strings.TrimSpace(msg),
		CreatedAt:    time.Now(),
	}
	if dbErr := dao.DB.WithContext(context.WithoutCancel(ctx)).Create(rec).Error; dbErr != nil {
		zlog.Error("record service metric failed", "service", serviceType, "err", dbErr)
	}
}
