package sshmonitor

import (
	"encoding/json"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pika-monitor/pika/internal/protocol"
)

// SendEventFromEnv 从 PAM 环境变量构建并发送事件
func SendEventFromEnv() error {
	if os.Getenv("PAM_TYPE") != "open_session" {
		return nil
	}
	event := BuildEventFromEnv()
	return SendEvent(event)
}

// BuildEventFromEnv 从 PAM 环境变量构建事件
func BuildEventFromEnv() protocol.SSHLoginEvent {
	now := time.Now().UnixMilli()

	username := os.Getenv("PAM_USER")
	if username == "" {
		username = "unknown"
	}

	ip := os.Getenv("PAM_RHOST")
	tty := os.Getenv("PAM_TTY")
	if tty == "" {
		tty = "unknown"
	}

	port := ""
	if conn := os.Getenv("SSH_CONNECTION"); conn != "" {
		parts := strings.Fields(conn)
		if len(parts) >= 2 {
			port = parts[1]
		}
		if ip == "" && len(parts) >= 1 {
			ip = parts[0]
		}
	}

	if ip == "" {
		ip = "localhost"
	}

	return protocol.SSHLoginEvent{
		Username:  username,
		IP:        ip,
		Port:      port,
		Timestamp: now,
		Status:    "success",
		TTY:       tty,
		SessionID: strconv.Itoa(os.Getpid()),
	}
}

// SendEvent 发送事件到本地 socket
func SendEvent(event protocol.SSHLoginEvent) error {
	addr := &net.UnixAddr{
		Name: DefaultSocketPath,
		Net:  "unixgram",
	}

	conn, err := net.DialUnix("unixgram", nil, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = conn.Write(data)
	return err
}
