package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gitea.loveuer.com/loveuer/ufshare/v2/internal/model"
)

func setupUserService(t *testing.T) (*UserService, *AuthService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewUserService(db), NewAuthService(db, "test-secret", 3600)
}

// 创建普通用户并返回其 ID
func createNormalUser(t *testing.T, svc *UserService, username string) uint {
	t.Helper()
	u, err := svc.CreateUser(username, "password123", username+"@test.com", false)
	if err != nil {
		t.Fatalf("createNormalUser(%q): %v", username, err)
	}
	return u.ID
}

// 创建管理员用户并返回其 ID
func createAdminUser(t *testing.T, svc *UserService, username string) uint {
	t.Helper()
	u, err := svc.CreateUser(username, "password123", username+"@test.com", true)
	if err != nil {
		t.Fatalf("createAdminUser(%q): %v", username, err)
	}
	return u.ID
}

// ─── DeleteUser ──────────────────────────────────────────────────────────────

func TestDeleteUser_Normal(t *testing.T) {
	svc, _ := setupUserService(t)
	adminID := createAdminUser(t, svc, "admin")
	userID := createNormalUser(t, svc, "alice")

	if err := svc.DeleteUser(adminID, userID); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// 确认用户已被删除
	if _, err := svc.GetUser(userID); err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound after delete, got: %v", err)
	}
}

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	svc, _ := setupUserService(t)
	adminID := createAdminUser(t, svc, "admin")

	err := svc.DeleteUser(adminID, adminID)
	if err != ErrCannotDeleteSelf {
		t.Errorf("expected ErrCannotDeleteSelf, got: %v", err)
	}
}

func TestDeleteUser_CannotDeleteAdmin(t *testing.T) {
	svc, _ := setupUserService(t)
	adminID := createAdminUser(t, svc, "admin")
	otherAdminID := createAdminUser(t, svc, "admin2")

	err := svc.DeleteUser(adminID, otherAdminID)
	if err != ErrCannotDeleteAdmin {
		t.Errorf("expected ErrCannotDeleteAdmin, got: %v", err)
	}
}

func TestDeleteUser_CannotDeleteSoleAdmin(t *testing.T) {
	// 只有一个 admin，尝试用自身删自身 → ErrCannotDeleteSelf
	svc, _ := setupUserService(t)
	adminID := createAdminUser(t, svc, "admin")

	err := svc.DeleteUser(adminID, adminID)
	if err != ErrCannotDeleteSelf {
		t.Errorf("expected ErrCannotDeleteSelf, got: %v", err)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	svc, _ := setupUserService(t)
	adminID := createAdminUser(t, svc, "admin")

	err := svc.DeleteUser(adminID, 99999)
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got: %v", err)
	}
}

func TestDeleteUser_AdminCanDeleteNonAdmin(t *testing.T) {
	// 确认管理员能正常删除普通用户
	svc, _ := setupUserService(t)
	adminID := createAdminUser(t, svc, "admin")
	u1 := createNormalUser(t, svc, "bob")
	u2 := createNormalUser(t, svc, "carol")

	if err := svc.DeleteUser(adminID, u1); err != nil {
		t.Errorf("delete bob: %v", err)
	}
	if err := svc.DeleteUser(adminID, u2); err != nil {
		t.Errorf("delete carol: %v", err)
	}

	if _, err := svc.GetUser(u1); err != ErrUserNotFound {
		t.Errorf("bob should be deleted")
	}
	if _, err := svc.GetUser(u2); err != ErrUserNotFound {
		t.Errorf("carol should be deleted")
	}
}

func TestDeleteUser_AdminCanBeDeletedAfterRevoke(t *testing.T) {
	// 撤销管理员权限后可以被删除
	svc, _ := setupUserService(t)
	adminID := createAdminUser(t, svc, "admin")
	targetID := createAdminUser(t, svc, "admin2")

	// 撤销 admin2 的管理员权限
	if err := svc.SetAdmin(targetID, false); err != nil {
		t.Fatalf("revoke admin: %v", err)
	}

	// 现在可以删除
	if err := svc.DeleteUser(adminID, targetID); err != nil {
		t.Errorf("expected success after revoke, got: %v", err)
	}
}
