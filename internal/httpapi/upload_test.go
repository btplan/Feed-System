package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clipflow/internal/account"
	"clipflow/internal/auth"
	"clipflow/internal/database"
	"clipflow/internal/video"
	"github.com/gin-gonic/gin"
)

// TestUploadPublishFlow 验证上传、归属检查、发布、列表、详情和分段下载组成完整业务链。
func TestUploadPublishFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := video.Migrate(db); err != nil {
		t.Fatal(err)
	}
	tokens := auth.NewTokenManager(strings.Repeat("k", 32))
	owner, err := tokens.Issue(1, "owner")
	if err != nil {
		t.Fatal(err)
	}
	other, err := tokens.Issue(2, "other")
	if err != nil {
		t.Fatal(err)
	}
	storage := filepath.Join(dir, "uploads")
	router := NewRouter(tokens, account.NewHandler(nil), video.NewHandler(video.NewService(video.NewRepository(db), storage)))
	// 仅构造 MP4 文件签名，用于传输校验测试，不把它声称为可播放的完整视频。
	mp4 := append([]byte{0, 0, 0, 24}, []byte("ftypisom\x00\x00\x00\x00isommp42")...)
	unauthorized := uploadTestFile(t, router, "", mp4)
	if unauthorized.Code != 401 {
		t.Fatalf("unauthorized=%d", unauthorized.Code)
	}
	invalid := uploadTestFile(t, router, owner, []byte("not a video"))
	if invalid.Code != 400 {
		t.Fatalf("invalid file=%d %s", invalid.Code, invalid.Body.String())
	}
	var count int64
	db.Model(&video.Upload{}).Count(&count)
	if count != 0 {
		t.Fatal("rejected file created a record")
	}
	uploaded := uploadTestFile(t, router, owner, mp4)
	if uploaded.Code != 201 {
		t.Fatalf("upload=%d %s", uploaded.Code, uploaded.Body.String())
	}
	var result video.UploadResponse
	if err := json.Unmarshal(uploaded.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.OwnerID != 1 || result.SizeBytes != int64(len(mp4)) || result.Published {
		t.Fatalf("upload=%+v", result)
	}
	stored, err := os.ReadFile(filepath.Join(storage, result.ID+".mp4"))
	if err != nil || !bytes.Equal(stored, mp4) {
		t.Fatalf("file differs: %v", err)
	}
	db.Model(&video.Video{}).Count(&count)
	if count != 0 {
		t.Fatal("upload automatically published")
	}
	body := fmt.Sprintf(`{"title":"first upload","upload_id":%q}`, result.ID)
	if got := callVideoAPI(router, "POST", "/videos", other, body); got.Code != 404 {
		t.Fatalf("foreign upload=%d %s", got.Code, got.Body.String())
	}
	if got := callVideoAPI(router, "POST", "/videos", owner, `{"title":"first","playback_url":"https://example.com/a.mp4"}`); got.Code != 400 {
		t.Fatalf("legacy publish=%d", got.Code)
	}
	if got := callVideoAPI(router, "POST", "/videos", owner, fmt.Sprintf(`{"title":" ","upload_id":%q}`, result.ID)); got.Code != 400 {
		t.Fatalf("invalid title=%d", got.Code)
	}
	published := callVideoAPI(router, "POST", "/videos", owner, body)
	if published.Code != 201 {
		t.Fatalf("publish=%d %s", published.Code, published.Body.String())
	}
	var item video.Video
	if err := json.Unmarshal(published.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.UploadID == nil || *item.UploadID != result.ID || item.PlaybackURL != result.PlaybackURL || item.AuthorID != 1 {
		t.Fatalf("video=%+v", item)
	}
	if got := callVideoAPI(router, "POST", "/videos", owner, body); got.Code != 409 {
		t.Fatalf("repeat=%d %s", got.Code, got.Body.String())
	}
	db.Model(&video.Video{}).Count(&count)
	if count != 1 {
		t.Fatalf("videos=%d", count)
	}
	var row video.Upload
	if err := db.First(&row, "id = ?", result.ID).Error; err != nil || !row.Published {
		t.Fatalf("upload not marked published: %v", err)
	}
	for _, path := range []string{"/videos", fmt.Sprintf("/videos/%d", item.ID)} {
		got := callVideoAPI(router, "GET", path, "", "")
		if got.Code != 200 || !strings.Contains(got.Body.String(), item.PlaybackURL) {
			t.Fatalf("read %s=%d %s", path, got.Code, got.Body.String())
		}
	}
	req := httptest.NewRequest("GET", item.PlaybackURL, nil)
	req.Header.Set("Range", "bytes=0-7")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 206 || !bytes.Equal(response.Body.Bytes(), mp4[:8]) || response.Header().Get("Content-Type") != "video/mp4" {
		t.Fatalf("range=%d %s", response.Code, response.Body.String())
	}
	if got := callVideoAPI(router, "GET", "/media/"+strings.Repeat("b", 32), "", ""); got.Code != 404 {
		t.Fatalf("missing=%d", got.Code)
	}
	// 超大文件保持有效签名，确认检查的是文件大小而非扩展名或签名失败。
	large := make([]byte, video.MaxVideoBytes+1)
	copy(large, mp4)
	if got := uploadTestFile(t, router, owner, large); got.Code != 413 {
		t.Fatalf("oversize=%d %s", got.Code, got.Body.String())
	}
	db.Model(&video.Upload{}).Count(&count)
	if count != 1 {
		t.Fatalf("unexpected uploads=%d", count)
	}
	entries, err := os.ReadDir(storage)
	if err != nil || len(entries) != 1 {
		t.Fatalf("unexpected stored files: %v %v", entries, err)
	}
}

// uploadTestFile 模拟调用方用 multipart 上传一个文件，不依赖真实网络或用户数据库。
func uploadTestFile(t *testing.T, router http.Handler, token string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "../../client.mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/uploads", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

// callVideoAPI 发送带可选 Token 的 JSON 请求并保留 HTTP 响应供业务断言。
func callVideoAPI(router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}
