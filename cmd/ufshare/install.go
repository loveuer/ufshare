package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

const systemdServiceTemplate = `[Unit]
Description=UFShare - Artifact Repository Manager
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ufshare --data=%s --address=%s
Restart=always
RestartSec=5
User=root
WorkingDirectory=%s
Environment="JWT_SECRET=%s"
Environment="BODY_SIZE=1GB"

[Install]
WantedBy=multi-user.target
`

func newInstallCmd() *cobra.Command {
	var (
		dataDir string
		address string
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "安装 ufshare 到系统服务",
		Long: `将 ufshare 安装为系统服务:
  1. 拷贝可执行文件到 /usr/local/bin
  2. 创建 systemd 服务文件 /etc/systemd/system/ufshare.service
  3. 启动并启用服务`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkRoot(); err != nil {
				return err
			}
			return runInstall(dataDir, address)
		},
	}

	cmd.Flags().StringVar(&dataDir, "data", "/data/ufshare", "数据目录")
	cmd.Flags().StringVar(&address, "address", "0.0.0.0:9817", "监听地址")

	return cmd
}

func runInstall(dataDir, address string) error {
	// 获取当前可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	// 确保是绝对路径
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("解析可执行文件路径失败: %w", err)
	}

	fmt.Printf("安装 ufshare...\n")
	fmt.Printf("  数据目录: %s\n", dataDir)
	fmt.Printf("  监听地址: %s\n", address)
	fmt.Println()

	// 1. 创建数据目录
	fmt.Println("[1/5] 创建数据目录...")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	fmt.Printf("      已创建: %s\n", dataDir)

	// 2. 拷贝可执行文件
	fmt.Println("[2/5] 拷贝可执行文件到 /usr/local/bin...")
	targetPath := "/usr/local/bin/ufshare"
	if err := copyFile(exePath, targetPath); err != nil {
		return fmt.Errorf("拷贝文件失败: %w", err)
	}
	fmt.Printf("      已拷贝: %s\n", targetPath)

	// 生成随机 JWT_SECRET
	jwtSecret, err := generateRandomSecret(32)
	if err != nil {
		return fmt.Errorf("生成 JWT_SECRET 失败: %w", err)
	}

	// 3. 创建 systemd 服务文件
	fmt.Println("[3/5] 创建 systemd 服务文件...")
	serviceContent := fmt.Sprintf(systemdServiceTemplate, dataDir, address, dataDir, jwtSecret)
	servicePath := "/etc/systemd/system/ufshare.service"
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("创建服务文件失败: %w", err)
	}
	fmt.Printf("      已创建: %s\n", servicePath)

	// 4. 重新加载 systemd
	fmt.Println("[4/5] 重新加载 systemd...")
	if output, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload 失败: %w\n%s", err, string(output))
	}
	fmt.Println("      已重新加载")

	// 5. 启用并启动服务
	fmt.Println("[5/5] 启用并启动服务...")
	if output, err := exec.Command("systemctl", "enable", "ufshare", "--now").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable --now 失败: %w\n%s", err, string(output))
	}
	fmt.Println("      服务已启用并启动")

	fmt.Println()
	fmt.Println("====================================")
	fmt.Println("✓ 安装完成!")
	fmt.Println("====================================")
	fmt.Println()
	fmt.Printf("访问地址: http://%s\n", address)
	fmt.Println()
	fmt.Println("常用命令:")
	fmt.Println("  systemctl status ufshare    # 查看服务状态")
	fmt.Println("  systemctl stop ufshare      # 停止服务")
	fmt.Println("  systemctl start ufshare     # 启动服务")
	fmt.Println("  systemctl restart ufshare   # 重启服务")
	fmt.Println("  systemctl disable ufshare   # 禁用开机自启")
	fmt.Println()
	fmt.Println("日志查看:")
	fmt.Println("  journalctl -u ufshare -f    # 实时查看日志")
	fmt.Println("  journalctl -u ufshare -n 100 # 查看最近100条日志")
	fmt.Println()

	return nil
}

func copyFile(src, dst string) error {
	// 读取源文件
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// 写入目标文件
	if err := os.WriteFile(dst, data, 0755); err != nil {
		return err
	}

	return nil
}

// isRoot 检查是否以 root 权限运行
func isRoot() bool {
	return os.Getuid() == 0
}

// checkRoot 检查 root 权限并提示
func checkRoot() error {
	if !isRoot() {
		return fmt.Errorf("需要 root 权限运行此命令，请使用 sudo")
	}
	return nil
}

// generateRandomSecret 生成随机十六进制密钥
func generateRandomSecret(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
