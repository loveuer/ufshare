package service

import (
	"context"
	"errors"

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
func (s *UserService) ListUsers(ctx context.Context, page, pageSize int) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	if err := s.db.WithContext(ctx).Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := s.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetUser 获取用户
func (s *UserService) GetUser(ctx context.Context, id uint) (*model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

// UpdateUser 更新用户
func (s *UserService) UpdateUser(ctx context.Context, id uint, updates map[string]interface{}) error {
	if pwd, ok := updates["password"]; ok {
		hashed, err := hashPassword(pwd.(string))
		if err != nil {
			return err
		}
		updates["password"] = hashed
	}
	return s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteUser 删除用户。callerID 为当前操作者 ID。
// 禁止：自删、删除管理员用户（需先撤销管理员权限）。
func (s *UserService) DeleteUser(ctx context.Context, callerID, targetID uint) error {
	if callerID == targetID {
		return ErrCannotDeleteSelf
	}

	var target model.User
	if err := s.db.WithContext(ctx).First(&target, targetID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}

	if target.IsAdmin {
		return ErrCannotDeleteAdmin
	}

	return s.db.WithContext(ctx).Delete(&model.User{}, targetID).Error
}

// CreateUser 创建用户
func (s *UserService) CreateUser(ctx context.Context, username, password, email string, isAdmin bool) (*model.User, error) {
	hashed, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username: username,
		Password: hashed,
		Email:    email,
		IsAdmin:  isAdmin,
		Status:   1,
	}

	if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
		return nil, err
	}

	return user, nil
}

// ResetPassword 管理员直接重置用户密码（无需旧密码）
func (s *UserService) ResetPassword(ctx context.Context, id uint, newPassword string) error {
	hashed, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	result := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("password", hashed)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (s *UserService) SetAdmin(ctx context.Context, id uint, isAdmin bool) error {
	return s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("is_admin", isAdmin).Error
}

// SetStatus 设置用户状态
func (s *UserService) SetStatus(ctx context.Context, id uint, status int) error {
	return s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Update("status", status).Error
}
