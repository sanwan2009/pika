package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pika-monitor/pika/pkg/version"
	"golang.org/x/text/unicode/norm"
)

// validateOfficialWebBuild 启动时验证官方管理前端和默认主题的发布产物完整性。
func validateOfficialWebBuild(webDir, defaultThemeDir string) error {
	if version.GetVersion() == "dev" && strings.TrimSpace(os.Getenv("PIKA_WEB_DIR")) == "" {
		return nil
	}
	requiredFiles := []string{
		filepath.Join(webDir, "index.html"),
		filepath.Join(defaultThemeDir, "pika-theme.json"),
		filepath.Join(defaultThemeDir, "dist", "index.html"),
	}
	for _, path := range requiredFiles {
		stat, err := os.Stat(path)
		if err != nil || !stat.Mode().IsRegular() {
			return fmt.Errorf("官方 Web 构建缺失: %s", path)
		}
	}
	assetStat, err := os.Stat(filepath.Join(webDir, "assets"))
	if err != nil || !assetStat.IsDir() {
		return fmt.Errorf("官方管理静态资源目录缺失: %s", filepath.Join(webDir, "assets"))
	}
	manifest, err := loadThemeManifest(filepath.Join(defaultThemeDir, "pika-theme.json"))
	if err != nil {
		return fmt.Errorf("官方默认主题清单无效: %w", err)
	}
	if manifest.ID != DefaultThemeID {
		return errors.New("官方默认主题清单 id 必须为 default")
	}
	if err := validateThemeManifest(manifest, defaultThemeDir); err != nil {
		return fmt.Errorf("官方默认主题无效: %w", err)
	}
	return nil
}

// loadThemeManifest 读取并解析 pika-theme.json。
func loadThemeManifest(path string) (*ThemeManifest, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxThemeManifestSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxThemeManifestSize {
		return nil, errors.New("主题清单超过大小限制")
	}
	var manifest ThemeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("解析主题清单失败: %w", err)
	}
	return &manifest, nil
}

func validateThemeManifest(manifest *ThemeManifest, root string) error {
	if manifest.SchemaVersion != ThemeSchemaVersion {
		return fmt.Errorf("不支持的主题清单版本: %d", manifest.SchemaVersion)
	}
	if manifest.ID != DefaultThemeID && !validThemeID(manifest.ID) {
		return errors.New("主题 id 无效或属于保留名称")
	}
	if strings.TrimSpace(manifest.Name) == "" || strings.TrimSpace(manifest.Author) == "" || strings.TrimSpace(manifest.Version) == "" {
		return errors.New("主题清单缺少 name、author 或 version")
	}
	if manifest.APIVersion != ThemeAPIVersion {
		return fmt.Errorf("主题 API 版本 %q 与系统 %q 不兼容", manifest.APIVersion, ThemeAPIVersion)
	}
	if manifest.Entry != "dist/index.html" {
		return errors.New("entry 必须是 dist/index.html")
	}
	if err := validateRelativeFile(root, manifest.Entry, 0); err != nil {
		return fmt.Errorf("主题入口无效: %w", err)
	}
	if err := validateThemeIndex(filepath.Join(root, manifest.Entry)); err != nil {
		return fmt.Errorf("主题入口无效: %w", err)
	}
	if err := validateRelativeFile(root, manifest.Preview, maxThemePreviewSize); err != nil {
		return fmt.Errorf("主题预览图无效: %w", err)
	}
	capabilities := make(map[string]bool, len(manifest.Capabilities))
	for _, capability := range manifest.Capabilities {
		capabilities[capability] = true
	}
	for _, required := range requiredThemeCaps {
		if !capabilities[required] {
			return fmt.Errorf("主题缺少核心能力 %s", required)
		}
	}
	return nil
}

