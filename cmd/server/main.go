// GopherPaper 科研文献智能解析与知识服务系统 —— 服务入口。
// 启动顺序：配置、日志、基础设施、模型资源、知识库、编排器、HTTP 路由。
package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"GopherPaper/internal/ai"
	"GopherPaper/internal/ai/toolkit"
	"GopherPaper/internal/aimodel"
	"GopherPaper/internal/auth"
	"GopherPaper/internal/config"
	"GopherPaper/internal/dao"
	"GopherPaper/internal/history"
	"GopherPaper/internal/knowledge"
	"GopherPaper/internal/model"
	"GopherPaper/internal/mq"
	"GopherPaper/internal/parser"
	"GopherPaper/internal/router"
	paperservice "GopherPaper/internal/service/paper"
	"GopherPaper/internal/zlog"
	"GopherPaper/pkg/utils"
)

func main() {
	cfgPath := flag.String("c", "config/config.toml", "配置文件路径")
	flag.Parse()

	if err := run(*cfgPath); err != nil {
		zlog.Error("服务启动失败", "err", err)
		os.Exit(1)
	}
}

func run(cfgPath string) error {
	// 1. 配置 + 日志
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if err := zlog.Init(cfg.Log.Level, cfg.Log.File); err != nil {
		return err
	}

	ctx := context.Background()

	// 2. 基础设施，均由 deploy/docker-compose.yml 提供
	if err := dao.InitMySQL(cfg.MySQL); err != nil {
		return err
	}
	if err := model.AutoMigrate(dao.DB); err != nil {
		return err
	}
	zlog.Info("MySQL 已连接，数据表已就绪")

	// 会话历史：trpc MySQL Session 承载多轮上下文与历史，须在 MySQL 之后
	if err := history.Init(cfg.MySQL); err != nil {
		return err
	}
	zlog.Info("会话历史 Session 已就绪")

	if err := dao.InitRedis(ctx, cfg.Redis); err != nil {
		return err
	}
	defer dao.RDB.Close()
	zlog.Info("Redis 已连接")

	mqClient, err := mq.New(cfg.MQ)
	if err != nil {
		return err
	}
	defer mqClient.Close()
	zlog.Info("RabbitMQ 已连接")

	// 3. 模型资源：embedding 启动期建一次,意图/对话模型按用户懒建
	aimodel.Init(cfg)

	// agent 工具来源：mcp 工具集与 skill 仓库,挂到下游 chat agent,无配置则纯对话
	if err := toolkit.Init(cfg.Tools); err != nil {
		return err
	}

	// 4. 多租户知识库
	if err := knowledge.InitTRPCStore(ctx, cfg.Milvus, cfg.Milvus.Collection, aimodel.NewEmbedder(cfg.Embedding), cfg.Embedding.Dim); err != nil {
		return err
	}
	defer knowledge.CloseTRPC()
	zlog.Info("Milvus 知识库已就绪")

	// 5. PDF 解析：MinerU 在线 API 客户端
	parser.Init(cfg.Parser)
	zlog.Info("PDF 解析器已就绪")

	// 6. 编排器：只准备全局检索器,模型由 aimodel 按用户缓存
	if err := ai.Init(ctx); err != nil {
		return err
	}
	zlog.Info("编排器已就绪")

	// 7. 论文解析入库：注入 MQ 句柄并拉起解析消费者
	if err := paperservice.Init(ctx, mqClient, cfg.MQ.ParseQueue); err != nil {
		return err
	}
	zlog.Info("论文解析消费者已启动")

	// 7. JWT、邮件与 HTTP 服务
	auth.Init(cfg.JWT)
	utils.InitMail(cfg.Mail)
	engine := router.Init(cfg.Server.Mode)
	srv := &http.Server{Addr: cfg.Server.Addr, Handler: engine}

	go func() {
		zlog.Info("HTTP 服务启动", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			zlog.Error("HTTP 服务异常退出", "err", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	zlog.Info("收到退出信号，开始优雅关闭")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
