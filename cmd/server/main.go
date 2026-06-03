// GopherCPP 智能编程助教 —— 服务入口。
// 启动顺序：配置、日志、基础设施、模型工厂、知识库、编排器、HTTP 路由。
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

	"GopherCPP/internal/auth"
	"GopherCPP/internal/config"
	"GopherCPP/internal/dao"
	"GopherCPP/internal/factory"
	"GopherCPP/internal/knowledge"
	"GopherCPP/internal/model"
	"GopherCPP/internal/mq"
	"GopherCPP/internal/router"
	"GopherCPP/internal/service"
	"GopherCPP/internal/zlog"
	"GopherCPP/pkg/utils"
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

	// 3. 模型工厂：意图小模型、主力大模型、embedding
	mf := factory.NewModelFactory(cfg)
	intentModel, err := mf.NewIntentModel(ctx)
	if err != nil {
		return err
	}
	chatModel, err := mf.NewChatModel(ctx)
	if err != nil {
		return err
	}
	embedder, err := mf.NewEmbedder(ctx)
	if err != nil {
		return err
	}

	// 4. 多租户知识库
	store, err := knowledge.NewStore(ctx, cfg.Milvus, embedder)
	if err != nil {
		return err
	}
	defer store.Close()
	zlog.Info("Milvus 知识库已就绪")

	// 5. 编排器：意图识别到 rag，出题/批改另走显式接口
	if err := service.Init(ctx, intentModel, chatModel, store); err != nil {
		return err
	}
	zlog.Info("助教编排器已编译")

	// 6. JWT、邮件与 HTTP 服务
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
