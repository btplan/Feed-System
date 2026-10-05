package video

import "time"

// VideoLike 保存一个用户对一条视频的点赞关系，组合主键阻止重复点赞。
type VideoLike struct {
	UserID    int64     `gorm:"primaryKey;autoIncrement:false;not null" json:"user_id"`
	VideoID   int64     `gorm:"primaryKey;autoIncrement:false;not null;index" json:"video_id"`
	CreatedAt time.Time `gorm:"not null" json:"created_at"`
}

// LikeResponse 告诉调用方本次操作后的点赞状态和当时的视频点赞总数。
// 这是响应结构 并不是表
type LikeResponse struct {
	VideoID   int64 `json:"video_id"`
	Liked     bool  `json:"liked"`
	LikeCount int64 `json:"like_count"`
}
