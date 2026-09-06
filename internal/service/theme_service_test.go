package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/pika-monitor/pika/internal/config"
	"github.com/pika-monitor/pika/internal/models"
	"github.com/pika-monitor/pika/pkg/version"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestValidateOfficialWebBuildAllowsMissingBuildInDevelopment(t *testing.T) {
	previousVersion := version.Version
	version.Version = "dev"
	t.Cleanup(func() { version.Version = previousVersion })
	t.Setenv("PIKA_WEB_DIR", "")

	webRoot := t.TempDir()
	if err := validateOfficialWebBuild(webRoot, filepath.Join(webRoot, "default-theme")); err != nil {
		t.Fatalf("development build should not require packaged Web assets: %v", err)
	}
}

func TestValidateOfficialWebBuildChecksExplicitWebDirInDevelopment(t *testing.T) {
	previousVersion := version.Version
	version.Version = "dev"
	t.Cleanup(func() { version.Version = previousVersion })

	webRoot := t.TempDir()
	t.Setenv("PIKA_WEB_DIR", webRoot)
	if err := validateOfficialWebBuild(webRoot, filepath.Join(webRoot, "default-theme")); err == nil {
		t.Fatal("explicit PIKA_WEB_DIR should be validated in development")
	}
}

func TestExtractThemeArchiveSecurity(t *testing.T) {
	valid := makeThemeZIP(t, map[string]string{
		"pika-theme.json": `{}`,
		"dist/index.html": `<!doctype html>`,
		"preview.png":     "preview",
	})
	if err := extractThemeArchive(valid, t.TempDir()); err != nil {
		t.Fatalf("valid archive rejected: %v", err)
	}

	tests := []struct {
		name  string
		files map[string]string
	}{
		{name: "zip slip", files: map[string]string{"../escape.js": "x"}},
		{name: "absolute", files: map[string]string{"/escape.js": "x"}},
		{name: "windows drive", files: map[string]string{"C:/escape.js": "x"}},
		{name: "case duplicate", files: map[string]string{"dist/app.js": "a", "DIST/App.js": "b"}},
		{name: "unicode duplicate", files: map[string]string{"dist/caf\u00e9.js": "a", "dist/cafe\u0301.js": "b"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := extractThemeArchive(makeThemeZIP(t, test.files), t.TempDir()); err == nil {
				t.Fatal("unsafe archive was accepted")
			}
		})
	}
}

func TestExtractThemeArchiveRejectsSymlinkAndFileCount(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: "dist/link"}
	header.SetMode(os.ModeSymlink | 0o777)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("../../secret"))
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractThemeArchive(buffer.Bytes(), t.TempDir()); err == nil {
		t.Fatal("symlink archive was accepted")
	}

	// 用 FileHeader 直接生成唯一名字，避免测试数据本身发生覆盖。
	buffer.Reset()
	writer = zip.NewWriter(&buffer)
	for index := 0; index <= maxThemeFiles; index++ {
		if _, err := writer.Create("dist/file-" + strconv.Itoa(index)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractThemeArchive(buffer.Bytes(), t.TempDir()); err == nil {
		t.Fatal("archive exceeding file count was accepted")
	}
}

func TestValidateThemeManifest(t *testing.T) {
	root := createValidThemeRoot(t, "minimal")
	manifest := validTestManifest("minimal")
	if err := validateThemeManifest(&manifest, root); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ThemeManifest)
	}{
		{name: "reserved id", mutate: func(value *ThemeManifest) { value.ID = "admin" }},
		{name: "invalid schema", mutate: func(value *ThemeManifest) { value.SchemaVersion = 2 }},
		{name: "invalid api", mutate: func(value *ThemeManifest) { value.APIVersion = "v2" }},
		{name: "invalid entry", mutate: func(value *ThemeManifest) { value.Entry = "index.html" }},
		{name: "missing capability", mutate: func(value *ThemeManifest) { value.Capabilities = []string{"server-list"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := manifest
			test.mutate(&candidate)
			if err := validateThemeManifest(&candidate, root); err == nil {
				t.Fatal("invalid manifest was accepted")
			}
		})
	}
}

func TestApplySystemMetadataEscapesValues(t *testing.T) {
	rendered := applySystemMetadata(`<!doctype html><html><head><title>old</title></head><body></body></html>`, &models.SystemConfig{
		SystemNameZh: `<script>alert(1)</script>`,
		SystemNameEn: `Pika $1`,
	})
	if strings.Contains(rendered, `<script>alert(1)</script>`) || !strings.Contains(rendered, `&lt;script&gt;`) {
		t.Fatalf("metadata was not escaped: %s", rendered)
	}
	if !strings.Contains(rendered, `<meta name="description"`) || !strings.Contains(rendered, `<link rel="icon" href="/api/logo">`) {
		t.Fatalf("metadata tags missing: %s", rendered)
	}
}

