package video

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"clipflow/internal/httpapi/middleware"

	"github.com/gin-gonic/gin"
)

// Like 将当前用户对指定视频的状态设置为已点赞。
func (h *Handler) Like(c *gin.Context) {
	h.setLike(c, true)
}

// Unlike 将当前用户对指定视频的状态设置为未点赞。
func (h *Handler) Unlike(c *gin.Context) {
	h.setLike(c, false)
}

// setLike 读取可信用户身份和视频编号，调用业务并返回点赞操作结果。
func (h *Handler) setLike(c *gin.Context, liked bool) {
	userID, ok := middleware.CurrentUserID(c) // 通过中间件读出当前的用户ID
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization token is required or invalid"})
		return
	}
	videoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || videoID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "video id must be a positive int64"})
		return
	}
	response, err := h.service.SetLike(c.Request.Context(), userID, videoID, liked)
	// 处理报错
	switch {
	case errors.Is(err, ErrLikeUserNotFound):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization token is required or invalid"})
	case errors.Is(err, ErrVideoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case err != nil:
		log.Printf("set video like: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusOK, response)
	}
}
