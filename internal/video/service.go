package video

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"gorm.io/gorm"
)

var (
	ErrInvalidTitle       = errors.New("title must be 1-100 characters")
	ErrInvalidPlaybackURL = errors.New("playback_url must be a valid http or https URL")
	ErrVideoNotFound      = errors.New("video does not exist")
)

// Service 执行发布视频需要的输入校验和视频创建业务规则。
type Service struct {
	repo *Repository
}

// NewService 为视频发布业务配置视频数据访问对象。
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Publish 校验视频资料，并将当前登录用户作为作者写入新视频。
func (s *Service) Publish(ctx context.Context, authorID int64, req PublishRequest) (Video, error) {
	title := strings.TrimSpace(req.Title)
	if length := len([]rune(title)); length < 1 || length > 100 {
		return Video{}, ErrInvalidTitle
	}
	playbackURL := strings.TrimSpace(req.PlaybackURL)
	if !validPlaybackURL(playbackURL) {
		return Video{}, ErrInvalidPlaybackURL
	}
	video := Video{AuthorID: authorID, Title: title, PlaybackURL: playbackURL}
	if err := s.repo.Create(ctx, &video); err != nil {
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

// validPlaybackURL 判断播放地址是否是带主机名的 http 或 https 地址。
func validPlaybackURL(rawURL string) bool {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false
	}
	return parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

// Detail 查询指定视频，并把数据库的未找到结果转换为视频不存在的业务错误。
func (s *Service) Detail(ctx context.Context, videoID int64) (Video, error) {
	video, err := s.repo.FindByID(ctx, videoID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Video{}, ErrVideoNotFound
	}
	return video, err
}
