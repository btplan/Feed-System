package video

import (
	"context"

	"gorm.io/gorm"
)

// Repository 负责 videos 表的视频元数据读写操作。
type Repository struct {
	db *gorm.DB
}

// NewRepository 为视频业务创建数据库访问对象。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create 将已经通过发布规则验证的视频写入 videos 表。
func (r *Repository) Create(ctx context.Context, video *Video) error {
	return r.db.WithContext(ctx).Create(video).Error
}

// ListNewest 按发布时间和 ID 从新到旧读取一页加一条视频，用于判断是否还有下一页。
func (r *Repository) ListNewest(ctx context.Context, limit int, offset int) ([]Video, error) {
	var videos []Video
	// 拿到下面这些数据
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Order("id DESC").
		Limit(limit + 1). // 多读一页计算has_more
		Offset(offset).
		Find(&videos).Error // 让 Gorm 执行 SELECT，并填充 []Video。
	return videos, err
}

// Migrate 创建或补齐视频表、点赞关系表以及对应的索引。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&Video{}, &VideoLike{}, &Upload{})
}

// FindByID 根据视频编号读取一条视频，找不到时保留数据库的未找到错误。
func (r *Repository) FindByID(ctx context.Context, videoID int64) (Video, error) {
	var video Video
	err := r.db.WithContext(ctx).First(&video, videoID).Error
	return video, err
}
