package v0_1_2

import (
	"strings"

	"github.com/pika-monitor/pika/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func Migrate(logger *zap.Logger, db *gorm.DB) error {
	// 移除 ddns_configs 表的 enable_ipv4,enable_ipv6 字段的默认值

	logger.Info("开始执行 v0.1.2 版本数据迁移")

	migrator := db.Migrator()
	if migrator == nil {
		logger.Warn("无法获取数据库 migrator，跳过迁移")
		return nil
	}

	if !migrator.HasTable("ddns_configs") {
		logger.Info("未检测到 ddns_configs 表，跳过迁移")
		return nil
	}

	switch db.Dialector.Name() {
	case "postgres":
		if err := dropPostgresDefaults(logger, db, migrator); err != nil {
			return err
		}
	case "sqlite":
		if err := rebuildSQLiteDDNSConfigs(logger, db, migrator); err != nil {
			return err
		}
	default:
		logger.Warn("未知数据库类型，跳过移除默认值")
	}

	logger.Info("v0.1.2 版本数据迁移完成")
	return nil
}

func dropPostgresDefaults(logger *zap.Logger, db *gorm.DB, migrator gorm.Migrator) error {
	columns := []string{"enable_ipv4", "enable_ipv6"}
	for _, column := range columns {
		if !migrator.HasColumn("ddns_configs", column) {
			logger.Info("未检测到字段，跳过处理", zap.String("column", column))
			continue
		}
		sql := "ALTER TABLE ddns_configs ALTER COLUMN " + column + " DROP DEFAULT"
		if err := db.Exec(sql).Error; err != nil {
			logger.Error("移除字段默认值失败", zap.String("column", column), zap.Error(err))
			return err
		}
		logger.Info("已移除字段默认值", zap.String("column", column))
	}
	return nil
}

func rebuildSQLiteDDNSConfigs(logger *zap.Logger, db *gorm.DB, migrator gorm.Migrator) error {
	tempTable := "ddns_configs_backup_v0_1_2"
	if migrator.HasTable(tempTable) {
		logger.Warn("检测到残留备份表，跳过重建以避免覆盖", zap.String("table", tempTable))
		return nil
	}

	logger.Info("SQLite 不支持直接删除默认值，开始重建 ddns_configs 表")
	if err := db.Exec("ALTER TABLE ddns_configs RENAME TO " + tempTable).Error; err != nil {
		logger.Error("重命名 ddns_configs 表失败", zap.Error(err))
		return err
	}

	if err := db.AutoMigrate(&models.DDNSConfig{}); err != nil {
		logger.Error("重建 ddns_configs 表失败", zap.Error(err))
		return err
	}

	columns := []string{
		"id",
		"agent_id",
		"name",
		"enabled",
		"provider",
		"domains_ipv4",
		"domains_ipv6",
		"enable_ipv4",
		"enable_ipv6",
		"ipv4_get_method",
		"ipv6_get_method",
		"ipv4_get_value",
		"ipv6_get_value",
		"created_at",
		"updated_at",
	}
	columnList := strings.Join(columns, ", ")
	copySQL := "INSERT INTO ddns_configs (" + columnList + ") SELECT " + columnList + " FROM " + tempTable
	if err := db.Exec(copySQL).Error; err != nil {
		logger.Error("迁移 ddns_configs 数据失败", zap.Error(err))
		return err
	}

	if err := migrator.DropTable(tempTable); err != nil {
		logger.Error("删除备份表失败", zap.Error(err))
		return err
	}

	logger.Info("ddns_configs 表重建完成")
	return nil
}
