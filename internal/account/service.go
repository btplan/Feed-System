package account

import (
	"context"
	"errors"
	"strings"

	"clipflow/internal/auth"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidUsername    = errors.New("username must be 3-30 characters and contain only letters, numbers, or underscores")
	ErrInvalidPassword    = errors.New("password must be 8-72 characters")
	ErrUsernameTaken      = errors.New("username is already taken")
	ErrInvalidCredentials = errors.New("username or password is incorrect")
	ErrUserNotFound       = errors.New("user does not exist")
)

// Service 执行注册业务的格式校验、密码哈希和唯一用户名规则。
type Service struct {
	repo   *Repository
	tokens *auth.TokenManager
}

// NewService 为注册和登录业务配置用户数据访问对象与令牌签发器。
func NewService(repo *Repository, tokens *auth.TokenManager) *Service {
	return &Service{repo: repo, tokens: tokens}
}

// Login 验证用户名和密码，并为验证通过的用户签发身份令牌。
func (s *Service) Login(ctx context.Context, req LoginRequest) (LoginResponse, error) {
	username := strings.TrimSpace(req.Username)
	user, err := s.repo.FindByUsername(ctx, username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return LoginResponse{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResponse{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return LoginResponse{}, ErrInvalidCredentials
	}
	token, err := s.tokens.Issue(user.ID, user.Username)
	if err != nil {
		return LoginResponse{}, err
	}
	return LoginResponse{AccessToken: token, TokenType: "Bearer", User: user}, nil
}

// Profile 读取已经通过令牌验证的当前用户资料。
func (s *Service) Profile(ctx context.Context, userID int64) (User, error) {
	user, err := s.repo.FindByID(ctx, userID)
	// 读取user_id，用repo的函数判断用户是否还存在。（用gorm用users表读取一行数据）
	// FindByID 返回用户资料
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return User{}, ErrUserNotFound
	}
	return user, err
}

// Register 校验注册资料、加密密码并创建用户。
func (s *Service) Register(ctx context.Context, req RegisterRequest) (RegisterResponse, error) {
	username := strings.TrimSpace(req.Username)
	if !validUsername(username) {
		return RegisterResponse{}, ErrInvalidUsername
	}
	if length := len(req.Password); length < 8 || length > 72 {
		return RegisterResponse{}, ErrInvalidPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return RegisterResponse{}, err
	}
	user := User{Username: username, PasswordHash: string(hash)}
	if err := s.repo.Create(ctx, &user); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
			return RegisterResponse{}, ErrUsernameTaken
		}
		return RegisterResponse{}, err
	}
	return RegisterResponse{ID: user.ID, Username: user.Username, CreatedAt: user.CreatedAt}, nil
}

// validUsername 判断用户名是否符合注册接口允许的 ASCII 格式。
func validUsername(username string) bool {
	if length := len(username); length < 3 || length > 30 {
		return false
	}
	for _, char := range []byte(username) {
		if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '_' {
			return false
		}
	}
	return true
}
