package service

import (
	"regexp"
	"strings"
	"testing"
)

func TestSystemdScriptUsesSupportedTemplateKeys(t *testing.T) {
	// kardianos/service v1.3 uses a small template engine instead of
	// text/template. Top-level keys must not have a leading dot, ranges only
	// expose {{.}}, and compound expressions are not supported.
	validKeys := map[string]bool{
		".":           true,
		"Description": true, "Path": true, "Name": true,
		"Dependencies": true, "Arguments": true, "ChRoot": true,
		"WorkingDirectory": true, "UserName": true, "ReloadSignal": true,
		"PIDFile": true, "LogDirectory": true, "OutputFileSupport": true,
		"LimitNOFILE": true, "Restart": true, "SuccessExitStatus": true,
		"EnvVars": true,
	}
	validFunctions := map[string]bool{"cmd": true, "cmdEscape": true}
	actions := regexp.MustCompile(`{{-?\s*(.*?)\s*-?}}`).FindAllStringSubmatch(systemdScript, -1)

	for _, match := range actions {
		action := strings.TrimSpace(match[1])
		if action == "end" {
			continue
		}

		if strings.HasPrefix(action, "if ") || strings.HasPrefix(action, "range ") {
			fields := strings.Fields(action)
			if len(fields) != 2 || !validKeys[fields[1]] {
				t.Errorf("unsupported systemd template action %q", action)
			}
			continue
		}

		pipeline := strings.Split(action, "|")
		key := strings.TrimSpace(pipeline[0])
		if !validKeys[key] {
			t.Errorf("unsupported systemd template key %q", key)
		}
		for _, fn := range pipeline[1:] {
			name := strings.TrimSpace(fn)
			if !validFunctions[name] {
				t.Errorf("unsupported systemd template function %q", name)
			}
		}
	}

	if !strings.Contains(systemdScript, "RestartSec=5") {
		t.Error("systemd template must keep RestartSec=5")
	}
}

func TestWindowsServiceRecoveryOptions(t *testing.T) {
	options := serviceOptions("windows")

	want := map[string]any{
		"OnFailure":              "restart",
		"OnFailureDelayDuration": "1s",
		"OnFailureResetPeriod":   86400,
	}
	for key, expected := range want {
		if actual := options[key]; actual != expected {
			t.Errorf("option %q = %#v, want %#v", key, actual, expected)
		}
	}

	for _, legacyKey := range []string{"RestartDelay", "ResetPeriod"} {
		if _, exists := options[legacyKey]; exists {
			t.Errorf("legacy option %q must not be used", legacyKey)
		}
	}
}
