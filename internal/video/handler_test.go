package video

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"clipflow/internal/auth"
	"clipflow/internal/database"
	"clipflow/internal/httpapi/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TestPublish 验证只有带有效 Token 的用户才能创建视频，并且作者 ID 来自 Token。
func TestPublish(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "video.db"))
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
	tokens := auth.NewTokenManager("test-secret")
	router := gin.New()
	registerTestRoutes(t, router, db, tokens)
	accessToken, err := tokens.Issue(42, "video_author")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&Upload{ID: strings.Repeat("a", 32), OwnerID: 42, ContentType: "video/mp4", SizeBytes: 24}).Error; err != nil {
		t.Fatal(err)
	}
	published := postVideo(router, accessToken, `{"title":"我的第一条视频","upload_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	if published.Code != http.StatusCreated {
		t.Fatalf("publish status=%d body=%s", published.Code, published.Body.String())
	}
	var result Video
	if err := json.Unmarshal(published.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.AuthorID != 42 || result.Title != "我的第一条视频" {
		t.Fatalf("unexpected video: %+v", result)
	}
	var stored Video
	if err := db.First(&stored, result.ID).Error; err != nil || stored.AuthorID != 42 {
		t.Fatalf("stored video=%+v err=%v", stored, err)
	}
	if missingToken := postVideo(router, "", `{"title":"我的第一条视频","upload_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`); missingToken.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status=%d body=%s", missingToken.Code, missingToken.Body.String())
	}
	if legacyRequest := postVideo(router, accessToken, `{"title":"我的第一条视频","playback_url":"ftp://media.example.com/video-1.mp4"}`); legacyRequest.Code != http.StatusBadRequest {
		t.Fatalf("legacy request status=%d body=%s", legacyRequest.Code, legacyRequest.Body.String())
	}
}

// TestListNewest 验证游客能按最新顺序分页浏览视频，且非法分页参数会被拒绝。
func TestListNewest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "list.db"))
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
	for _, title := range []string{"first", "second", "third"} {
		if err := db.Create(&Video{AuthorID: 1, Title: title, PlaybackURL: "https://media.example.com/" + title + ".mp4"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	router := gin.New()
	handler := NewHandler(NewService(NewRepository(db), t.TempDir()))
	router.GET("/videos", handler.ListNewest)
	firstPage := getVideos(router, "/videos?limit=2&offset=0")
	if firstPage.Code != http.StatusOK {
		t.Fatalf("first page status=%d body=%s", firstPage.Code, firstPage.Body.String())
	}
	var first ListResponse
	if err := json.Unmarshal(firstPage.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Items[0].Title != "third" || first.Items[1].Title != "second" || !first.HasMore || first.NextOffset != 2 {
		t.Fatalf("unexpected first page: %+v", first)
	}
	secondPage := getVideos(router, "/videos?limit=2&offset=2")
	var second ListResponse
	if err := json.Unmarshal(secondPage.Body.Bytes(), &second); err != nil {
		t.Fatal(err)
	}
	if secondPage.Code != http.StatusOK || len(second.Items) != 1 || second.Items[0].Title != "first" || second.HasMore || second.NextOffset != 3 {
		t.Fatalf("unexpected second page status=%d body=%s response=%+v", secondPage.Code, secondPage.Body.String(), second)
	}
	if invalid := getVideos(router, "/videos?limit=0"); invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid page status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

// postVideo 携带可选 Bearer Token 向视频发布接口发送 JSON 请求。
func postVideo(router *gin.Engine, accessToken string, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/videos", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	router.ServeHTTP(response, request)
	return response
}

// getVideos 向公开视频列表接口发送 GET 请求并记录响应。
func getVideos(router *gin.Engine, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	router.ServeHTTP(response, request)
	return response
}

// registerTestRoutes 为视频测试配置与正式路由相同的受保护发布接口。
func registerTestRoutes(t *testing.T, router *gin.Engine, db *gorm.DB, tokens *auth.TokenManager) {
	handler := NewHandler(NewService(NewRepository(db), t.TempDir()))
	protected := router.Group("/")
	protected.Use(middleware.RequireUser(tokens))
	protected.POST("/videos", handler.Publish)
}