func TestRecoverFilesystemRestoresUniqueBackup(t *testing.T) {
	themeDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(themeDir, ".staging"), 0o755); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(themeDir, "minimal.backup-test")
	writeThemeRoot(t, backup, validTestManifest("minimal"))
	service := &ThemeService{themeDir: themeDir}
	if err := service.recoverFilesystem(); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	if !themeRootValid("minimal", filepath.Join(themeDir, "minimal")) {
		t.Fatal("unique valid backup was not restored")
	}
}

func TestThemeInstallActivateDeleteLifecycle(t *testing.T) {
	ctx := context.Background()
	webRoot := t.TempDir()
	writeOfficialWebBuild(t, webRoot)
	t.Setenv("PIKA_WEB_DIR", webRoot)
	t.Setenv("PIKA_DEFAULT_THEME_DIR", filepath.Join(webRoot, "default-theme"))
	themeDir := filepath.Join(t.TempDir(), "themes")

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&models.Property{}); err != nil {
		t.Fatal(err)
	}
	properties := NewPropertyService(zap.NewNop(), database)
	if err := properties.InitializeDefaultConfigs(ctx); err != nil {
		t.Fatal(err)
	}
	themeService, err := NewThemeService(zap.NewNop(), properties, &config.AppConfig{Theme: &config.ThemeConfig{Dir: themeDir}})
	if err != nil {
		t.Fatal(err)
	}

	archive := makeInstallableThemeZIP(t, validTestManifest("minimal"))
	result, err := themeService.Install(ctx, bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if result.Theme.ID != "minimal" || len(result.SHA256) != 64 {
		t.Fatalf("unexpected install result: %#v", result)
	}
	if _, err := themeService.Install(ctx, bytes.NewReader(archive)); !errors.Is(err, ErrThemeExists) {
		t.Fatalf("duplicate install should fail with ErrThemeExists, got %v", err)
	}
	caseVariant := validTestManifest("Minimal")
	if _, err := themeService.Install(ctx, bytes.NewReader(makeInstallableThemeZIP(t, caseVariant))); !errors.Is(err, ErrThemeExists) {
		t.Fatalf("case-insensitive duplicate should fail with ErrThemeExists, got %v", err)
	}
	defaultManifest := validTestManifest(DefaultThemeID)
	if _, err := themeService.Install(ctx, bytes.NewReader(makeInstallableThemeZIP(t, defaultManifest))); !errors.Is(err, ErrDefaultTheme) {
		t.Fatalf("default override should fail with ErrDefaultTheme, got %v", err)
	}

	if err := themeService.Activate(ctx, "minimal"); err != nil {
		t.Fatalf("activate failed: %v", err)
	}
	if active, err := themeService.Active(ctx); err != nil || active.ID != "minimal" {
		t.Fatalf("unexpected active theme: %#v, %v", active, err)
	}
	if err := themeService.Delete(ctx, "minimal"); !errors.Is(err, ErrThemeActive) {
		t.Fatalf("active delete should fail with ErrThemeActive, got %v", err)
	}
	if err := os.Remove(filepath.Join(themeDir, "minimal", "dist", "index.html")); err != nil {
		t.Fatal(err)
	}
	if active, err := themeService.Active(ctx); err != nil || active.ID != DefaultThemeID {
		t.Fatalf("broken active theme did not fall back: %#v, %v", active, err)
	}
	appearance, err := properties.GetAppearanceConfig(ctx)
	if err != nil || appearance.ActiveTheme != DefaultThemeID {
		t.Fatalf("fallback did not repair appearance config: %#v, %v", appearance, err)
	}
	if err := themeService.Delete(ctx, "minimal"); err != nil {
		t.Fatalf("inactive theme delete failed: %v", err)
	}
	if _, err := themeService.Get(ctx, "minimal"); !errors.Is(err, ErrThemeNotFound) {
		t.Fatalf("deleted theme still exists: %v", err)
	}
}

func makeThemeZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func makeInstallableThemeZIP(t *testing.T, manifest ThemeManifest) []byte {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return makeThemeZIP(t, map[string]string{
		"pika-theme.json": string(data),
		"dist/index.html": `<!doctype html><html><head><!-- pika:head --></head><body><!-- pika:body --></body></html>`,
		"preview.png":     "preview",
	})
}

func writeOfficialWebBuild(t *testing.T, webRoot string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(webRoot, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webRoot, "index.html"), []byte("<html>admin</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeThemeRoot(t, filepath.Join(webRoot, "default-theme"), validTestManifest(DefaultThemeID))
}

func createValidThemeRoot(t *testing.T, id string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), id)
	writeThemeRoot(t, root, validTestManifest(id))
	return root
}

func writeThemeRoot(t *testing.T, root string, manifest ThemeManifest) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pika-theme.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dist", "index.html"), []byte(`<!doctype html><html><head></head><body></body></html>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "preview.png"), []byte("preview"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func validTestManifest(id string) ThemeManifest {
	return ThemeManifest{
		SchemaVersion: 1,
		ID:            id,
		Name:          "Minimal",
		Version:       "1.2.3",
		Author:        "Pika",
		Preview:       "preview.png",
		Entry:         "dist/index.html",
		APIVersion:    ThemeAPIVersion,
		Capabilities:  append([]string(nil), requiredThemeCaps...),
	}
}
