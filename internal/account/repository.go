package account

import (
	"context"

	"gorm.io/gorm"
)

// Repository 负责 users 表的读写操作。
type Repository struct {
	db *gorm.DB
}

// NewRepository 将数据库连接交给用户数据访问对象。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Create 将已经准备好的用户记录写入 users 表。
func (r *Repository) Create(ctx context.Context, user *User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindByUsername 按用户名读取一名用户，以便登录时取得密码哈希。
func (r *Repository) FindByUsername(ctx context.Context, username string) (User, error) {
	var user User
	err := r.db.WithContext(ctx).Where("username = ?", username).First(&user).Error
	return user, err
}

// FindByID 按用户 ID 读取当前登录用户的资料。
func (r *Repository) FindByID(ctx context.Context, userID int64) (User, error) {
	var user User
	err := r.db.WithContext(ctx).First(&user, userID).Error
	return user, err
}

// Migrate 创建或补齐注册业务需要的 users 表和用户名唯一索引。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{})
}
