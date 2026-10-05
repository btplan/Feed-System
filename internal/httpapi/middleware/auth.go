package middleware

import (
	"net/http"
	"strings"

	"clipflow/internal/auth"

	"github.com/gin-gonic/gin"
)

const userIDContextKey = "authenticated_user_id"

// RequireUser 拦截受保护请求，验证 Bearer Token 后把用户 ID 放进本次请求上下文。
func RequireUser(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization") // 读取
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization token is required or invalid"})
			return
		} // 确认以bearer开头
		// 调用parse验证签名 算法 过期时间
		claims, err := tokens.Parse(strings.TrimPrefix(header, "Bearer "))
		// claim：jwt里携带的一项身份数据 登录成功时 jwt.go里面的issue会把userid username 过期时间、签发时间写进token
		// 连起来就是claims
		// 在jwt.go中有结构体
		if err != nil || claims.UserID <= 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization token is required or invalid"})
			return
		}
		// 将yoken claims的user_id 写入上下文
		c.Set(userIDContextKey, claims.UserID)
		c.Next()
	}
}

// CurrentUserID 从已通过身份验证的请求上下文中读取当前用户 ID。
func CurrentUserID(c *gin.Context) (int64, bool) {
	value, ok := c.Get(userIDContextKey)
	if !ok {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok
}
