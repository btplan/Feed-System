package video

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"clipflow/internal/database"
)

// TestPublishRollback 验证视频写入失败时上传占用状态回滚，用户仍可重试发布。
func TestPublishRollback(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "rollback.db"))
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	// 模拟旧库先有视频记录，再执行本课迁移，旧播放地址必须保留。
	if err := db.Exec(`CREATE TABLE videos (id integer PRIMARY KEY, author_id integer NOT NULL, title text NOT NULL, playback_url text NOT NULL, created_at datetime)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO videos (id,author_id,title,playback_url) VALUES (1,1,'legacy','https://example.com/old.mp4')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	var old Video
	if err := db.First(&old, 1).Error; err != nil || old.UploadID != nil || old.PlaybackURL != "https://example.com/old.mp4" {
		t.Fatalf("legacy migration: %+v %v", old, err)
	}
	upload := Upload{ID: strings.Repeat("c", 32), OwnerID: 1, SizeBytes: 24, ContentType: "video/mp4"}
	if err := db.Create(&upload).Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(db)
	// 已存在的主键让 INSERT 明确失败，以此检验事务中之前的 UPDATE 是否回滚。
	conflict := Video{ID: 1, AuthorID: 1, Title: "conflict", UploadID: &upload.ID, PlaybackURL: "/media/" + upload.ID}
	if err := repo.PublishUpload(context.Background(), &conflict, upload.ID); err == nil {
		t.Fatal("expected insert failure")
	}
	var after Upload
	if err := db.First(&after, "id = ?", upload.ID).Error; err != nil || after.Published {
		t.Fatalf("upload consumed after failure: %+v %v", after, err)
	}
	conflict.ID = 0
	if err := repo.PublishUpload(context.Background(), &conflict, upload.ID); err != nil {
		t.Fatalf("retry failed: %v", err)
	}
}
