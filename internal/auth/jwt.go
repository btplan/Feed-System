package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenIssuer    = "clipflow"
	tokenAudience  = "clipflow-api"
	accessTokenTTL = 15 * time.Minute
)

// TokenManager 使用启动时注入的密钥签发和验证访问令牌。
type TokenManager struct {
	secret []byte
}

// Claims 保存 Token 中需要带给后续受保护接口的用户身份。
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// NewTokenManager 将 JWT 签名密钥保存到令牌签发器中。
func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secret: []byte(secret)}
}

// Issue 为验证通过的用户签发有效期 15 分钟且限定用途的 JWT。
func (m *TokenManager) Issue(userID int64, username string) (string, error) {
	if userID <= 0 || strings.TrimSpace(username) == "" {
		return "", fmt.Errorf("invalid token identity")
	}
	now := time.Now()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Audience:  jwt.ClaimStrings{tokenAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

// Parse 验证调用方提交的 JWT 签名和有效期，并取回其中的用户身份。
func (m *TokenManager) Parse(accessToken string) (Claims, error) {
	claims := Claims{}
	// 此回调只向 JWT 库提供服务端密钥，允许的算法由下面的选项限定。
	token, err := jwt.ParseWithClaims(accessToken, &claims, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(), jwt.WithIssuedAt(),
		jwt.WithIssuer(tokenIssuer), jwt.WithAudience(tokenAudience))
	if err != nil {
		return Claims{}, fmt.Errorf("parse jwt: %w", err)
	}
	if !token.Valid {
		return Claims{}, fmt.Errorf("jwt is invalid")
	}
	return claims, nil
}

// Validate 补充检查令牌的用户身份、必需签发时间和允许的最长有效期。
func (c Claims) Validate() error {
	if c.UserID <= 0 || strings.TrimSpace(c.Username) == "" {
		return fmt.Errorf("invalid token identity")
	}
	if c.IssuedAt == nil || c.ExpiresAt == nil {
		return fmt.Errorf("iat and exp are required")
	}
	lifetime := c.ExpiresAt.Sub(c.IssuedAt.Time)
	if lifetime <= 0 || lifetime > accessTokenTTL {
		return fmt.Errorf("invalid access token lifetime")
	}
	return nil
}
