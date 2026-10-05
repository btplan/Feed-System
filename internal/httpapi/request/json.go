package request

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const MaxJSONBytes int64 = 64 * 1024

// BindJSON 限量读取完整 JSON 请求，超限返回 413，格式或字段绑定失败返回 400。
func BindJSON(c *gin.Context, target any, invalidMessage string) bool {
	reader := http.MaxBytesReader(c.Writer, c.Request.Body, MaxJSONBytes)
	defer reader.Close()
	data, err := io.ReadAll(reader)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": "JSON request body exceeds 65536 bytes"})
		return false
	}
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": invalidMessage})
		return false
	}
	if err := json.Unmarshal(data, target); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": invalidMessage})
		return false
	}
	if err := binding.Validator.ValidateStruct(target); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": invalidMessage})
		return false
	}
	return true
}
