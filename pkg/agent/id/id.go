package id

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/pika-monitor/pika/pkg/agent/utils"
)

// Manager 管理探针的唯一标识
type Manager struct {
	idFilePath string
}

// NewManager 创建 ID 管理器
func NewManager() *Manager {
	return &Manager{
		idFilePath: GetIDFilePath(),
	}
}

func oldGetIDFilePath() string {
	// 获取用户主目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// 如果无法获取主目录，使用当前目录
		homeDir = "."
	}

	// 统一使用 ~/.pika/agent.id
	return filepath.Join(homeDir, ".pika", "agent.id")
}

// GetIDFilePath 获取 ID 文件路径
func GetIDFilePath() string {
	// 获取用户主目录
	var homeDir = utils.GetSafeHomeDir()
	// 统一使用 ~/.pika/agent.id
	return filepath.Join(homeDir, ".pika", "agent.id")
}

// Load 加载或生成探针 ID
// 如果 ID 文件存在，则读取；否则生成新的 UUID 并保存
func (m *Manager) Load() (string, error) {
	// 尝试从旧路径迁移
	if err := m.migrateFromOldPath(); err != nil {
		// 迁移失败不影响后续流程，仅记录错误
		fmt.Printf("警告: 迁移旧 ID 文件失败: %v\n", err)
	}

	// 尝试读取现有 ID
	if id, err := m.read(); err == nil && id != "" {
		return id, nil
	}

	// 生成新 ID
	id := uuid.NewString()

	// 保存 ID
	if err := m.save(id); err != nil {
		return "", fmt.Errorf("保存 agent ID 失败: %w", err)
	}

	return id, nil
}

// read 读取 ID 文件
func (m *Manager) read() (string, error) {
	data, err := os.ReadFile(m.idFilePath)
	if err != nil {
		return "", err
	}

	id := strings.TrimSpace(string(data))
	if id == "" {
		return "", fmt.Errorf("ID 文件为空")
	}

	return id, nil
}

// save 保存 ID 到文件
func (m *Manager) save(id string) error {
	// 确保目录存在
	dir := filepath.Dir(m.idFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入 ID 文件
	if err := os.WriteFile(m.idFilePath, []byte(id), 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}

	return nil
}

// GetPath 获取 ID 文件路径
func (m *Manager) GetPath() string {
	return m.idFilePath
}

// Exists 检查 ID 文件是否存在
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.idFilePath)
	return err == nil
}

// Delete 删除 ID 文件
func (m *Manager) Delete() error {
	return os.Remove(m.idFilePath)
}

// migrateFromOldPath 从旧路径迁移 ID 文件到新路径
func (m *Manager) migrateFromOldPath() error {
	oldPath := oldGetIDFilePath()
	newPath := m.idFilePath

	// 如果旧路径和新路径相同，无需迁移
	if oldPath == newPath {
		return nil
	}

	// 检查旧文件是否存在
	oldInfo, err := os.Stat(oldPath)
	if err != nil {
		// 旧文件不存在，无需迁移
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("检查旧 ID 文件失败: %w", err)
	}

	// 确保新路径的目录存在
	newDir := filepath.Dir(newPath)
	if err := os.MkdirAll(newDir, 0755); err != nil {
		return fmt.Errorf("创建新目录失败: %w", err)
	}

	// 强制迁移：读取旧文件内容
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return fmt.Errorf("读取旧 ID 文件失败: %w", err)
	}

	// 写入新路径（覆盖已存在的文件）
	if err := os.WriteFile(newPath, data, oldInfo.Mode()); err != nil {
		return fmt.Errorf("写入新 ID 文件失败: %w", err)
	}

	// 删除旧文件
	if err := os.Remove(oldPath); err != nil {
		fmt.Printf("警告: 删除旧 ID 文件失败: %v\n", err)
	}

	fmt.Printf("已迁移 ID 文件: %s -> %s\n", oldPath, newPath)
	return nil
}
