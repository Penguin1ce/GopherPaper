// Package config 这里是解析config.toml的配置类
package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"

	"GopherPaper/pkg/constant"
)

// Config 是应用的全局配置，从 config/config.toml 加载。
// 模型组件按来源拆成独立配置块，由工厂按 Provider 创建。
type Config struct {
	Server    ServerConfig `toml:"server"`
	Log       LogConfig    `toml:"log"`
	Models    ModelsConfig `toml:"models"`
	Tools     ToolsConfig  `toml:"tools"`
	Embedding ModelConfig  `toml:"embedding"`
	Milvus    MilvusConfig `toml:"milvus"`
	Parser    ParserConfig `toml:"parser"`
	MySQL     MySQLConfig  `toml:"mysql"`
	Redis     RedisConfig  `toml:"redis"`
	MQ        MQConfig     `toml:"mq"`
	JWT       JWTConfig    `toml:"jwt"`
	Mail      MailConfig   `toml:"mail"`
}

// ParserConfig 是 MinerU 在线 API 的连接参数，PDF 解析走异步任务制。
type ParserConfig struct {
	BaseURL      string `toml:"base_url"`      // MinerU API 根地址，默认 https://mineru.net/api/v4
	Token        string `toml:"token"`         // API 密钥
	Timeout      int    `toml:"timeout"`       // 单次 HTTP 超时，秒
	PollInterval int    `toml:"poll_interval"` // 轮询任务状态间隔，秒
	PollTimeout  int    `toml:"poll_timeout"`  // 轮询总超时，秒
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
	// Intent 是 Host 意图路由模型，走 API 做 tool call 选专家，与下游解耦可单独换小模型。
	Intent ModelConfig `toml:"intent"`
	// Chat 是下游 RAG/抽取/报告 agent 使用的主力大模型，走 API。
	Chat ModelConfig `toml:"chat"`
	// Vlm 是带图推理用的视觉模型:解析期给图片生成描述、问答期把召回图片喂模型,与 Chat 解耦可单独换型。
	Vlm ModelConfig `toml:"vlm"`
}

// ToolsConfig 是 ai agent 的工具来源,挂在下游 chat agent 上供 mcp 调用与 skill 加载。
// 全部留空时 agent 退化为纯对话,与无工具时行为一致。
type ToolsConfig struct {
	MCP    []MCPServerConfig `toml:"mcp"`    // mcp 工具服务,每项一个 toolset
	Skills []string          `toml:"skills"` // 本地 skill 目录,作为 FSRepository 的根
}

// MCPServerConfig 描述一个 mcp 工具服务的连接方式。
type MCPServerConfig struct {
	Name      string   `toml:"name"`       // 工具集名,用于日志与冲突区分
	Transport string   `toml:"transport"`  // stdio / sse / streamable
	ServerURL string   `toml:"server_url"` // sse 与 streamable 用
	Command   string   `toml:"command"`    // stdio 用,可执行文件
	Args      []string `toml:"args"`       // stdio 用,启动参数
}

// ModelConfig 描述单个模型或 embedding 组件的连接参数。
// 同一个结构既能描述 ollama 也能描述 openai，由 Provider 决定工厂走哪条分支。
type ModelConfig struct {
	Provider constant.Provider `toml:"provider"` // ollama / openai
	BaseURL  string            `toml:"base_url"` // ollama: http://localhost:11434 ; openai: 网关地址
	APIKey   string            `toml:"api_key"`  // openai 必填，ollama 留空
	Model    string            `toml:"model"`    // 模型名，如 qwen2.5:1.5b / bge-m3
	Dim      int               `toml:"dim"`      // 仅 embedding 用，需与 Milvus collection 维度对齐
	// MaxTokens 输出 token 上限，0 表示不显式设置走 provider 默认。
	// openai 分支映射到 max_completion_tokens，覆盖 reasoning 与可见输出。
	// gpt-5 或 o 系列需要配足，避免 JSON 输出截断。
	MaxTokens int `toml:"max_tokens"`
	// ReasoningEffort 推理强度 low/medium/high，仅 openai 推理模型有效，留空走默认 medium。
	ReasoningEffort string `toml:"reasoning_effort"`
}

type MilvusConfig struct {
	Address  string `toml:"address"`  // host:port，如 localhost:19530
	Username string `toml:"username"` // 可空
	Password string `toml:"password"` // 可空
	// Collection 存放公共知识和学生私有知识。
	Collection string `toml:"collection"`
}

// MySQLConfig 业务数据库：用户、论文、元信息、会话等。
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

// MQConfig 消息队列，默认 RabbitMQ，用于 PDF 解析异步化。
type MQConfig struct {
	URL        string `toml:"url"`         // amqp://user:pass@host:port/
	ParseQueue string `toml:"parse_queue"` // PDF 解析入库流水线
}

// MailConfig SMTP 邮件配置，用于发送验证码等邮件。
type MailConfig struct {
	ServerMail    string `toml:"server_mail"`    // 发件邮箱
	Host          string `toml:"smtp_host"`      // SMTP 服务器
	Port          int    `toml:"smtp_port"`      // SMTP 端口，SSL 一般 465
	Key           string `toml:"key"`            // 授权码或密码
	RecipientMail string `toml:"recipient_mail"` // 测试/默认收件邮箱
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
		c.JWT.Issuer = "gopherpaper"
	}
	if c.Mail.Port == 0 {
		c.Mail.Port = 465
	}
	if c.Milvus.Collection == "" {
		c.Milvus.Collection = constant.DefaultKnowledgeCollection
	}
	if c.MQ.ParseQueue == "" {
		c.MQ.ParseQueue = "paper.parse"
	}
	if c.Parser.BaseURL == "" {
		c.Parser.BaseURL = "https://mineru.net/api/v4"
	}
	if c.Parser.Timeout == 0 {
		c.Parser.Timeout = 60
	}
	if c.Parser.PollInterval == 0 {
		c.Parser.PollInterval = 5
	}
	if c.Parser.PollTimeout == 0 {
		c.Parser.PollTimeout = 600
	}
}
