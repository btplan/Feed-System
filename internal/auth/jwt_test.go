package auth_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"clipflow/internal/auth"
	"clipflow/internal/httpapi/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// TestAccessTokenPolicy 验证新令牌可用，签名或声明不合格的令牌在 HTTP 中间件处被拒绝。
func TestAccessTokenPolicy(t *testing.T) {
	secret := strings.Repeat("k", 32)
	manager := auth.NewTokenManager(secret)
	valid, err := manager.Issue(1, "alice")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := manager.Parse(valid)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != 1 || claims.Issuer != "clipflow" || claims.ExpiresAt.Sub(claims.IssuedAt.Time) != 15*time.Minute {
		t.Fatal("issued token does not match access token policy")
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	// 此测试接口用来证明认证成功后才能进入业务处理函数。
	router.GET("/protected", middleware.RequireUser(manager), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	for _, name := range []string{
		"valid", "missing token", "malformed", "wrong key", "HS384", "none",
		"missing exp", "expired", "missing iat", "future iat", "reversed time",
		"long lifetime", "missing iss", "wrong iss", "missing aud", "wrong aud",
		"missing user", "negative user", "empty username",
	} {
		now := time.Now().Unix()
		payload := jwt.MapClaims{
			"user_id": 1, "username": "alice", "iss": "clipflow", "aud": "clipflow-api",
			"iat": now, "exp": now + 900,
		}
		method := jwt.SigningMethod(jwt.SigningMethodHS256)
		var key any = []byte(secret)
		switch name {
		case "wrong key":
			key = []byte(strings.Repeat("x", 32))
		case "HS384":
			method = jwt.SigningMethodHS384
		case "none":
			method = jwt.SigningMethodNone
			key = jwt.UnsafeAllowNoneSignatureType
		case "missing exp":
			delete(payload, "exp")
		case "expired":
			payload["iat"] = now - 1800
			payload["exp"] = now - 900
		case "missing iat":
			delete(payload, "iat")
		case "future iat":
			payload["iat"] = now + 3600
			payload["exp"] = now + 4500
		case "reversed time":
			payload["iat"] = now
			payload["exp"] = now - 1
		case "long lifetime":
			payload["exp"] = now + 3600
		case "missing iss":
			delete(payload, "iss")
		case "wrong iss":
			payload["iss"] = "another-service"
		case "missing aud":
			delete(payload, "aud")
		case "wrong aud":
			payload["aud"] = "another-api"
		case "missing user":
			delete(payload, "user_id")
		case "negative user":
			payload["user_id"] = -1
		case "empty username":
			payload["username"] = ""
		}
		token, err := jwt.NewWithClaims(method, payload).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		if name == "valid" {
			token = valid
		}
		if name == "malformed" {
			token = "not-a-token"
		}
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		if name != "missing token" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		want := http.StatusUnauthorized
		if name == "valid" {
			want = http.StatusOK
		}
		if response.Code != want {
			t.Fatalf("%s: status=%d want=%d", name, response.Code, want)
		}
		if want == http.StatusUnauthorized && strings.Contains(response.Body.String(), secret) {
			t.Fatal("response leaked signing key")
		}
	}
}
