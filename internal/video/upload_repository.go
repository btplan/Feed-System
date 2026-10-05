package video

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// CreateUpload 保存文件大小和上传者身份，供后续发布时验证所有权。
func (r *Repository) CreateUpload(ctx context.Context, upload *Upload) error {
	return r.db.WithContext(ctx).Create(upload).Error
}

// FindUpload 根据上传编号读取文件元数据，不会读取磁盘中的视频内容。
func (r *Repository) FindUpload(ctx context.Context, id string) (Upload, error) {
	var upload Upload
	err := r.db.WithContext(ctx).First(&upload, "id = ?", id).Error
	return upload, err
}

// PublishUpload 在同一事务内占用本人的未发布文件并创建视频，失败时一起回滚。
func (r *Repository) PublishUpload(ctx context.Context, item *Video, uploadID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var upload Upload
		err := tx.First(&upload, "id = ? AND owner_id = ?", uploadID, item.AuthorID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUploadNotFound
		}
		if err != nil {
			return err
		}
		result := tx.Model(&Upload{}).Where("id = ? AND owner_id = ? AND published = ?", uploadID, item.AuthorID, false).Update("published", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrUploadUsed
		}
		return tx.Create(item).Error
	})
}
