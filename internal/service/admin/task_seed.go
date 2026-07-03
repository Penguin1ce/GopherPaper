package admin

import (
	"context"
	"fmt"
	mrand "math/rand"
	"time"

	"GopherPaper/internal/dao"
	"GopherPaper/internal/dto"
	"GopherPaper/internal/model"
)

// 演示任务统一以此作为创建人标记,ClearDemoTasks 仅清除该标记的数据,不误删真实任务。
const seedTaskCreator = "演示数据"

var demoTaskTitles = []struct {
	Title string
	Desc  string
}{
	{"修复 MinerU 解析偶发超时", "部分大体积 PDF 在解析阶段偶发 30s 超时,需要排查队列与重试策略。"},
	{"向量库检索召回率优化", "针对中文论文摘要检索召回偏低,评估调整分块大小与 overlap。"},
	{"后台导出增加任务看板 CSV", "运营同学希望能把任务看板导出为 CSV 便于周报统计。"},
	{"小云雀引用草稿状态回归测试", "上线前对引用草稿状态做一轮完整回归。"},
	{"知识图谱节点渲染性能", "论文超过 200 个引用时前端图谱渲染卡顿,需虚拟化或分层加载。"},
	{"管理员审计日志分页慢查询", "audit 表数据量增大后分页查询变慢,补充复合索引。"},
	{"用户反馈工单 SLA 提醒", "超过 48 小时未处理的工单自动高亮提醒。"},
	{"模型配置中心增加连通性测试", "保存配置前先做一次连通性探活,减少线上误配置。"},
	{"论文批量删除二次确认", "批量删除易误操作,增加数量确认与撤销窗口。"},
	{"站内公告支持定时发布", "公告支持设置发布时间,到点自动上架。"},
	{"标签合并的关联迁移校验", "合并标签时校验目标标签已存在的论文关联,避免重复。"},
	{"会话列表按 agent 类型筛选", "会话过多,按 agent 类型筛选便于定位问题会话。"},
	{"存储管理页体积 Top 榜", "运维希望直观看到占用体积最大的论文,便于清理。"},
	{"新用户引导流程优化", "首次登录的空状态引导不够清晰,补充示例数据入口。"},
	{"服务调用日志接入告警", "失败率超过阈值时接入企业微信告警。"},
}

var demoAssignees = []string{"云天", "向量", "图谱", "解析", "运维值班"}
var demoPriorities = []string{"urgent", "high", "high", "medium", "medium", "medium", "low"}
var demoTaskStatuses = []string{"todo", "todo", "todo", "doing", "doing", "review", "done", "done"}
var demoLabelPool = []string{"后端", "前端", "性能", "Bug", "运维", "体验", "数据", "高优"}

var demoChecklistPool = []string{
	"复现问题并定位根因",
	"编写修复方案与评审",
	"补充单元测试",
	"灰度验证",
	"更新文档",
	"通知相关同学",
}

var demoComments = []string{
	"已经在测试环境复现,初步怀疑是队列积压。",
	"方案我看过了,建议先加监控再动代码。",
	"今天先处理这个,预计明天可以进入验收。",
	"和产品确认过优先级,可以排到本周。",
	"已合并到主干,等灰度结果。",
}

