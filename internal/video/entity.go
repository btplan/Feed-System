package video

import "time"

// Video 表示一条已经发布、可被后续视频流读取的视频元数据。
type Video struct {
	UploadID    *string   `gorm:"uniqueIndex" json:"upload_id,omitempty"`
	ID          int64     `gorm:"primaryKey" json:"id"`
	AuthorID    int64     `gorm:"not null;index" json:"author_id"`
	Title       string    `gorm:"not null" json:"title"`
	PlaybackURL string    `gorm:"not null" json:"playback_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// PublishRequest 表示调用方发布视频时提交的标题和已上传文件编号。
type PublishRequest struct {
	Title    string `json:"title"`
	UploadID string `json:"upload_id"`
}

// ListResponse 表示调用方浏览一页最新视频时获得的视频和下一页信息。
type ListResponse struct {
	Items      []Video `json:"items"`
	NextOffset int     `json:"next_offset"`
	HasMore    bool    `json:"has_more"`
}
