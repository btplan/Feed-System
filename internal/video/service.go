package video

import (
	"context"
	"errors"

	"strings"

	"gorm.io/gorm"
)

var (
	ErrInvalidTitle    = errors.New("title must be 1-100 characters")
	ErrInvalidUploadID = errors.New("upload_id must be a 32-character upload identifier")
	ErrVideoNotFound   = errors.New("video does not exist")
)

// Service 执行发布视频需要的输入校验和视频创建业务规则。
type Service struct {
	repo      *Repository
	uploadDir string
}

// NewService 为视频发布业务配置视频数据访问对象。
func NewService(repo *Repository, uploadDir string) *Service {
	return &Service{repo: repo, uploadDir: uploadDir}
}

// Publish 校验视频资料，并将当前登录用户作为作者写入新视频。
func (s *Service) Publish(ctx context.Context, authorID int64, req PublishRequest) (Video, error) {
	title := strings.TrimSpace(req.Title)
	if length := len([]rune(title)); length < 1 || length > 100 {
		return Video{}, ErrInvalidTitle
	}
	if !uploadIDPattern.MatchString(req.UploadID) {
		return Video{}, ErrInvalidUploadID
	}
	video := Video{AuthorID: authorID, Title: title, UploadID: &req.UploadID, PlaybackURL: "/media/" + req.UploadID}
	if err := s.repo.PublishUpload(ctx, &video, req.UploadID); err != nil {
		return Video{}, err
	}
	return video, nil
}

// ListNewest 读取一页最新视频，并计算调用方是否还需要请求下一页。
func (s *Service) ListNewest(ctx context.Context, limit int, offset int) (ListResponse, error) {
	videos, err := s.repo.ListNewest(ctx, limit, offset)
	// 要求repository 实际读取limit+1条 因为单凭十条无法判断数据库中是否有11条 需要has_more
	if err != nil {
		return ListResponse{}, err
	}
	// 裁剪掉额外的那条
	hasMore := len(videos) > limit
	if hasMore {
		videos = videos[:limit]
	}
	// 返回给调用方
	return ListResponse{Items: videos, NextOffset: offset + len(videos), HasMore: hasMore}, nil
}

// Detail 查询指定视频，并把数据库的未找到结果转换为视频不存在的业务错误。
func (s *Service) Detail(ctx context.Context, videoID int64) (Video, error) {
	video, err := s.repo.FindByID(ctx, videoID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Video{}, ErrVideoNotFound
	}
	return video, err
}
