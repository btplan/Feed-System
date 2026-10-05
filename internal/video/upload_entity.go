package video

import "time"

// Upload 记录已保存文件的归属；上传完成不代表已经发布视频。
type Upload struct {
	ID          string    `gorm:"primaryKey;size:32" json:"upload_id"`
	OwnerID     int64     `gorm:"not null;index" json:"owner_id"`
	SizeBytes   int64     `gorm:"not null" json:"size_bytes"`
	ContentType string    `gorm:"not null" json:"content_type"`
	Published   bool      `gorm:"not null;default:false" json:"published"`
	CreatedAt   time.Time `json:"created_at"`
}

// UploadResponse 把上传凭据和相对访问地址交给调用方保存。
type UploadResponse struct {
	Upload
	PlaybackURL string `json:"playback_url"`
}
