package router

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// TestStaticFrontend 校验内嵌的前端构建产物可经路由直出:
// / 返回页面壳,其引用的 /static/assets/*.js 可达。
func TestStaticFrontend(t *testing.T) {
	r := Init("test")

	// 首页应含标题与挂载点。
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/ status = %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "GopherPaper 科研文献智能助手") {
		t.Fatalf("/ body missing title")
	}
	if !strings.Contains(body, `id="root"`) {
		t.Fatalf("/ body missing react root")
	}

	// 资源文件名带内容哈希,从 index.html 解析后再请求,避免硬编码哈希。
	asset := regexp.MustCompile(`/static/assets/[^"']+\.js`).FindString(body)
	if asset == "" {
		t.Fatalf("/ body missing hashed js asset reference")
	}
	wa := httptest.NewRecorder()
	r.ServeHTTP(wa, httptest.NewRequest(http.MethodGet, asset, nil))
	if wa.Code != http.StatusOK {
		t.Fatalf("%s status = %d", asset, wa.Code)
	}
}
