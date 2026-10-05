package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"clipflow/internal/account"
	"clipflow/internal/auth"
	"clipflow/internal/database"
	"clipflow/internal/video"
	"github.com/gin-gonic/gin"
)

// TestVideoLike 验证正式路由的重复点赞、取消、用户隔离和非法请求不会破坏关联表。
func TestVideoLike(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "likes.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := account.Migrate(db); err != nil {
		t.Fatal(err)
	}
	if err := video.Migrate(db); err != nil {
		t.Fatal(err)
	}
	for _, user := range []account.User{
		{ID: 1, Username: "alice", PasswordHash: "test-only"},
		{ID: 2, Username: "bob", PasswordHash: "test-only"},
	} {
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	item := video.Video{ID: 1, AuthorID: 1, Title: "test", PlaybackURL: "https://example.com/a.mp4"}
	if err := db.Create(&item).Error; err != nil {
		t.Fatal(err)
	}
	tokens := auth.NewTokenManager("test-secret")
	router := NewRouter(tokens,
		account.NewHandler(account.NewService(account.NewRepository(db), tokens)),
		video.NewHandler(video.NewService(video.NewRepository(db), t.TempDir())))
	for _, tc := range []struct {
		method string
		path   string
		userID int64
		status int
		liked  bool
		count  int64
	}{
		{"PUT", "/videos/1/like", 0, 401, false, 0},
		{"PUT", "/videos/abc/like", 1, 400, false, 0},
		{"PUT", "/videos/0/like", 1, 400, false, 0},
		{"PUT", "/videos/999/like", 1, 404, false, 0},
		{"PUT", "/videos/1/like", 99, 401, false, 0},
		{"PUT", "/videos/1/like?user_id=2", 1, 200, true, 1},
		{"PUT", "/videos/1/like", 1, 200, true, 1},
		{"PUT", "/videos/1/like", 2, 200, true, 2},
		{"DELETE", "/videos/1/like", 1, 200, false, 1},
		{"DELETE", "/videos/1/like", 1, 200, false, 1},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		if tc.userID != 0 {
			token, err := tokens.Issue(tc.userID, "test-user")
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s %s: status=%d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if rec.Code == http.StatusOK {
			var result video.LikeResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.VideoID != 1 || result.Liked != tc.liked || result.LikeCount != tc.count {
				t.Fatalf("unexpected response: %+v", result)
			}
		}
		var count int64
		if err := db.Model(&video.VideoLike{}).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != tc.count {
			t.Fatalf("database count=%d want=%d", count, tc.count)
		}
	}
	var remaining video.VideoLike
	if err := db.First(&remaining).Error; err != nil {
		t.Fatal(err)
	}
	if remaining.UserID != 2 {
		t.Fatalf("cancel removed another user's like: %+v", remaining)
	}
	if err := db.Create(&video.VideoLike{UserID: 2, VideoID: 1}).Error; err == nil {
		t.Fatal("database allowed a duplicate user/video pair")
	}
}
