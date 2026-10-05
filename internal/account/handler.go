package account

import (
	"errors"
	"log"
	"net/http"

	"clipflow/internal/httpapi/middleware"

	"github.com/gin-gonic/gin"
)

// Handler 将用户注册 HTTP 请求交给注册业务，并返回 JSON 响应。
type Handler struct {
	service *Service
}

// NewHandler 为账号 HTTP 接口配置已经组装好的账号业务服务。
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Login 读取登录 JSON，验证用户名和密码，并返回身份令牌。
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request body must contain valid JSON username and password"})
		return
	}
	response, err := h.service.Login(c.Request.Context(), req)
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case err != nil:
		log.Printf("login user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusOK, response)
	}
}

// Profile 返回 Bearer Token 所代表的当前用户资料。
func (h *Handler) Profile(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization token is required or invalid"})
		return
	}
	// 读取上下文的ID，后面即使客户端添加也不读取
	user, err := h.service.Profile(c.Request.Context(), userID)
	switch {
	case errors.Is(err, ErrUserNotFound):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization token is required or invalid"})
	case err != nil:
		log.Printf("read current user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusOK, user)
	}
}

// Register 读取注册 JSON，调用注册业务，并按结果返回状态码。
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request body must contain valid JSON username and password"})
		return
	}
	response, err := h.service.Register(c.Request.Context(), req)
	switch {
	case errors.Is(err, ErrInvalidUsername), errors.Is(err, ErrInvalidPassword):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrUsernameTaken):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil:
		log.Printf("register user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	default:
		c.JSON(http.StatusCreated, response)
	}
}
