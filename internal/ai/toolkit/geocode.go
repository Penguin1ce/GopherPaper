// toolkit geocode.go 是百度地理编码 function tool:把中文地址/地标/商圈解析成经纬度,
// 供小云雀在瑞幸点单等场景把"我在国贸"变成 queryShopList 必填的坐标。
package toolkit

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// geocodeBaseURL 百度地理编码端点,测试时可替换。
var geocodeBaseURL = "https://api.map.baidu.com/geocoding/v3/"

var geocodeClient = &http.Client{Timeout: 8 * time.Second}

type geocodeInput struct {
	Address string `json:"address" jsonschema:"description=待解析的中文地址/地标/商圈,建议带上城市更准,如:北京市朝阳区国贸,required"`
}

type geocodeOutput struct {
	Longitude  float64 `json:"longitude" jsonschema:"description=经度,GCJ-02 坐标系"`
	Latitude   float64 `json:"latitude" jsonschema:"description=纬度,GCJ-02 坐标系"`
	Level      string  `json:"level,omitempty" jsonschema:"description=解析出的地址类型,如 商务大厦/道路/区县"`
	Confidence int     `json:"confidence,omitempty" jsonschema:"description=解析可信度,越大越可信"`
}

// baiduGeocodeResp 是百度地理编码 v3 的响应,status 非 0 即失败,错误文案在 msg/message。
type baiduGeocodeResp struct {
	Status  int    `json:"status"`
	Msg     string `json:"msg"`
	Message string `json:"message"`
	Result  struct {
		Location struct {
			Lng float64 `json:"lng"`
			Lat float64 `json:"lat"`
		} `json:"location"`
		Confidence int    `json:"confidence"`
		Level      string `json:"level"`
	} `json:"result"`
}

// newGeocodeTool 构建百度地理编码工具。ret_coordtype 取国测局坐标(GCJ-02),
// 与国内 App 通用坐标系一致;百度默认的 bd09ll 会偏约一公里。
// sk 非空时按百度 SN 校验规则给请求签名,对应控制台选了 SN 校验的 AK。
func newGeocodeTool(ak, sk string) tool.Tool {
	fn := func(ctx context.Context, in geocodeInput) (geocodeOutput, error) {
		return geocode(ctx, ak, sk, in)
	}
	return function.NewFunctionTool(fn,
		function.WithName("geocode"),
		function.WithDescription("把中文地址、地标或商圈名解析成经纬度(GCJ-02 坐标系)。当工具需要经纬度参数(如查询附近瑞幸门店)而用户只说了地点名称时调用,不要让用户报数字坐标。"),
	)
}

func geocode(ctx context.Context, ak, sk string, in geocodeInput) (geocodeOutput, error) {
	if in.Address == "" {
		return geocodeOutput{}, fmt.Errorf("geocode: address 不能为空")
	}
	q := url.Values{}
	q.Set("address", in.Address)
	q.Set("output", "json")
	q.Set("ret_coordtype", "gcj02ll")
	q.Set("ak", ak)
	rawQuery := q.Encode()
	// SN 校验:对「路径?查询串+SK」整体 urlencode 后取 MD5,追加为 sn 参数;
	// 签名覆盖实际发送的查询串,故 sn 必须最后拼接。
	if sk != "" {
		u, err := url.Parse(geocodeBaseURL)
		if err != nil {
			return geocodeOutput{}, fmt.Errorf("geocode: %w", err)
		}
		sn := md5.Sum([]byte(url.QueryEscape(u.Path + "?" + rawQuery + sk)))
		rawQuery += "&sn=" + fmt.Sprintf("%x", sn)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, geocodeBaseURL+"?"+rawQuery, nil)
	if err != nil {
		return geocodeOutput{}, fmt.Errorf("geocode: %w", err)
	}
	rsp, err := geocodeClient.Do(req)
	if err != nil {
		return geocodeOutput{}, fmt.Errorf("geocode: 请求失败: %w", err)
	}
	defer rsp.Body.Close()
	var br baiduGeocodeResp
	if err := json.NewDecoder(rsp.Body).Decode(&br); err != nil {
		return geocodeOutput{}, fmt.Errorf("geocode: 解析响应失败: %w", err)
	}
	if br.Status != 0 {
		return geocodeOutput{}, fmt.Errorf("geocode: 解析失败 status=%d %s%s", br.Status, br.Msg, br.Message)
	}
	return geocodeOutput{
		Longitude:  br.Result.Location.Lng,
		Latitude:   br.Result.Location.Lat,
		Level:      br.Result.Level,
		Confidence: br.Result.Confidence,
	}, nil
}
