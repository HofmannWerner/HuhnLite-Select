package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed frontend/*
var frontendFS embed.FS

type Mandant struct {
	ID           int    `json:"id"`
	MandantNr    int    `json:"mandantNr"`
	Name         string `json:"name"`
	Beschreibung string `json:"beschreibung"`
	Port         int    `json:"port"`
	Icon         string `json:"icon"`
	Online       bool   `json:"online"`
}

type Settings struct {
	ServerExec string    `json:"serverExec"`
	BasePort   int       `json:"basePort"`
	BaseLink   string    `json:"baseLink"`
	IP         string    `json:"ip"`
	Port       int       `json:"port"`
	Mandanten  []Mandant `json:"mandanten"`
}

var (
	settings     Settings
	settingsLock sync.RWMutex
	loadedFile   string
)

func getExecutableVariant() string {
	execPath, err := os.Executable()
	if err != nil || execPath == "" {
		if len(os.Args) > 0 {
			execPath = os.Args[0]
		}
	}
	base := filepath.Base(execPath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	lower := strings.ToLower(nameWithoutExt)

	// 1. Spezifische Datenbank-Keywords prüfen
	if strings.Contains(lower, "mariadb") {
		return "mariadb"
	}
	if strings.Contains(lower, "postgres") || strings.Contains(lower, "pgsql") {
		return "postgres"
	}
	if strings.Contains(lower, "sqlite") {
		return "sqlite"
	}

	// 2. Generische Suffixe nach Präfixen prüfen (z.B. HuhnLite-Select-XYZ)
	for _, prefix := range []string{
		"huhnlite-select-", "huhnlite-select_", "huhnlite_select-", "huhnlite_select_",
		"huhnlite-", "huhnlite_", "select-", "select_",
	} {
		if strings.HasPrefix(lower, prefix) {
			suffix := strings.TrimPrefix(lower, prefix)
			if suffix != "" {
				switch suffix {
				case "select", "launcher", "remote", "windows", "linux", "darwin", "amd64", "arm64", "x86", "x64", "386", "exe":
					// Keine DB-Variante
				default:
					return suffix
				}
			}
		}
	}

	// 3. Fallback: Trennzeichen '-' oder '_'
	if idx := strings.LastIndexAny(nameWithoutExt, "-_"); idx != -1 && idx < len(nameWithoutExt)-1 {
		part := strings.ToLower(nameWithoutExt[idx+1:])
		switch part {
		case "select", "launcher", "remote", "windows", "linux", "darwin", "amd64", "arm64", "x86", "x64", "386", "exe":
			// Keine DB-Variante
		default:
			return part
		}
	}

	return ""
}

func getSettingsFileCandidates(variant string) []string {
	var candidates []string

	if variant != "" {
		vLower := strings.ToLower(variant)
		variants := []string{vLower}
		if vLower == "postgres" {
			variants = append(variants, "postgresql")
		} else if vLower == "postgresql" {
			variants = append(variants, "postgres")
		}

		// Szenario 1: Optionale Weiterleitungs-Settings
		for _, v := range variants {
			candidates = append(candidates,
				fmt.Sprintf("settings_select_%s.json", v),
				fmt.Sprintf("settings-select-%s.json", v),
				fmt.Sprintf("Settings_select_%s.json", v),
				fmt.Sprintf("Settings-select-%s.json", v),
			)
		}
		candidates = append(candidates,
			"settings_select.json",
			"settings-select.json",
			"Settings_select.json",
			"Settings-select.json",
		)

		// Szenario 2 & 3: Spezifische Server-Settings (settings_server_mariadb.json / settings_server_postgres.json)
		for _, v := range variants {
			candidates = append(candidates,
				fmt.Sprintf("settings_server_%s.json", v),
				fmt.Sprintf("settings-server-%s.json", v),
				fmt.Sprintf("Settings_server_%s.json", v),
				fmt.Sprintf("Settings-server-%s.json", v),
				fmt.Sprintf("settings_%s.json", v),
				fmt.Sprintf("settings-%s.json", v),
			)
		}
		candidates = append(candidates,
			"settings_server.json",
			"settings-server.json",
			"settings.json",
		)
	} else {
		// Standard für HuhnLite-Select.exe: settings_select.json (Szenario 1) oder settings_server.json (Szenario 2)
		candidates = append(candidates,
			"settings_select.json",
			"settings-select.json",
			"Settings_select.json",
			"Settings-select.json",
			"settings_server.json",
			"settings-server.json",
			"Settings_server.json",
			"Settings-server.json",
			"settings.json",
		)
	}

	// Duplikate filtern
	var unique []string
	seen := make(map[string]bool)
	for _, c := range candidates {
		cLower := strings.ToLower(c)
		if !seen[cLower] {
			seen[cLower] = true
			unique = append(unique, c)
		}
	}
	return unique
}

func isSelectSettingsFile(filePath string) bool {
	if filePath == "" {
		return false
	}
	base := strings.ToLower(filepath.Base(filePath))
	return strings.HasPrefix(base, "settings_select") || strings.HasPrefix(base, "settings-select")
}

func isRemoteMode() bool {
	settingsLock.RLock()
	defer settingsLock.RUnlock()

	// Wenn eine settings_select*.json geladen wurde -> Szenario 1
	if isSelectSettingsFile(loadedFile) {
		return true
	}

	// Reiner Remote-Modus: Keine ServerExec und keine Mandanten konfiguriert, aber IP/Port oder BaseLink vorhanden
	if strings.TrimSpace(settings.ServerExec) == "" && len(settings.Mandanten) == 0 {
		if strings.TrimSpace(settings.BaseLink) != "" || (strings.TrimSpace(settings.IP) != "" && (settings.Port > 0 || settings.BasePort > 0)) {
			return true
		}
	}
	return false
}

func getRemoteTargetURL() string {
	settingsLock.RLock()
	defer settingsLock.RUnlock()

	if strings.TrimSpace(settings.BaseLink) != "" {
		link := strings.TrimSpace(settings.BaseLink)
		if !strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://") {
			link = "http://" + link
		}
		return link
	}
	if strings.TrimSpace(settings.IP) != "" {
		port := settings.Port
		if port == 0 {
			port = settings.BasePort
		}
		if port == 0 {
			port = 8080
		}
		return fmt.Sprintf("http://%s:%d", settings.IP, port)
	}
	return "http://localhost:8080"
}

type SessionTracker struct {
	sync.Mutex
	sessions map[string]time.Time
}

var activeSessions = SessionTracker{
	sessions: make(map[string]time.Time),
}

func (st *SessionTracker) Heartbeat(id string) {
	if id == "" {
		return
	}
	st.Lock()
	defer st.Unlock()
	st.sessions[id] = time.Now()
}

func (st *SessionTracker) Remove(id string) {
	st.Lock()
	defer st.Unlock()
	delete(st.sessions, id)
}

func (st *SessionTracker) CountActiveOtherSessions(currentID string) int {
	st.Lock()
	defer st.Unlock()
	now := time.Now()
	count := 0
	for id, lastSeen := range st.sessions {
		if id != currentID && now.Sub(lastSeen) < 15*time.Second {
			count++
		}
	}
	return count
}

func hasActiveMandanten() bool {
	_ = loadSettings()
	settingsLock.RLock()
	mandantenCopy := make([]Mandant, len(settings.Mandanten))
	copy(mandantenCopy, settings.Mandanten)
	basePort := settings.BasePort
	settingsLock.RUnlock()

	if basePort == 0 {
		basePort = 8080
	}

	for _, m := range mandantenCopy {
		port := m.Port
		if port == 0 && m.MandantNr > 0 {
			port = basePort + m.MandantNr
		}
		if isPortOpen(port) {
			return true
		}
	}
	return false
}

func deriveMandantDBConn(baseConn string, mandantNr int) string {
	baseConn = strings.TrimSpace(baseConn)
	if baseConn == "" {
		return ""
	}
	suffix := fmt.Sprintf("-%d", mandantNr)

	parts := strings.SplitN(baseConn, "?", 2)
	mainPath := parts[0]
	queryStr := ""
	if len(parts) > 1 {
		queryStr = "?" + parts[1]
	}

	slashIdx := strings.LastIndex(mainPath, "/")
	if slashIdx != -1 {
		dbName := mainPath[slashIdx+1:]
		if dbName != "" && !strings.HasSuffix(dbName, suffix) {
			dbNameWithSuffix := dbName + suffix
			return mainPath[:slashIdx+1] + dbNameWithSuffix + queryStr
		}
	}
	return baseConn
}

func getAppDataDir() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	variant := getExecutableVariant()
	if variant != "" {
		vTitle := strings.ToUpper(variant[:1]) + strings.ToLower(variant[1:])
		if strings.EqualFold(variant, "mariadb") {
			vTitle = "MariaDB"
		} else if strings.EqualFold(variant, "postgres") || strings.EqualFold(variant, "postgresql") {
			vTitle = "Postgres"
		}

		candidates := []string{
			filepath.Join(configDir, fmt.Sprintf("HuhnLite-%s", vTitle)),
			filepath.Join(configDir, fmt.Sprintf("HuhnLite-%s", strings.ToLower(variant))),
		}
		if strings.EqualFold(variant, "postgres") || strings.EqualFold(variant, "postgresql") {
			candidates = append(candidates,
				filepath.Join(configDir, "HuhnLite-PostgreSQL"),
				filepath.Join(configDir, "HuhnLite-postgresql"),
			)
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				return cand
			}
		}
		return candidates[0]
	}
	return filepath.Join(configDir, "HuhnLite")
}

func findSettingsFile() string {
	var searchDirs []string

	// 1. Priorität: CWD & Executable-Ordner (lokales Aufrufverzeichnis)
	cwd, _ := os.Getwd()
	if cwd != "" {
		searchDirs = append(searchDirs, cwd)
	}
	execPath, errExec := os.Executable()
	if errExec == nil {
		bundleDir := filepath.Dir(execPath)
		if bundleDir != "" && bundleDir != cwd {
			searchDirs = append(searchDirs, bundleDir)
		}
	}

	// 2. Fallback: %APPDATA%/HuhnLite-<Variant> & %APPDATA%/HuhnLite
	if configDir, err := os.UserConfigDir(); err == nil {
		variant := getExecutableVariant()
		if variant != "" {
			vTitle := strings.ToUpper(variant[:1]) + strings.ToLower(variant[1:])
			if strings.EqualFold(variant, "mariadb") {
				vTitle = "MariaDB"
			} else if strings.EqualFold(variant, "postgres") || strings.EqualFold(variant, "postgresql") {
				vTitle = "Postgres"
			}
			searchDirs = append(searchDirs,
				filepath.Join(configDir, fmt.Sprintf("HuhnLite-%s", vTitle)),
				filepath.Join(configDir, fmt.Sprintf("HuhnLite-%s", strings.ToLower(variant))),
			)
			if strings.EqualFold(variant, "postgres") || strings.EqualFold(variant, "postgresql") {
				searchDirs = append(searchDirs,
					filepath.Join(configDir, "HuhnLite-PostgreSQL"),
					filepath.Join(configDir, "HuhnLite-postgresql"),
				)
			}
		}
		appDataGeneric := filepath.Join(configDir, "HuhnLite")
		searchDirs = append(searchDirs, appDataGeneric)
	}

	// Duplikate entfernen
	var uniqueSearchDirs []string
	seenDirs := make(map[string]bool)
	for _, d := range searchDirs {
		clean := filepath.Clean(d)
		if !seenDirs[strings.ToLower(clean)] {
			seenDirs[strings.ToLower(clean)] = true
			uniqueSearchDirs = append(uniqueSearchDirs, clean)
		}
	}

	variant := getExecutableVariant()
	candidates := getSettingsFileCandidates(variant)

	for _, fileName := range candidates {
		for _, dir := range uniqueSearchDirs {
			p := filepath.Join(dir, fileName)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

func loadSettings() error {
	settingsLock.Lock()
	defer settingsLock.Unlock()

	targetFile := findSettingsFile()
	if targetFile == "" {
		variant := getExecutableVariant()
		candidates := getSettingsFileCandidates(variant)
		return fmt.Errorf("keine Settings-Datei (%s) im Aufruf- oder Roaming-Verzeichnis gefunden", strings.Join(candidates, ", "))
	}

	data, err := os.ReadFile(targetFile)
	if err != nil {
		return fmt.Errorf("Settings-Datei %s konnte nicht gelesen werden: %w", targetFile, err)
	}

	// Raw JSON Map strikt parsen (keine Fehlertoleranz)
	var rawData map[string]interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		return fmt.Errorf("Fehler beim Parsen von %s: %w", targetFile, err)
	}

	var newSettings Settings
	_ = json.Unmarshal(data, &newSettings)

	if execVal, ok := rawData["serverExec"].(string); ok && execVal != "" {
		newSettings.ServerExec = execVal
	}
	if linkVal, ok := rawData["baseLink"].(string); ok && linkVal != "" {
		newSettings.BaseLink = linkVal
	} else if linkVal, ok := rawData["baselink"].(string); ok && linkVal != "" {
		newSettings.BaseLink = linkVal
	}

	if ipVal, ok := rawData["ip"].(string); ok && ipVal != "" {
		newSettings.IP = ipVal
	} else if ipVal, ok := rawData["ipadresse"].(string); ok && ipVal != "" {
		newSettings.IP = ipVal
	} else if ipVal, ok := rawData["ip_adresse"].(string); ok && ipVal != "" {
		newSettings.IP = ipVal
	} else if hostVal, ok := rawData["host"].(string); ok && hostVal != "" {
		newSettings.IP = hostVal
	}

	if newSettings.BasePort == 0 {
		if portVal, ok := rawData["basePort"].(float64); ok {
			newSettings.BasePort = int(portVal)
		} else if portVal, ok := rawData["port"].(float64); ok {
			newSettings.BasePort = int(portVal)
		}
	}
	if newSettings.Port == 0 {
		if portVal, ok := rawData["port"].(float64); ok {
			newSettings.Port = int(portVal)
		}
	}

	// Falls es eine fertige "mandanten" Liste im JSON gibt
	if len(newSettings.Mandanten) > 0 {
		for i := range newSettings.Mandanten {
			if strings.TrimSpace(newSettings.Mandanten[i].Name) == "" {
				newSettings.Mandanten[i].Name = fmt.Sprintf("Mandant %d", newSettings.Mandanten[i].MandantNr)
			}
			if newSettings.Mandanten[i].Icon == "" {
				newSettings.Mandanten[i].Icon = "storefront"
			}
		}
	} else {
		// Dynamisches Auslesen aus "mandant_1", "mandant_2", ..., "mandant_n"
		nMap := make(map[int]bool)
		for k := range rawData {
			parts := strings.Split(k, "_")
			if len(parts) >= 2 {
				if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil && n > 0 {
					nMap[n] = true
				}
			}
		}

		var nList []int
		for n := range nMap {
			nList = append(nList, n)
		}
		sort.Ints(nList)

		for _, n := range nList {
			name := ""
			for _, key := range []string{fmt.Sprintf("mandant_%d", n), fmt.Sprintf("mandant_name_%d", n), fmt.Sprintf("mandantname_%d", n), fmt.Sprintf("name_%d", n)} {
				if v, ok := rawData[key].(string); ok && strings.TrimSpace(v) != "" {
					name = strings.TrimSpace(v)
					break
				}
			}
			if name == "" {
				name = fmt.Sprintf("Mandant %d", n)
			}

			beschreibung := ""
			for _, key := range []string{fmt.Sprintf("beschreibung_%d", n), fmt.Sprintf("desc_%d", n)} {
				if v, ok := rawData[key].(string); ok && strings.TrimSpace(v) != "" {
					beschreibung = strings.TrimSpace(v)
					break
				}
			}

			basePort := 8080
			if newSettings.BasePort > 0 {
				basePort = newSettings.BasePort
			}
			port := basePort + n
			if pVal, ok := rawData[fmt.Sprintf("port_%d", n)]; ok {
				if pNum, ok := pVal.(float64); ok {
					port = int(pNum)
				}
			}

			newSettings.Mandanten = append(newSettings.Mandanten, Mandant{
				ID:           n,
				MandantNr:    n,
				Name:         name,
				Beschreibung: beschreibung,
				Port:         port,
				Icon:         "storefront",
			})
		}
	}

	settings = newSettings
	loadedFile = targetFile
	log.Printf("Settings geladen aus %s (%d Mandanten, ServerExec: '%s', BasePort: %d)", targetFile, len(settings.Mandanten), settings.ServerExec, settings.BasePort)

	return nil
}

func isPortOpen(port int) bool {
	address := net.JoinHostPort("localhost", strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func openBrowser(rawURL string) error {
	targetURL := strings.TrimSpace(rawURL)
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		targetURL = "http://" + targetURL
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", targetURL)
	case "darwin":
		cmd = exec.Command("open", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	return cmd.Start()
}

func dirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func resolveServerExec(serverExec string) string {
	execName := strings.Trim(strings.TrimSpace(serverExec), "\"'")
	// If execName has a stray % prefix not followed by ProgramFiles, trim it
	if strings.HasPrefix(execName, "%") && !strings.HasPrefix(strings.ToLower(execName), "%programfiles%") {
		execName = strings.TrimPrefix(execName, "%")
	}
	if strings.HasPrefix(strings.ToLower(execName), "%programfiles%") {
		pf := os.Getenv("ProgramFiles")
		if pf != "" {
			execName = strings.Replace(execName, "%programfiles%", pf, 1)
			execName = strings.Replace(execName, "%PROGRAMFILES%", pf, 1)
		}
	}

	// Falls der konfigurierte Pfad bereits direkt existiert
	if execName != "" {
		if _, err := os.Stat(execName); err == nil {
			if abs, errAbs := filepath.Abs(execName); errAbs == nil {
				return abs
			}
			return execName
		}
	}

	cwd, _ := os.Getwd()
	execDir := ""
	if execPath, err := os.Executable(); err == nil {
		execDir = filepath.Dir(execPath)
	}

	// Falls execName ein relativer Name ist, zuerst direkt im Ordner der laufenden Executable und im CWD prüfen
	if execName != "" && !filepath.IsAbs(execName) {
		if execDir != "" {
			cand := filepath.Join(execDir, execName)
			if _, err := os.Stat(cand); err == nil {
				if absCand, errAbs := filepath.Abs(cand); errAbs == nil {
					return absCand
				}
				return cand
			}
		}
		if cwd != "" {
			cand := filepath.Join(cwd, execName)
			if _, err := os.Stat(cand); err == nil {
				if absCand, errAbs := filepath.Abs(cand); errAbs == nil {
					return absCand
				}
				return cand
			}
		}
	}

	// Fallback für C:\Programme\... -> C:\Program Files\... auf Windows
	if strings.HasPrefix(strings.ToLower(execName), "c:\\programme\\") {
		pf := os.Getenv("ProgramFiles")
		if pf == "" {
			pf = "C:\\Program Files"
		}
		execNameAlt := filepath.Join(pf, execName[13:])
		if _, err := os.Stat(execNameAlt); err == nil {
			return execNameAlt
		}
	} else if strings.HasPrefix(strings.ToLower(execName), "c:\\program files\\") {
		execNameAlt := filepath.Join("C:\\Programme", execName[17:])
		if _, err := os.Stat(execNameAlt); err == nil {
			return execNameAlt
		}
	}

	// Falls absolute Pfad nicht existiert, Dateinamen für Suchkandidaten verwenden
	baseExec := execName
	if filepath.IsAbs(execName) {
		baseExec = filepath.Base(execName)
	}

	settingsDir := ""
	if loadedFile != "" {
		settingsDir = filepath.Dir(loadedFile)
	}

	variant := getExecutableVariant()

	var searchDirs []string
	if cwd != "" {
		searchDirs = append(searchDirs, cwd)
	}
	if execDir != "" && execDir != cwd {
		searchDirs = append(searchDirs, execDir)
	}
	if settingsDir != "" && settingsDir != cwd && settingsDir != execDir {
		searchDirs = append(searchDirs, settingsDir)
	}

	if variant != "" {
		vTitle := strings.ToUpper(variant[:1]) + strings.ToLower(variant[1:])
		if strings.EqualFold(variant, "mariadb") {
			vTitle = "MariaDB"
		} else if strings.EqualFold(variant, "postgres") || strings.EqualFold(variant, "postgresql") {
			vTitle = "Postgres"
		}
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			searchDirs = append(searchDirs,
				filepath.Join(pf, fmt.Sprintf("HuhnLite-%s", vTitle)),
				filepath.Join(pf, fmt.Sprintf("HuhnLite-%s", strings.ToLower(variant))),
			)
		}
		if pf86 := os.Getenv("ProgramFiles(x86)"); pf86 != "" {
			searchDirs = append(searchDirs,
				filepath.Join(pf86, fmt.Sprintf("HuhnLite-%s", vTitle)),
				filepath.Join(pf86, fmt.Sprintf("HuhnLite-%s", strings.ToLower(variant))),
			)
		}
		searchDirs = append(searchDirs,
			filepath.Join("C:\\Programme", fmt.Sprintf("HuhnLite-%s", vTitle)),
			filepath.Join("C:\\Programme", fmt.Sprintf("HuhnLite-%s", strings.ToLower(variant))),
		)
	}

	if pf := os.Getenv("ProgramFiles"); pf != "" {
		searchDirs = append(searchDirs, filepath.Join(pf, "HuhnLite"), pf)
	}
	if pf86 := os.Getenv("ProgramFiles(x86)"); pf86 != "" {
		searchDirs = append(searchDirs, filepath.Join(pf86, "HuhnLite"), pf86)
	}
	searchDirs = append(searchDirs, "C:\\Programme\\HuhnLite", "C:\\Programme")

	var names []string
	if baseExec != "" {
		names = append(names, baseExec)
	}

	if variant != "" {
		vTitle := strings.ToUpper(variant[:1]) + strings.ToLower(variant[1:])
		if strings.EqualFold(variant, "mariadb") {
			vTitle = "MariaDB"
		} else if strings.EqualFold(variant, "postgres") || strings.EqualFold(variant, "postgresql") {
			vTitle = "Postgres"
		}

		variantNames := []string{
			fmt.Sprintf("HuhnLite-Server-%s.exe", vTitle),
			fmt.Sprintf("huhnlite-server-%s.exe", strings.ToLower(variant)),
			fmt.Sprintf("HuhnLite-Server-%s", vTitle),
			fmt.Sprintf("huhnlite-server-%s", strings.ToLower(variant)),
		}
		if strings.EqualFold(variant, "postgres") || strings.EqualFold(variant, "postgresql") {
			variantNames = append(variantNames,
				"HuhnLite-Server-PostgreSQL.exe",
				"huhnlite-server-postgresql.exe",
				"HuhnLite-Server-PostgreSQL",
				"huhnlite-server-postgresql",
			)
		}
		names = append(names, variantNames...)
	}

	names = append(names, "HuhnLite-Server.exe", "HuhnLite-Server", "huhnlite-wails.exe")

	for _, n := range names {
		if n == "" {
			continue
		}
		if filepath.IsAbs(n) {
			if _, err := os.Stat(n); err == nil {
				return n
			}
			continue
		}
		for _, dir := range searchDirs {
			if dir == "" {
				continue
			}
			cand := filepath.Join(dir, n)
			if _, err := os.Stat(cand); err == nil {
				if absCand, errAbs := filepath.Abs(cand); errAbs == nil {
					return absCand
				}
				return cand
			}
		}
	}

	if !filepath.IsAbs(execName) {
		fallbackDir := cwd
		if fallbackDir == "" || !dirExists(fallbackDir) {
			fallbackDir = execDir
		}
		if fallbackDir == "" || !dirExists(fallbackDir) {
			if loadedFile != "" {
				fallbackDir = filepath.Dir(loadedFile)
			}
		}
		if absExec, err := filepath.Abs(filepath.Join(fallbackDir, execName)); err == nil {
			log.Printf("[HuhnLite-Select] WARNUNG: Server-Executable '%s' wurde in den Suchverzeichnissen %v nicht direkt per os.Stat gefunden. Verwende absoluten Pfad: '%s'", execName, searchDirs, absExec)
			return absExec
		}
	}

	return execName
}

func initLogging() *os.File {
	cwd, _ := os.Getwd()
	variant := getExecutableVariant()
	logFileName := "huhnlite_select.log"
	if variant != "" {
		logFileName = fmt.Sprintf("huhnlite_select_%s.log", strings.ToLower(variant))
	}
	logPath := filepath.Join(cwd, logFileName)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		if appDataDir := getAppDataDir(); appDataDir != "" && dirExists(appDataDir) {
			logPath = filepath.Join(appDataDir, logFileName)
			f, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		}
	}

	if err == nil && f != nil {
		mw := io.MultiWriter(os.Stdout, f)
		log.SetOutput(mw)
		log.Println("==================================================")
		log.Printf("[HuhnLite-Select] Log gestartet um %s", time.Now().Format("2006-01-02 15:04:05"))
		log.Printf("[HuhnLite-Select] Log-Datei: %s", logPath)
		return f
	}
	return nil
}

func parsePortFromURL(rawURL string) int {
	target := strings.TrimSpace(rawURL)
	if target == "" {
		return 0
	}
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
			u, err = url.Parse("http://" + target)
		}
	}
	if err == nil && u != nil && u.Host != "" {
		hostStr := u.Host
		if _, portStr, err := net.SplitHostPort(hostStr); err == nil {
			if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
				return p
			}
		}
	}
	return 0
}

func logFatal(logFile *os.File, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	log.Println(msg)
	if logFile != nil {
		_ = logFile.Sync()
		_ = logFile.Close()
	}
	time.Sleep(100 * time.Millisecond)
	os.Exit(1)
}

func main() {
	logFile := initLogging()
	if logFile != nil {
		defer logFile.Close()
	}

	log.Println("[HuhnLite-Select] Launcher wird gestartet...")

	cwd, _ := os.Getwd()
	execPath, _ := os.Executable()
	variant := getExecutableVariant()
	log.Printf("[HuhnLite-Select] CWD: '%s'", cwd)
	log.Printf("[HuhnLite-Select] Executable: '%s'", execPath)
	if variant != "" {
		log.Printf("[HuhnLite-Select] Erkannte Server/DB-Variante: '%s'", variant)
	}

	// 1. Settings laden - Bei Fehler sofort abbrechen
	if err := loadSettings(); err != nil {
		logFatal(logFile, "[HuhnLite-Select] FEHLER beim Laden der Settings: %v. Anwendung bricht ab.", err)
	}
	log.Printf("[HuhnLite-Select] Geladene Datei: %s", loadedFile)
	log.Printf("[HuhnLite-Select] Konfiguriert -> ServerExec: '%s', BaseLink: '%s', BasePort: %d, Mandanten: %d",
		settings.ServerExec, settings.BaseLink, settings.BasePort, len(settings.Mandanten))

	// Szenario 1: Reiner Remote-Modus (IP/Port oder BaseLink ohne lokale ServerExec / Mandanten)
	if isRemoteMode() {
		targetURL := getRemoteTargetURL()
		log.Printf("[HuhnLite-Select] Szenario 1 (Remote-Modus aus %s). Öffne Standardbrowser: %s und beende Anwendung.\n", loadedFile, targetURL)
		if err := openBrowser(targetURL); err != nil {
			log.Printf("[HuhnLite-Select] Fehler beim Öffnen des Browsers: %v\n", err)
		}
		time.Sleep(500 * time.Millisecond)
		return
	}

	// Szenario 2 & 3: Lokaler Mandanten-Auswahl Modus
	if settings.BasePort <= 0 {
		logFatal(logFile, "[HuhnLite-Select] FEHLER: Ungültiger oder fehlender 'basePort' in %s. Anwendung bricht ab.", loadedFile)
	}
	if strings.TrimSpace(settings.ServerExec) == "" {
		logFatal(logFile, "[HuhnLite-Select] FEHLER: Fehlendes 'serverExec' in %s für den Server-Start. Anwendung bricht ab.", loadedFile)
	}

	port := settings.BasePort
	targetURL := settings.BaseLink
	if strings.TrimSpace(targetURL) == "" {
		targetURL = fmt.Sprintf("http://localhost:%d", port)
	}

	urlPort := parsePortFromURL(targetURL)
	if urlPort > 0 {
		port = urlPort
	}

	// Falls bereits eine Instanz auf dem Port läuft: Browser öffnen und beenden
	if isPortOpen(port) {
		log.Printf("[HuhnLite-Select] Port %d bereits aktiv. Öffne Browser: %s\n", port, targetURL)
		if err := openBrowser(targetURL); err != nil {
			log.Printf("[HuhnLite-Select] Fehler beim Öffnen des Browsers: %v\n", err)
		}
		time.Sleep(500 * time.Millisecond)
		return
	}

	// HTTP Server Router (Mandanten-Auswähler UI)
	mux := http.NewServeMux()

	// API Endpunkte
	mux.HandleFunc("/api/mandanten", handleGetMandanten)
	mux.HandleFunc("/api/start", handleStartMandant)
	mux.HandleFunc("/api/heartbeat", handleHeartbeat)
	mux.HandleFunc("/api/exit", handleExit)

	// Embedded Static Frontend
	frontendSub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		logFatal(logFile, "Fehler beim Laden des embedded Frontends: %v", err)
	}
	
	fileServer := http.FileServer(http.FS(frontendSub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileServer.ServeHTTP(w, r)
	})

	// Standardbrowser automatisch aufrufen
	go func() {
		time.Sleep(150 * time.Millisecond)
		log.Printf("[HuhnLite-Select] Öffne Standardbrowser: %s\n", targetURL)
		if err := openBrowser(targetURL); err != nil {
			log.Printf("[HuhnLite-Select] Fehler beim Öffnen des Browsers: %v\n", err)
		}
	}()

	addr := fmt.Sprintf(":%d", port)
	log.Printf("[HuhnLite-Select] Launcher läuft unter %s\n", targetURL)
	if err := http.ListenAndServe(addr, mux); err != nil {
		logFatal(logFile, "Server-Fehler: %v", err)
	}
}

func handleGetMandanten(w http.ResponseWriter, r *http.Request) {
	_ = loadSettings() // Frisch einlesen bei API Abruf

	settingsLock.RLock()
	mandantenCopy := make([]Mandant, len(settings.Mandanten))
	copy(mandantenCopy, settings.Mandanten)
	settingsLock.RUnlock()

	basePort := 8080
	if settings.BasePort > 0 {
		basePort = settings.BasePort
	}

	// Port Status prüfen für jeden Mandanten (BasePort + MandantNr)
	for i := range mandantenCopy {
		if mandantenCopy[i].Port == 0 && mandantenCopy[i].MandantNr > 0 {
			mandantenCopy[i].Port = basePort + mandantenCopy[i].MandantNr
		}
		mandantenCopy[i].Online = isPortOpen(mandantenCopy[i].Port)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mandantenCopy)
}

func handleStartMandant(w http.ResponseWriter, r *http.Request) {
	mandantIDStr := r.URL.Query().Get("mandantId")
	mandantID, err := strconv.Atoi(mandantIDStr)
	if err != nil {
		http.Error(w, `{"success":false, "message":"Ungültige mandantId"}`, http.StatusBadRequest)
		return
	}

	settingsLock.RLock()
	var targetMandant *Mandant
	for i := range settings.Mandanten {
		if settings.Mandanten[i].ID == mandantID {
			targetMandant = &settings.Mandanten[i]
			break
		}
	}
	serverExec := settings.ServerExec
	settingsLock.RUnlock()

	if targetMandant == nil {
		http.Error(w, `{"success":false, "message":"Mandant nicht gefunden"}`, http.StatusNotFound)
		return
	}

	basePort := 8080
	if settings.BasePort > 0 {
		basePort = settings.BasePort
	}

	port := targetMandant.Port
	if port == 0 {
		port = basePort + targetMandant.MandantNr
	}

	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	targetURL := fmt.Sprintf("%s://%s:%d", scheme, host, port)

	// Falls Port bereits offen ist: direkt Rückmeldung
	if isPortOpen(port) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"url":     targetURL,
			"already": true,
		})
		return
	}

	execName := resolveServerExec(serverExec)
	if strings.TrimSpace(execName) == "" {
		log.Printf("[HuhnLite-Select] FEHLER: Server-Executable '%s' konnte nicht aufgelöst werden.\n", serverExec)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Server-Executable '%s' nicht gefunden", serverExec),
		})
		return
	}

	lang := r.URL.Query().Get("lng")
	if lang == "" {
		lang = r.URL.Query().Get("lang")
	}
	if lang == "" {
		lang = r.URL.Query().Get("language")
	}
	if lang == "" {
		lang = "de"
	}

	darkmode := r.URL.Query().Get("darkmode")
	if darkmode == "" {
		darkmode = r.URL.Query().Get("dark")
	}
	if darkmode == "" {
		darkmode = "false"
	}

	// AUSSCHLIESSLICH: BasePort + MandantNr als Port, Darkmode, Sprachkennzeichen. Weiter Nichts!
	args := []string{
		"-port", strconv.Itoa(port),
		"-darkmode", darkmode,
		"-language", lang,
		"-lng", lang,
	}

	cmd := exec.Command(execName, args...)

	execDir := ""
	if loadedFile != "" {
		execDir = filepath.Dir(loadedFile)
	}
	if execDir == "" || !dirExists(execDir) {
		if filepath.IsAbs(execName) {
			execDir = filepath.Dir(execName)
		}
	}
	if execDir == "" || !dirExists(execDir) {
		execDir, _ = os.Getwd()
	}
	cmd.Dir = execDir

	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	log.Printf("[HuhnLite-Select] Starte Mandant %d (%s) auf Port %d mit Befehl: %s (Dir: %s)\n", targetMandant.MandantNr, targetMandant.Name, port, cmd.String(), cmd.Dir)

	if err := cmd.Start(); err != nil {
		log.Printf("[HuhnLite-Select] Fehler beim Ausführen von cmd.Start(): %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": fmt.Sprintf("Prozess konnte nicht gestartet werden: %v", err),
		})
		return
	}

	// Kurze Wartezeit (bis zu 5 Sekunden), um zu prüfen, ob der Port erreichbar wird
	started := false
	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)
		if isPortOpen(port) {
			started = true
			break
		}
	}

	if !started {
		output := strings.TrimSpace(outputBuf.String())
		if output != "" {
			log.Printf("[HuhnLite-Select] WARNUNG: Mandant %d (Port %d) ist nach 5s nicht erreichbar. Server-Konsolenausgabe:\n%s\n", targetMandant.MandantNr, port, output)
		} else {
			log.Printf("[HuhnLite-Select] WARNUNG: Mandant %d (Port %d) ist nach 5s nicht erreichbar (keine Server-Konsolenausgabe).\n", targetMandant.MandantNr, port)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if started {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"url":     targetURL,
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"url":     targetURL,
			"warning": "Server startet noch...",
		})
	}
}

func handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID != "" {
		activeSessions.Heartbeat(sessionID)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
}

func handleExit(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Tab wird geschlossen.",
	})
	log.Println("[HuhnLite-Select] Beenden angefordert - Browsertab wird geschlossen.")
}
