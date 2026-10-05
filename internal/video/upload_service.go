package video

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"gorm.io/gorm"
)

const MaxVideoBytes int64 = 32 * 1024 * 1024

var (
	ErrUploadTooLarge   = errors.New("video file exceeds 32 MiB")
	ErrInvalidVideoFile = errors.New("file must have an MP4 signature")
	ErrUploadNotFound   = errors.New("upload does not exist or does not belong to you")
	ErrUploadUsed       = errors.New("upload has already been published")
	uploadIDPattern     = regexp.MustCompile(`^[a-f0-9]{32}$`)
)

// SaveUpload 检查文件签名和大小，保存文件并记录当前用户的上传归属。
func (s *Service) SaveUpload(ctx context.Context, ownerID int64, source io.Reader) (UploadResponse, error) {
	reader := bufio.NewReader(source)
	header, err := reader.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) {
		return UploadResponse{}, err
	}
	if http.DetectContentType(header) != "video/mp4" {
		return UploadResponse{}, ErrInvalidVideoFile
	}
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return UploadResponse{}, err
	}
	id := hex.EncodeToString(key)
	if err := os.MkdirAll(s.uploadDir, 0755); err != nil {
		return UploadResponse{}, err
	}
	path := s.uploadPath(id)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return UploadResponse{}, err
	}
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	size, err := io.Copy(file, io.LimitReader(reader, MaxVideoBytes+1))
	if err != nil {
		return UploadResponse{}, err
	}
	if size > MaxVideoBytes {
		return UploadResponse{}, ErrUploadTooLarge
	}
	if err := file.Close(); err != nil {
		return UploadResponse{}, err
	}
	upload := Upload{ID: id, OwnerID: ownerID, SizeBytes: size, ContentType: "video/mp4"}
	if err := s.repo.CreateUpload(ctx, &upload); err != nil {
		return UploadResponse{}, err
	}
	keep = true
	return UploadResponse{Upload: upload, PlaybackURL: "/media/" + id}, nil
}

// Media 查找一个已上传文件，只有服务生成的合法编号才能构造磁盘路径。
func (s *Service) Media(ctx context.Context, id string) (Upload, string, error) {
	if !uploadIDPattern.MatchString(id) {
		return Upload{}, "", ErrUploadNotFound
	}
	upload, err := s.repo.FindUpload(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Upload{}, "", ErrUploadNotFound
	}
	return upload, s.uploadPath(id), err
}

// uploadPath 用服务生成的编号定位上传目录内的文件，不使用客户端文件名。
func (s *Service) uploadPath(id string) string {
	return filepath.Join(s.uploadDir, id+".mp4")
}
