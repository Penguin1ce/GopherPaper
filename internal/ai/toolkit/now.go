// toolkit now.go 是获取系统时间的 function tool:模型自身不知道当下时间,
// 涉及今天、现在、几点的问题先调它取真实时间再回答。
package toolkit

import (
	"context"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

type nowInput struct {
	Timezone string `json:"timezone,omitempty" jsonschema:"description=IANA 时区名,如 Asia/Shanghai、America/New_York,留空用服务器本地时区"`
}

type nowOutput struct {
	Datetime string `json:"datetime" jsonschema:"description=当前时间,格式 2006-01-02 15:04:05"`
	Weekday  string `json:"weekday" jsonschema:"description=星期几"`
	Timezone string `json:"timezone" jsonschema:"description=实际使用的时区"`
	Unix     int64  `json:"unix" jsonschema:"description=Unix 秒级时间戳"`
}

// newNowTool 构建系统时间工具,纯本地计算无外部依赖。
func newNowTool() tool.Tool {
	fn := func(ctx context.Context, in nowInput) (nowOutput, error) {
		loc := time.Local
		if in.Timezone != "" {
			l, err := time.LoadLocation(in.Timezone)
			if err != nil {
				return nowOutput{}, fmt.Errorf("current_time: 无效时区 %q: %w", in.Timezone, err)
			}
			loc = l
		}
		t := time.Now().In(loc)
		return nowOutput{
			Datetime: t.Format("2006-01-02 15:04:05"),
			Weekday:  t.Format("Monday"),
			Timezone: loc.String(),
			Unix:     t.Unix(),
		}, nil
	}
	return function.NewFunctionTool(fn,
		function.WithName("current_time"),
		function.WithDescription("获取当前系统时间与星期,可指定 IANA 时区。凡涉及今天、现在、最近多久等与时间相关的问题先调用本工具,不要凭猜测回答日期。"),
	)
}
