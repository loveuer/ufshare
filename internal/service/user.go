package service

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// ListUsers 列出所有用户
func (s *UserService) ListUsers(page, pageSize int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	s.db.Model(&model.User{}).Count(&total)

	offset := (page - 1) * pageSize
	if err := s.db.Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetUser 获取用户
func (s *UserService) GetUser(id uint) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// UpdateUser 更新用户
func (s *UserService) UpdateUser(id uint, updates map[string]interface{}) error {
	// 如果更新密码，需要加密
	if pwd, ok := updates["password"]; ok {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(pwd.(string)), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		updates["password"] = string(hashedPassword)
	}

	return s.db.Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(id uint) error {
	return s.db.Delete(&model.User{}, id).Error
}

// CreateUser 创建用户
func (s *UserService) CreateUser(username, password, email string, isAdmin bool) (*model.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username: username,
		Password: string(hashedPassword),
		Email:    email,
		IsAdmin:  isAdmin,
		Status:   1,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// SetAdmin 设置管理员状态
func (s *UserService) SetAdmin(id uint, isAdmin bool) error {
	return s.db.Model(&model.User{}).Where("id = ?", id).Update("is_admin", isAdmin).Error
}

// SetStatus 设置用户状态
func (s *UserService) SetStatus(id uint, status int) error {
	return s.db.Model(&model.User{}).Where("id = ?", id).Update("status", status).Error
}
