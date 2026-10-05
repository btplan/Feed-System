package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"clipflow/internal/account"
	"clipflow/internal/auth"
	"clipflow/internal/httpapi/request"
	"clipflow/internal/video"
	"github.com/gin-gonic/gin"
)

// TestJSONBoundary 验证真实读取长度边界、未知长度以及非法 JSON 都得到约定响应。
func TestJSONBoundary(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// 此处理函数在绑定成功后返回 200，用于观察读取边界。
	router.POST("/check", func(c *gin.Context) {
		var payload struct {
			Name string `json:"name" binding:"required"`
		}
		if !request.BindJSON(c, &payload, "invalid JSON") {
			return
		}
		c.Status(http.StatusOK)
	})
	exact := `{"name":"` + strings.Repeat("a", int(request.MaxJSONBytes)-11) + `"}`
	for _, tc := range []struct {
		name, body string
		unknown    bool
		status     int
	}{
		{"normal", `{"name":"alice"}`, false, 200},
		{"exact", exact, false, 200},
		{"oversized", exact + " ", false, 413},
		{"unknown length", exact + " ", true, 413},
		{"malformed", `{`, false, 400},
		{"empty", "", false, 400},
		{"missing required", `{}`, false, 400},
		{"two objects", `{"name":"a"}{"name":"b"}`, false, 400},
	} {
		req := httptest.NewRequest(http.MethodPost, "/check", strings.NewReader(tc.body))
		if tc.unknown {
			req.ContentLength = -1
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s: status=%d body=%s", tc.name, rec.Code, rec.Body.String())
		}
	}
}

// TestOversizedBusinessRequests 验证三个正式接口拒绝超大 JSON 且不会调用未配置的业务服务。
func TestOversizedBusinessRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := auth.NewTokenManager(strings.Repeat("k", 32))
	token, err := tokens.Issue(1, "alice")
	if err != nil {
		t.Fatal(err)
	}
	router := NewRouter(tokens, account.NewHandler(nil), video.NewHandler(nil))
	for _, path := range []string{"/users/register", "/users/login", "/videos"} {
		body := `{"padding":"` + strings.Repeat("a", int(request.MaxJSONBytes)) + `"}`
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("%s: status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}
