package video

import (
	"errors"
	"log"
	"net/http"

	"clipflow/internal/httpapi/middleware"
	"github.com/gin-gonic/gin"
)

// Upload 接收单个 multipart 文件，将已验证身份和文件内容交给上传业务。
func (h *Handler) Upload(c *gin.Context) {
	ownerID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization token is required or invalid"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxVideoBytes+1024*1024)
	defer c.Request.Body.Close()
	err := c.Request.ParseMultipartForm(1024 * 1024)
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "multipart request exceeds 33 MiB"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "send multipart/form-data with one file field"})
		return
	}
	files := c.Request.MultipartForm.File
	if len(files) != 1 || len(files["file"]) != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "send exactly one file in field file"})
		return
	}
	header := files["file"][0]
	if header.Size > MaxVideoBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": ErrUploadTooLarge.Error()})
		return
	}
	file, err := header.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot open uploaded file"})
		return
	}
	defer file.Close()
	result, err := h.service.SaveUpload(c.Request.Context(), ownerID, file)
	switch {
	case errors.Is(err, ErrInvalidVideoFile):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrUploadTooLarge):
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": err.Error()})
	case err != nil:
		log.Printf("upload video: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusCreated, result)
	}
}

// Media 允许通过上传编号读取文件，并由标准文件响应支持浏览器的 Range 分段请求。
func (h *Handler) Media(c *gin.Context) {
	upload, path, err := h.service.Media(c.Request.Context(), c.Param("id"))
	switch {
	case errors.Is(err, ErrUploadNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "upload does not exist"})
		return
	case err != nil:
		log.Printf("read upload: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Header("Content-Type", upload.ContentType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.File(path)
}
