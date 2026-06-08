// Package toolkit 把配置里的工具来源建成 trpc 的工具集与 skill 仓库,
// 供 agentrt 挂到下游 chat agent 上。全部留空则访问器返回 nil,agent 退化纯对话。
package toolkit

import (
	"fmt"

	"trpc.group/trpc-go/trpc-agent-go/skill"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/mcp"

	"GopherPaper/internal/config"
	"GopherPaper/internal/zlog"
)

// 由 Init 一次性建好,与用户无关,各用户 agent 共享同一份工具来源。
var (
	toolSets  []tool.ToolSet
	skillRepo skill.Repository
)

// Init 按配置建工具集与 skill 仓库,无配置即空,不报错。
func Init(c config.ToolsConfig) error {
	for _, m := range c.MCP {
		toolSets = append(toolSets, mcp.NewMCPToolSet(mcp.ConnectionConfig{
			Transport: m.Transport,
			ServerURL: m.ServerURL,
			Command:   m.Command,
			Args:      m.Args,
		}, mcp.WithName(m.Name)))
		zlog.Info("mcp 工具集已登记", "name", m.Name, "transport", m.Transport)
	}
	if len(c.Skills) > 0 {
		repo, err := skill.NewFSRepository(c.Skills...)
		if err != nil {
			return fmt.Errorf("toolkit: 建 skill 仓库失败: %w", err)
		}
		skillRepo = repo
		zlog.Info("skill 仓库已就绪", "roots", c.Skills)
	}
	return nil
}

// ToolSets 返回 mcp 工具集,未配置返回 nil。
func ToolSets() []tool.ToolSet { return toolSets }

// SkillRepo 返回 skill 仓库,未配置返回 nil。
func SkillRepo() skill.Repository { return skillRepo }
