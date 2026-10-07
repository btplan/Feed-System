package httpapi

import (
	"net/http"

	"clipflow/internal/account"
	"clipflow/internal/auth"
	"clipflow/internal/httpapi/middleware"
	"clipflow/internal/video"
	"github.com/gin-gonic/gin"
)

// NewRouter 集中配置 API 的 URL、公开接口和需要登录的接口。
func NewRouter(tokens *auth.TokenManager, accounts *account.Handler, videos *video.Handler) *gin.Engine {
	router := gin.Default()
	router.GET("/healthz", healthz)
	router.POST("/users/register", accounts.Register)
	router.POST("/users/login", accounts.Login)
	router.GET("/videos", videos.ListNewest)
	router.GET("/videos/:id", videos.Detail)

	protected := router.Group("/")
	protected.Use(middleware.RequireUser(tokens))
	protected.GET("/users/me", accounts.Profile)
	protected.POST("/videos", videos.Publish)
	protected.PUT("/videos/:id/like", videos.Like)
	protected.DELETE("/videos/:id/like", videos.Unlike)
	return router
}

// healthz 告诉调用者当前 API 服务正在正常运行。
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
