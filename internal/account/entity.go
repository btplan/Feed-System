package account

import "time"

// User 保存用户公开资料和加密后的密码，映射到 users 表。
type User struct {
	ID           int64     `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"not null;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"not null" json:"-"`
	CreatedAt    time.Time `gorm:"not null" json:"created_at"`
}

// RegisterRequest 定义注册接口接收的用户名和原始密码。
type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterResponse 定义注册成功后返回给客户端的安全用户资料。
type RegisterResponse struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginRequest 定义登录接口接收的用户名和原始密码。
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse 定义登录成功后返回的用户资料和身份令牌。
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	User        User   `json:"user"`
}
