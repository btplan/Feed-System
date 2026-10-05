package video

import (
	"context"

	"clipflow/internal/account"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SetLike 在同一事务中检查用户和视频、修改点赞关系并统计当前点赞数。
func (r *Repository) SetLike(ctx context.Context, userID, videoID int64, liked bool) (int64, error) {
	var count int64
	// 此事务将存在性检查、点赞写入和计数作为一次完整操作提交。
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 开始事务 tx是本次事务的数据库操作对象
		var userCount int64
		if err := tx.Model(&account.User{}).Where("id = ?", userID).Count(&userCount).Error; err != nil {
			return err // 检查用户是否存在 放在userCount
		}
		if userCount == 0 {
			return ErrLikeUserNotFound
		}
		var videoCount int64 // 查视频 看视频是否存在
		if err := tx.Model(&Video{}).Where("id = ?", videoID).Count(&videoCount).Error; err != nil {
			return err
		}
		if videoCount == 0 {
			return ErrVideoNotFound // 返回视频不存在错误
		}
		if liked { //如果是要点赞
			relation := VideoLike{UserID: userID, VideoID: videoID}
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "user_id"}, {Name: "video_id"}},
				DoNothing: true,
			}).Create(&relation).Error // 检查组合主键，如果冲突do nothing 不报错
			if err != nil {
				return err
			}
		} else { // 如果是取消点赞
			if err := tx.Where("user_id = ? AND video_id = ?", userID, videoID).Delete(&VideoLike{}).Error; err != nil {
				return err // 按组合主键删除，如果没有匹配记录也算成功
			}
		}
		// 统计该视频的全部点赞 写进count 把结果return 给 transcation事务，不是service!
		return tx.Model(&VideoLike{}).Where("video_id = ?", videoID).Count(&count).Error
	})
	// 事务提交
	return count, err // 把计数和错误返回给service
}