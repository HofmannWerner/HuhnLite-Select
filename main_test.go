package main

import (
	"testing"
)

func TestGetSettingsFileCandidates(t *testing.T) {
	tests := []struct {
		variant          string
		expectedFirst    string
		expectedContains []string
	}{
		{
			variant:       "mariadb",
			expectedFirst: "settings_select_mariadb.json",
			expectedContains: []string{
				"settings_select_mariadb.json",
				"settings_server_mariadb.json",
				"settings-server-mariadb.json",
				"settings_select.json",
			},
		},
		{
			variant:       "postgres",
			expectedFirst: "settings_select_postgres.json",
			expectedContains: []string{
				"settings_select_postgres.json",
				"settings_server_postgres.json",
				"settings-server-postgres.json",
				"settings_select.json",
			},
		},
		{
			variant:       "",
			expectedFirst: "settings_select.json",
			expectedContains: []string{
				"settings_select.json",
				"settings_server.json",
				"settings-server.json",
				"settings.json",
			},
		},
	}

	for _, tt := range tests {
		candidates := getSettingsFileCandidates(tt.variant)
		if len(candidates) == 0 {
			t.Fatalf("getSettingsFileCandidates(%q) returned empty slice", tt.variant)
		}
		if candidates[0] != tt.expectedFirst {
			t.Errorf("getSettingsFileCandidates(%q)[0] = %q; want %q", tt.variant, candidates[0], tt.expectedFirst)
		}

		for _, expected := range tt.expectedContains {
			found := false
			for _, c := range candidates {
				if c == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("getSettingsFileCandidates(%q) missing expected candidate %q", tt.variant, expected)
			}
		}
	}
}

func TestDeriveMandantDBConn(t *testing.T) {
	tests := []struct {
		baseConn  string
		mandantNr int
		want      string
	}{
		{
			baseConn:  "postgres://postgres:post@192.168.178.28:5432/huhnlite-prod?sslmode=disable",
			mandantNr: 1,
			want:      "postgres://postgres:post@192.168.178.28:5432/huhnlite-prod-1?sslmode=disable",
		},
		{
			baseConn:  "postgres://postgres:post@192.168.178.28:5432/huhnlite-prod?sslmode=disable",
			mandantNr: 2,
			want:      "postgres://postgres:post@192.168.178.28:5432/huhnlite-prod-2?sslmode=disable",
		},
		{
			baseConn:  "root:studio@tcp(127.0.0.1:3307)/huhnlite-prod?parseTime=true&allowNativePasswords=true",
			mandantNr: 1,
			want:      "root:studio@tcp(127.0.0.1:3307)/huhnlite-prod-1?parseTime=true&allowNativePasswords=true",
		},
		{
			baseConn:  "postgres://postgres:post@192.168.178.28:5432/huhnlite-prod-1?sslmode=disable",
			mandantNr: 1,
			want:      "postgres://postgres:post@192.168.178.28:5432/huhnlite-prod-1?sslmode=disable",
		},
	}

	for _, tt := range tests {
		got := deriveMandantDBConn(tt.baseConn, tt.mandantNr)
		if got != tt.want {
			t.Errorf("deriveMandantDBConn(%q, %d) = %q; want %q", tt.baseConn, tt.mandantNr, got, tt.want)
		}
	}
}

func TestIsRemoteModeAndTargetURL(t *testing.T) {
	settingsLock.Lock()
	// Testfall 1: Szenario 1 Remote-Weiterleitung (nur baseLink)
	settings = Settings{
		BaseLink: "http://192.168.178.100:9000",
	}
	settingsLock.Unlock()

	if !isRemoteMode() {
		t.Errorf("isRemoteMode() = false; want true for pure BaseLink config")
	}
	if got := getRemoteTargetURL(); got != "http://192.168.178.100:9000" {
		t.Errorf("getRemoteTargetURL() = %q; want %q", got, "http://192.168.178.100:9000")
	}

	settingsLock.Lock()
	// Testfall 2: Szenario 1 Remote-Weiterleitung (IP & Port)
	settings = Settings{
		IP:   "192.168.1.50",
		Port: 8080,
	}
	settingsLock.Unlock()

	if !isRemoteMode() {
		t.Errorf("isRemoteMode() = false; want true for IP & Port config")
	}
	if got := getRemoteTargetURL(); got != "http://192.168.1.50:8080" {
		t.Errorf("getRemoteTargetURL() = %q; want %q", got, "http://192.168.1.50:8080")
	}

	settingsLock.Lock()
	// Testfall 3: Lokaler Mandanten-Modus (ServerExec + Mandanten)
	settings = Settings{
		ServerExec: "HuhnLite-Server.exe",
		BasePort:   9000,
		Mandanten: []Mandant{
			{ID: 1, MandantNr: 1, Name: "Mandant 1"},
		},
	}
	settingsLock.Unlock()

	if isRemoteMode() {
		t.Errorf("isRemoteMode() = true; want false for local ServerExec config")
	}
}
