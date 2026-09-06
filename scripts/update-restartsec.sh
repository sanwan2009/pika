#!/bin/bash

# Pika Agent RestartSec 升级脚本
# 用途: 将 systemd 服务的 RestartSec 从 120 改为 5
# 使用: curl -fsSL https://raw.githubusercontent.com/dushixiang/pika/main/scripts/update-restartsec.sh | sudo bash

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

SERVICE_NAME="pika-agent"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

echo "================================================"
echo "Pika Agent RestartSec 升级脚本"
echo "================================================"
echo ""

# 检查是否以 root 权限运行
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}错误: 请使用 root 权限运行此脚本${NC}"
    echo "使用方法: curl -fsSL https://raw.githubusercontent.com/dushixiang/pika/main/scripts/update-restartsec.sh | sudo bash"
    exit 1
fi

# 检查服务文件是否存在
if [ ! -f "$SERVICE_FILE" ]; then
    echo -e "${RED}错误: 服务文件不存在: $SERVICE_FILE${NC}"
    echo "请先安装 pika-agent 服务"
    exit 1
fi

echo -e "${YELLOW}[1/3] 检查当前配置...${NC}"
# 检查当前的 RestartSec 值
CURRENT_RESTART_SEC=$(grep -oP '^\s*RestartSec=\K\d+' "$SERVICE_FILE" || echo "")

if [ -z "$CURRENT_RESTART_SEC" ]; then
    echo -e "${YELLOW}警告: 未找到 RestartSec 配置${NC}"
else
    echo "当前 RestartSec = $CURRENT_RESTART_SEC 秒"
fi

# 如果已经是 5，则无需修改
if [ "$CURRENT_RESTART_SEC" = "5" ]; then
    echo -e "${GREEN}✓ RestartSec 已经是 5 秒，无需修改${NC}"
    exit 0
fi

echo ""
echo -e "${YELLOW}[2/3] 修改 RestartSec 配置...${NC}"
# 修改 RestartSec 为 5
if grep -q "^\s*RestartSec=" "$SERVICE_FILE"; then
    # 如果存在 RestartSec，则替换
    sed -i 's/^\(\s*\)RestartSec=.*/\1RestartSec=5/' "$SERVICE_FILE"
    echo "已将 RestartSec 修改为 5 秒"
else
    # 如果不存在 RestartSec，则在 Restart=always 后添加
    sed -i '/^\s*Restart=always/a RestartSec=5' "$SERVICE_FILE"
    echo "已添加 RestartSec=5 配置"
fi

echo ""
echo -e "${YELLOW}[3/4] 重新加载 systemd 配置...${NC}"
# 重新加载 systemd 配置
systemctl daemon-reload
echo "systemd 配置已重新加载"

# 验证修改后的值
NEW_RESTART_SEC=$(grep -oP '^\s*RestartSec=\K\d+' "$SERVICE_FILE")

if [ "$NEW_RESTART_SEC" != "5" ]; then
    echo ""
    echo "================================================"
    echo -e "${RED}✗ 配置修改失败${NC}"
    echo "================================================"
    echo ""
    echo "当前 RestartSec = $NEW_RESTART_SEC 秒"
    echo "请检查服务文件: cat $SERVICE_FILE"
    exit 1
fi

echo ""
echo -e "${YELLOW}[4/4] 重启服务以应用新配置...${NC}"
# 重启服务
if systemctl restart $SERVICE_NAME; then
    echo "服务已重启"

    # 等待一下，确保服务启动
    sleep 2

    # 检查服务状态
    if systemctl is-active --quiet $SERVICE_NAME; then
        echo -e "${GREEN}✓ 服务运行正常${NC}"
    else
        echo -e "${YELLOW}警告: 服务可能未正常启动，请检查状态${NC}"
        echo "执行: systemctl status $SERVICE_NAME"
    fi
else
    echo -e "${RED}✗ 服务重启失败${NC}"
    echo "请手动检查: systemctl status $SERVICE_NAME"
    exit 1
fi

echo ""
echo "================================================"
echo -e "${GREEN}✓ 升级完成！${NC}"
echo "================================================"
echo ""
echo "修改后 RestartSec = $NEW_RESTART_SEC 秒"
echo "服务已自动重启并应用新配置"
echo ""
