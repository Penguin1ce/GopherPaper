// Package response 把 HTTP 响应统一为 code/message/data 信封，处理层只调本包写出。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"GopherPaper/internal/dto"
)

// OK 写出成功响应，data 为业务载荷。
func OK(c *gin.Context, data any) {
	OKMsg(c, "success", data)
}

// OKMsg 写出带自定义提示语的成功响应。
func OKMsg(c *gin.Context, msg string, data any) {
	c.JSON(http.StatusOK, dto.Response{Code: 0, Message: msg, Data: data})
}

// Fail 写出失败响应，业务 code 与 HTTP status 同步。
func Fail(c *gin.Context, status int, msg string) {
	c.JSON(status, dto.Response{Code: status, Message: msg})
}

// Abort 与 Fail 相同但中断后续处理，供中间件使用。
func Abort(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, dto.Response{Code: status, Message: msg})
}