func validateThemeIndex(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxThemeIndexSize+1))
	if err != nil {
		return err
	}
	if len(data) == 0 || len(data) > maxThemeIndexSize {
		return errors.New("index.html 为空或超过 8 MiB")
	}
	lower := strings.ToLower(string(data))
	if !strings.Contains(lower, "</head>") || !strings.Contains(lower, "</body>") {
		return errors.New("index.html 缺少 head 或 body 结束标签")
	}
	return nil
}

func validThemeID(id string) bool {
	if !themeIDPattern.MatchString(id) {
		return false
	}
	_, reserved := reservedThemeIDs[strings.ToLower(id)]
	return !reserved
}

// validateRelativeFile 防止文件路径穿越（含符号链接）。
func validateRelativeFile(root, relative string, maxSize int64) error {
	if relative == "" || filepath.IsAbs(relative) || strings.Contains(relative, "\\") {
		return errors.New("文件路径无效")
	}
	clean := filepath.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return errors.New("文件路径越界")
	}
	full := filepath.Join(root, clean)
	rel, err := filepath.Rel(root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("文件路径越界")
	}
	stat, err := os.Lstat(full)
	if err != nil || !stat.Mode().IsRegular() {
		return errors.New("文件不存在或不是普通文件")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return errors.New("主题根目录不可用")
	}
	realFile, err := filepath.EvalSymlinks(full)
	if err != nil {
		return errors.New("文件路径不可用")
	}
	realRel, err := filepath.Rel(realRoot, realFile)
	if err != nil || realRel == ".." || strings.HasPrefix(realRel, ".."+string(os.PathSeparator)) {
		return errors.New("文件路径经过符号链接越界")
	}
	if maxSize > 0 && stat.Size() > maxSize {
		return errors.New("文件超过大小限制")
	}
	return nil
}

// readThemeArchive 读取上传的 ZIP 数据并计算 SHA-256。
func readThemeArchive(reader io.Reader) ([]byte, string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxThemeArchiveSize+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || len(data) > maxThemeArchiveSize {
		return nil, "", errors.New("主题压缩包为空或超过 64 MiB")
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

// extractThemeArchive 安全解压主题 ZIP，防御 ZIP Slip、符号链接、重复路径、大小溢出等。
func extractThemeArchive(data []byte, destination string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return errors.New("无法读取主题 ZIP")
	}
	if len(reader.File) == 0 || len(reader.File) > maxThemeFiles {
		return errors.New("主题 ZIP 文件数量无效或超过限制")
	}
	seen := map[string]bool{}
	var total uint64
	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		clean := filepath.ToSlash(filepath.Clean(name))
		if clean == "." || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "../") ||
			strings.Contains(clean, "/../") || (len(clean) >= 2 && clean[1] == ':') {
			return fmt.Errorf("主题 ZIP 包含非法路径: %s", file.Name)
		}
		key := strings.ToLower(norm.NFC.String(clean))
		if seen[key] {
			return fmt.Errorf("主题 ZIP 包含重复路径: %s", file.Name)
		}
		seen[key] = true
		mode := file.Mode()
		if mode&os.ModeSymlink != 0 || (mode&os.ModeType != 0 && !mode.IsDir()) {
			return fmt.Errorf("主题 ZIP 包含不支持的文件类型: %s", file.Name)
		}
		if file.UncompressedSize64 > maxThemeFileSize {
			return fmt.Errorf("主题文件超过大小限制: %s", file.Name)
		}
		total += file.UncompressedSize64
		if total > maxThemeExtracted {
			return errors.New("主题解压后总大小超过限制")
		}
		target := filepath.Join(destination, filepath.FromSlash(clean))
		rel, relErr := filepath.Rel(destination, target)
		if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return errors.New("主题 ZIP 路径越界")
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, io.LimitReader(src, maxThemeFileSize+1))
		closeErr := dst.Close()
		src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func replaceBeforeFirst(value string, marker *regexp.Regexp, insertion string) string {
	location := marker.FindStringIndex(value)
	if location == nil {
		return value
	}
	return value[:location[0]] + insertion + value[location[0]:]
}