// SeedDemoTasks 批量写入一批演示任务(含子清单、评论、活动),便于空库演示看板。
func SeedDemoTasks(ctx context.Context, actorID uint, actorName string) (*dto.AdminTaskSeedResult, error) {
	db := dao.DB.WithContext(ctx)
	rng := mrand.New(mrand.NewSource(time.Now().UnixNano()))
	now := time.Now()

	created := 0
	for i, t := range demoTaskTitles {
		status := demoTaskStatuses[rng.Intn(len(demoTaskStatuses))]
		priority := demoPriorities[rng.Intn(len(demoPriorities))]
		assignee := demoAssignees[rng.Intn(len(demoAssignees))]

		// 随机 1~3 个标签。
		labels := make(model.JSONStrings, 0, 3)
		labelSeen := make(map[string]struct{})
		for n := 0; n < 1+rng.Intn(3); n++ {
			l := demoLabelPool[rng.Intn(len(demoLabelPool))]
			if _, ok := labelSeen[l]; ok {
				continue
			}
			labelSeen[l] = struct{}{}
			labels = append(labels, l)
		}

		// 截止时间:部分过去(制造逾期)、部分未来、部分为空。
		var due *time.Time
		switch rng.Intn(3) {
		case 0:
			d := now.AddDate(0, 0, -rng.Intn(6)-1)
			due = &d
		case 1:
			d := now.AddDate(0, 0, rng.Intn(10)+1)
			due = &d
		}

		task := &model.AdminTask{
			Title:       t.Title,
			Description: t.Desc,
			Status:      status,
			Priority:    priority,
			Assignee:    assignee,
			Labels:      labels,
			DueAt:       due,
			OrderIdx:    i,
			CreatorID:   actorID,
			CreatorName: seedTaskCreator,
		}
		if err := db.Create(task).Error; err != nil {
			return nil, fmt.Errorf("admin: 写入演示任务失败: %w", err)
		}
		created++

		recordActivity(db, task.ID, actorID, seedTaskCreator, "created", fmt.Sprintf("创建任务「%s」", t.Title))

		// 随机 2~4 条子清单,部分已完成。
		checkN := 2 + rng.Intn(3)
		for c := 0; c < checkN; c++ {
			db.Create(&model.AdminTaskChecklistItem{
				TaskID:   task.ID,
				Content:  demoChecklistPool[(i+c)%len(demoChecklistPool)],
				Done:     rng.Intn(2) == 0,
				OrderIdx: c,
			})
		}

		// 随机 0~2 条评论。
		commentN := rng.Intn(3)
		for c := 0; c < commentN; c++ {
			db.Create(&model.AdminTaskComment{
				TaskID:     task.ID,
				AuthorID:   actorID,
				AuthorName: assignee,
				Content:    demoComments[rng.Intn(len(demoComments))],
			})
			recordActivity(db, task.ID, actorID, assignee, "commented", "发表了评论")
		}

		if status != "todo" {
			recordActivity(db, task.ID, actorID, assignee, "moved", "待办 → "+statusLabel(status))
		}
	}

	return &dto.AdminTaskSeedResult{Created: created, Message: fmt.Sprintf("已生成 %d 条演示任务", created)}, nil
}

// ClearDemoTasks 清除全部演示任务及其从属数据。
func ClearDemoTasks(ctx context.Context) (*dto.AdminTaskSeedResult, error) {
	db := dao.DB.WithContext(ctx)
	var ids []uint
	if err := db.Model(&model.AdminTask{}).
		Where("creator_name = ?", seedTaskCreator).
		Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("admin: 查询演示任务失败: %w", err)
	}
	if len(ids) == 0 {
		return &dto.AdminTaskSeedResult{Message: "没有演示任务需要清除"}, nil
	}
	if err := db.Where("task_id IN ?", ids).Delete(&model.AdminTaskComment{}).Error; err != nil {
		return nil, fmt.Errorf("admin: 清除演示评论失败: %w", err)
	}
	if err := db.Where("task_id IN ?", ids).Delete(&model.AdminTaskChecklistItem{}).Error; err != nil {
		return nil, fmt.Errorf("admin: 清除演示子清单失败: %w", err)
	}
	if err := db.Where("task_id IN ?", ids).Delete(&model.AdminTaskActivity{}).Error; err != nil {
		return nil, fmt.Errorf("admin: 清除演示活动失败: %w", err)
	}
	if err := db.Where("creator_name = ?", seedTaskCreator).Delete(&model.AdminTask{}).Error; err != nil {
		return nil, fmt.Errorf("admin: 清除演示任务失败: %w", err)
	}
	return &dto.AdminTaskSeedResult{Created: len(ids), Message: fmt.Sprintf("已清除 %d 条演示任务", len(ids))}, nil
}
