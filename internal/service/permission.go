package service

import (
	"errors"

	"gorm.io/gorm"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

var (
	ErrModuleNotFound = errors.New("module not found")
	ErrModuleExists   = errors.New("module already exists")
	ErrNoPermission   = errors.New("no permission")
)

type PermissionService struct {
	db *gorm.DB
}

func NewPermissionService(db *gorm.DB) *PermissionService {
	return &PermissionService{db: db}
}

// CreateModule 创建模块
func (s *PermissionService) CreateModule(name string, moduleType model.ModuleType, description string, publicRead, publicWrite bool) (*model.Module, error) {
	var existing model.Module
	if err := s.db.Where("name = ?", name).First(&existing).Error; err == nil {
		return nil, ErrModuleExists
	}

	module := &model.Module{
		Name:        name,
		Type:        moduleType,
		Description: description,
		PublicRead:  publicRead,
		PublicWrite: publicWrite,
	}

	if err := s.db.Create(module).Error; err != nil {
		return nil, err
	}

	return module, nil
}

// GetModule 获取模块
func (s *PermissionService) GetModule(name string) (*model.Module, error) {
	var module model.Module
	if err := s.db.Where("name = ?", name).First(&module).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModuleNotFound
		}
		return nil, err
	}
	return &module, nil
}

// GetModuleByID 根据 ID 获取模块
func (s *PermissionService) GetModuleByID(id uint) (*model.Module, error) {
	var module model.Module
	if err := s.db.First(&module, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrModuleNotFound
		}
		return nil, err
	}
	return &module, nil
}

// ListModules 列出所有模块
func (s *PermissionService) ListModules() ([]model.Module, error) {
	var modules []model.Module
	if err := s.db.Find(&modules).Error; err != nil {
		return nil, err
	}
	return modules, nil
}

// UpdateModule 更新模块
func (s *PermissionService) UpdateModule(id uint, updates map[string]interface{}) error {
	return s.db.Model(&model.Module{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteModule 删除模块
func (s *PermissionService) DeleteModule(id uint) error {
	return s.db.Delete(&model.Module{}, id).Error
}

// GrantPermission 授予用户权限
func (s *PermissionService) GrantPermission(userID, moduleID uint, canRead, canWrite bool) error {
	var perm model.Permission
	err := s.db.Where("user_id = ? AND module_id = ?", userID, moduleID).First(&perm).Error
	
	if err == nil {
		// 更新已有权限
		return s.db.Model(&perm).Updates(map[string]interface{}{
			"can_read":  canRead,
			"can_write": canWrite,
		}).Error
	}
	
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建新权限
		perm = model.Permission{
			UserID:   userID,
			ModuleID: moduleID,
			CanRead:  canRead,
			CanWrite: canWrite,
		}
		return s.db.Create(&perm).Error
	}
	
	return err
}

// RevokePermission 撤销用户权限
func (s *PermissionService) RevokePermission(userID, moduleID uint) error {
	return s.db.Where("user_id = ? AND module_id = ?", userID, moduleID).Delete(&model.Permission{}).Error
}

// GetUserPermission 获取用户对模块的权限
func (s *PermissionService) GetUserPermission(userID, moduleID uint) (*model.Permission, error) {
	var perm model.Permission
	if err := s.db.Where("user_id = ? AND module_id = ?", userID, moduleID).First(&perm).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &perm, nil
}

// CheckPermission 检查用户是否有权限
// 返回 (canRead, canWrite, error)
func (s *PermissionService) CheckPermission(userID uint, moduleName string, isAdmin bool) (bool, bool, error) {
	module, err := s.GetModule(moduleName)
	if err != nil {
		return false, false, err
	}

	// 管理员拥有所有权限
	if isAdmin {
		return true, true, nil
	}

	// 检查公有权限
	canRead := module.PublicRead
	canWrite := module.PublicWrite

	// 检查用户特定权限
	if userID > 0 {
		perm, err := s.GetUserPermission(userID, module.ID)
		if err != nil {
			return canRead, canWrite, err
		}
		if perm != nil {
			canRead = canRead || perm.CanRead
			canWrite = canWrite || perm.CanWrite
		}
	}

	return canRead, canWrite, nil
}

// GetUserPermissions 获取用户的所有权限
func (s *PermissionService) GetUserPermissions(userID uint) ([]model.Permission, error) {
	var perms []model.Permission
	if err := s.db.Preload("Module").Where("user_id = ?", userID).Find(&perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
}
