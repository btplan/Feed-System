package video

import (
	"context"
	"errors"
)

var ErrLikeUserNotFound = errors.New("authenticated user does not exist")

// SetLike 将用户期望的点赞状态写入数据库，成功后组装点赞结果。
func (s *Service) SetLike(ctx context.Context, userID, videoID int64, liked bool) (LikeResponse, error) {
	count, err := s.repo.SetLike(ctx, userID, videoID, liked)
	// repo.SetLike 在同一事务中检查用户和视频、修改点赞关系并统计当前点赞数。
	if err != nil {
		return LikeResponse{}, err
	}
	// 组装likeresponse响应格式 返回handler
	return LikeResponse{VideoID: videoID, Liked: liked, LikeCount: count}, nil
}
