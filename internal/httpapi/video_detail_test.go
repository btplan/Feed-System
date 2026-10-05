package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"clipflow/internal/account"
	"clipflow/internal/auth"
	"clipflow/internal/database"
	"clipflow/internal/video"
	"github.com/gin-gonic/gin"
)

// TestVideoDetail 验证正式路由允许游客读取真实视频，并区分非法编号和不存在的视频。
func TestVideoDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "detail.db"))
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
	stored := video.Video{AuthorID: 7, Title: "a real video", PlaybackURL: "https://example.com/video.mp4"}
	if err := db.Create(&stored).Error; err != nil {
		t.Fatal(err)
	}
	tokens := auth.NewTokenManager("test-secret")
	accounts := account.NewHandler(account.NewService(account.NewRepository(db), tokens))
	videos := video.NewHandler(video.NewService(video.NewRepository(db), t.TempDir()))
	router := NewRouter(tokens, accounts, videos)

	for _, tc := range []struct {
		id     string
		status int
	}{
		{strconv.FormatInt(stored.ID, 10), http.StatusOK},
		{"999", http.StatusNotFound},
		{"abc", http.StatusBadRequest},
		{"0", http.StatusBadRequest},
		{"-1", http.StatusBadRequest},
		{"9223372036854775808", http.StatusBadRequest},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/videos/"+tc.id, nil))
		if response.Code != tc.status {
			t.Fatalf("id=%s status=%d body=%s", tc.id, response.Code, response.Body.String())
		}
		if tc.status == http.StatusOK {
			var got video.Video
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.ID != stored.ID || got.AuthorID != stored.AuthorID || got.Title != stored.Title || got.PlaybackURL != stored.PlaybackURL || !got.CreatedAt.Equal(stored.CreatedAt) {
				t.Fatalf("detail differs from database: got=%+v stored=%+v", got, stored)
			}
		}
	}
}
