package video

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"clipflow/internal/httpapi/middleware"

	"github.com/gin-gonic/gin"
)

// Handler 将发布视频的 HTTP 请求交给视频业务，并返回 JSON 响应。
type Handler struct {
	service *Service
}

// NewHandler 为视频 HTTP 接口配置已经组装好的视频业务服务。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListNewest 读取分页参数并返回任何调用方都能浏览的最新视频列表。
func (h *Handler) ListNewest(c *gin.Context) {
	limit, offset, err := listPage(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 拿到service.listNewesr=t的返回结果 listresponse
	response, err := h.service.ListNewest(c.Request.Context(), limit, offset)
	if err != nil {
		log.Printf("list newest videos: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, response)
}

// Publish 读取视频发布资料，用 Token 中的用户 ID 创建一条视频。
func (h *Handler) Publish(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization token is required or invalid"})
		return
	}
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request body must contain valid JSON title and playback_url"})
		return
	}
	video, err := h.service.Publish(c.Request.Context(), userID, req)
	switch {
	case errors.Is(err, ErrInvalidTitle), errors.Is(err, ErrInvalidPlaybackURL):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case err != nil:
		log.Printf("publish video: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusCreated, video)
	}
}

// listPage 检查视频列表的 limit 和 offset，并为缺省参数提供安全默认值。
// 读取并检查一下参数 调用video.service.listnewest
func listPage(c *gin.Context) (int, int, error) {
	limit := 10
	offset := 0
	if rawLimit := c.Query("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 50 {
			return 0, 0, errors.New("limit must be an integer between 1 and 50")
		}
		limit = parsed
	}
	if rawOffset := c.Query("offset"); rawOffset != "" {
		parsed, err := strconv.Atoi(rawOffset)
		if err != nil || parsed < 0 {
			return 0, 0, errors.New("offset must be a non-negative integer")
		}
		offset = parsed
	}
	return limit, offset, nil
}

// Detail 读取路径中的视频编号，并向游客返回视频资料或对应的错误响应。
func (h *Handler) Detail(c *gin.Context) {
	videoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || videoID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "video id must be a positive int64"})
		return
	}
	video, err := h.service.Detail(c.Request.Context(), videoID)
	switch {
	case errors.Is(err, ErrVideoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case err != nil:
		log.Printf("read video detail: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusOK, video)
	}
}
