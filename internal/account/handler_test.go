package account

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"clipflow/internal/auth"
	"clipflow/internal/database"
	"clipflow/internal/httpapi/middleware"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// TestRegister 验证注册会写入 bcrypt 密码哈希，并拒绝重复用户名。
func TestRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "account.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	router := gin.New()
	registerTestRoutes(router, db, auth.NewTokenManager("test-secret"))

	created := postRegister(router, `{"username":"alice_01","password":"passw0rd!"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var response RegisterResponse
	if err := json.Unmarshal(created.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	var stored User
	if err := db.First(&stored, response.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.PasswordHash == "passw0rd!" || bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("passw0rd!")) != nil {
		t.Fatalf("password hash is invalid: %+v", stored)
	}
	if bytes.Contains(created.Body.Bytes(), []byte("password")) {
		t.Fatalf("response leaks password information: %s", created.Body.String())
	}
	if duplicate := postRegister(router, `{"username":"alice_01","password":"anotherPass1"}`); duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}
	if invalid := postRegister(router, `{"username":"ab","password":"short"}`); invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

// TestLogin 验证已注册用户能登录并获得 Token，错误凭证只能得到统一的 401。
func TestLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "login.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	router := gin.New()
	registerTestRoutes(router, db, auth.NewTokenManager("test-secret"))
	if response := postJSON(router, "/users/register", `{"username":"login_user","password":"passw0rd!"}`); response.Code != http.StatusCreated {
		t.Fatalf("register status=%d body=%s", response.Code, response.Body.String())
	}
	login := postJSON(router, "/users/login", `{"username":"login_user","password":"passw0rd!"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
	var result LoginResponse
	if err := json.Unmarshal(login.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.AccessToken == "" || result.TokenType != "Bearer" || result.User.Username != "login_user" || result.User.PasswordHash != "" {
		t.Fatalf("unexpected login response: %+v", result)
	}
	profile := getProfile(router, result.AccessToken)
	if profile.Code != http.StatusOK || !bytes.Contains(profile.Body.Bytes(), []byte(`"username":"login_user"`)) || bytes.Contains(profile.Body.Bytes(), []byte("password_hash")) {
		t.Fatalf("profile status=%d body=%s", profile.Code, profile.Body.String())
	}
	if missingToken := getProfile(router, ""); missingToken.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status=%d body=%s", missingToken.Code, missingToken.Body.String())
	}
	if invalidToken := getProfile(router, "not-a-jwt"); invalidToken.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token status=%d body=%s", invalidToken.Code, invalidToken.Body.String())
	}
	if wrongPassword := postJSON(router, "/users/login", `{"username":"login_user","password":"wrongpass"}`); wrongPassword.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status=%d body=%s", wrongPassword.Code, wrongPassword.Body.String())
	}
	if unknownUser := postJSON(router, "/users/login", `{"username":"nobody","password":"passw0rd!"}`); unknownUser.Code != http.StatusUnauthorized {
		t.Fatalf("unknown user status=%d body=%s", unknownUser.Code, unknownUser.Body.String())
	}
}

// postRegister 向注册接口发送 JSON 请求并记录响应。
func postRegister(router *gin.Engine, body string) *httptest.ResponseRecorder {
	return postJSON(router, "/users/register", body)
}

// postJSON 向指定用户接口发送 JSON 请求并记录响应。
func postJSON(router *gin.Engine, path string, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	return response
}

// getProfile 携带 Bearer Token 请求当前登录用户资料。
func getProfile(router *gin.Engine, accessToken string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/users/me", nil)
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	router.ServeHTTP(response, request)
	return response
}

// registerTestRoutes 为账号测试配置与正式路由相同的账号接口和认证中间件。
func registerTestRoutes(router *gin.Engine, db *gorm.DB, tokens *auth.TokenManager) {
	handler := NewHandler(NewService(NewRepository(db), tokens))
	router.POST("/users/register", handler.Register)
	router.POST("/users/login", handler.Login)
	protected := router.Group("/")
	protected.Use(middleware.RequireUser(tokens))
	protected.GET("/users/me", handler.Profile)
}
