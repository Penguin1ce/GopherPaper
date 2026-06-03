// Package handler 是 HTTP 处理层，处理函数为包级函数，直接作为路由处理器。
// 业务直接调用 service / auth 的包级函数，本层不持有任何依赖。
package handler

import (
	"GopherCPP/internal/agent"
	"GopherCPP/internal/dto"
)

func reply2resp(reply *agent.Reply) dto.ChatResponse {
	return dto.ChatResponse{
		Intent:  string(reply.Intent),
		Content: reply.Content,
		Meta:    reply.Meta,
	}
}
