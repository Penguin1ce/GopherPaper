package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"GopherCPP/pkg/constant"
)

// Config 是应用的全局配置，从 config/config.toml 加载。
// 模型组件按来源拆成独立配置块，由工厂按 Provider 创建。
type Config struct {
	Server    ServerConfig `toml:"server"`
	Log       LogConfig    `toml:"log"`
	Models    ModelsConfig `toml:"models"`
	Embedding ModelConfig  `toml:"embedding"`
	Milvus    MilvusConfig `toml:"milvus"`
	MySQL     MySQLConfig  `toml:"mysql"`
	Redis     RedisConfig  `toml:"redis"`
	MQ        MQConfig     `toml:"mq"`
	JWT       JWTConfig    `toml:"jwt"`
	Mail      MailConfig   `toml:"mail"`
}

type ServerConfig struct {
	Addr string `toml:"addr"` // 监听地址，如 ":8080"
	Mode string `toml:"mode"` // gin 模式: debug / release
}

type LogConfig struct {
	Level string `toml:"level"` // debug / info / warn / error
	File  string `toml:"file"`  // 空则输出到 stdout
}

// ModelsConfig 聚合了系统中用到的多个对话模型。
type ModelsConfig struct {
	// Intent 是做意图识别的小模型，本地 ollama。
	Intent ModelConfig `toml:"intent"`
	// Chat 是下游 RAG/出卷/批改 agent 使用的主力大模型，走 API。
	Chat ModelConfig `toml:"chat"`
}

// ModelConfig 描述单个模型或 embedding 组件的连接参数。
// 同一个结构既能描述 ollama 也能描述 openai，由 Provider 决定工厂走哪条分支。
type ModelConfig struct {
	Provider constant.Provider `toml:"provider"` // ollama / openai
	BaseURL  string            `toml:"base_url"` // ollama: http://localhost:11434 ; openai: 网关地址
	APIKey   string            `toml:"api_key"`  // openai 必填，ollama 留空
	Model    string            `toml:"model"`    // 模型名，如 qwen2.5:1.5b / bge-m3
	Dim      int               `toml:"dim"`      // 仅 embedding 用，需与 Milvus collection 维度对齐
}

type MilvusConfig struct {
	Address  string `toml:"address"`  // host:port，如 localhost:19530
	Username string `toml:"username"` // 可空
	Password string `toml:"password"` // 可空
	// SharedCollection 是所有租户共享的「教材知识库」collection。
	SharedCollection string `toml:"shared_collection"`
	// StudentCollection 是存放各学生私有知识库的 collection，
	// 内部按 partition 做租户隔离。
	StudentCollection string `toml:"student_collection"`
}

// MySQLConfig 业务数据库：学生、作业、试卷、批改记录等。
type MySQLConfig struct {
	DSN             string `toml:"dsn"` // user:pass@tcp(host:port)/db?charset=utf8mb4&parseTime=True&loc=Local
	MaxOpenConns    int    `toml:"max_open_conns"`
	MaxIdleConns    int    `toml:"max_idle_conns"`
	ConnMaxLifetime int    `toml:"conn_max_lifetime"` // 秒
}

// RedisConfig 缓存 / 会话 / 限流。
type RedisConfig struct {
	Addr     string `toml:"addr"` // host:port
	Password string `toml:"password"`
	DB       int    `toml:"db"`
}

// MQConfig 消息队列，默认 RabbitMQ，用于出卷批改等耗时任务异步化。
type MQConfig struct {
	URL string `toml:"url"` // amqp://user:pass@host:port/
	// 任务队列名，按业务划分。
	ExamQueue  string `toml:"exam_queue"`  // 出卷任务
	GradeQueue string `toml:"grade_queue"` // 批改任务
}

// MailConfig SMTP 邮件配置，用于发送验证码等邮件。
type MailConfig struct {
	ServerMail string `toml:"server_mail"` // 发件邮箱
	Host       string `toml:"smtp_host"`   // SMTP 服务器
	Port       int    `toml:"smtp_port"`   // SMTP 端口，SSL 一般 465
	Key        string `toml:"key"`         // 授权码或密码
}

// JWTConfig JWT 鉴权配置。学生登录后签发 token，请求时带
// Authorization: Bearer <token>，中间件校验并取出租户身份。
type JWTConfig struct {
	Secret      string `toml:"secret"`       // 签名密钥，生产务必用强随机值并保密
	Issuer      string `toml:"issuer"`       // 签发方标识
	ExpireHours int    `toml:"expire_hours"` // token 有效期，单位小时
}

// Load 从指定路径读取 toml 配置。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg.applyDefaults()
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Server.Mode == "" {
		c.Server.Mode = "debug"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
	if c.JWT.ExpireHours == 0 {
		c.JWT.ExpireHours = 24
	}
	if c.JWT.Issuer == "" {
		c.JWT.Issuer = "gophercpp"
	}
	if c.Mail.Port == 0 {
		c.Mail.Port = 465
	}
}
