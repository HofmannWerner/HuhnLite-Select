package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	Mandanten  []Mandant `json:"mandanten"`
}

var (
	settingsFileNames = []string{
		"settings_server.json",
		"settings_server_mariadb.json",
		"Settings-server.json",
		"settings_serv.json",
		"settings.json",
		"settings_select.json",
		"Settings-select.json",
	}
	settings     Settings
	settingsLock sync.RWMutex
	loadedFile   string
)


func findSettingsFile() string {
	configDir, err := os.UserConfigDir()
	var searchDirs []string

	// 1. Priorität: %APPDATA%/HuhnLite
	if err == nil {
		appDataDir := filepath.Join(configDir, "HuhnLite")
		searchDirs = append(searchDirs, appDataDir)
	}

	// 2. Fallback: CWD & Executable-Ordner
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

	for _, fileName := range settingsFileNames {
		for _, dir := range searchDirs {
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
		return fmt.Errorf("keine Settings-Datei (%s) gefunden", strings.Join(settingsFileNames, ", "))
	}

	data, err := os.ReadFile(targetFile)
	if err != nil {
		return fmt.Errorf("Settings-Datei %s konnte nicht gelesen werden: %w", targetFile, err)
	}

	// Raw JSON Map parsen, um dynamisch mandant_1, mandant_2, mandant_n Keys zu unterstützen
	var rawData map[string]interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		return fmt.Errorf("Fehler beim Parsen von %s: %w", targetFile, err)
	}

	var newSettings Settings
	// Direkt unmarshals, um BasePort und ServerExec einzulesen
	_ = json.Unmarshal(data, &newSettings)

	if execVal, ok := rawData["serverExec"].(string); ok && execVal != "" {
		newSettings.ServerExec = execVal
	}

	// Falls es eine fertige "mandanten" Liste im JSON gibt (z.B. aus der bisherigen Launcher-Struktur)
	if len(newSettings.Mandanten) > 0 {
		for i := range newSettings.Mandanten {
			if strings.TrimSpace(newSettings.Mandanten[i].Name) == "" {
				newSettings.Mandanten[i].Name = fmt.Sprintf("Mandant %d", newSettings.Mandanten[i].MandantNr)
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
	log.Printf("Settings geladen aus %s (%d Mandanten, ServerExec: '%s')", targetFile, len(settings.Mandanten), settings.ServerExec)
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

func main() {
	log.Println("[HuhnLite-Select] Launcher wird gestartet...")

	// 1. Settings laden
	if err := loadSettings(); err != nil {
		log.Printf("Warnung beim Laden der Settings: %v\n", err)
	}

	// 2. HTTP Server Router
	mux := http.NewServeMux()

	// API Endpunkte
	mux.HandleFunc("/api/mandanten", handleGetMandanten)
	mux.HandleFunc("/api/start", handleStartMandant)

	// Embedded Static Frontend
	frontendSub, err := fs.Sub(frontendFS, "frontend")
	if err != nil {
		log.Fatalf("Fehler beim Laden des embedded Frontends: %v", err)
	}
	
	fileServer := http.FileServer(http.FS(frontendSub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fileServer.ServeHTTP(w, r)
	})

	port := 8080
	if settings.BasePort > 0 {
		port = settings.BasePort
	}

	addr := fmt.Sprintf(":%d", port)
	log.Printf("[HuhnLite-Select] Launcher läuft unter http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server-Fehler: %v", err)
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

	// Port Status prüfen für jeden Mandanten (808n)
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

	port := targetMandant.Port
	if port == 0 {
		basePort := 8080
		if settings.BasePort > 0 {
			basePort = settings.BasePort
		}
		port = basePort + targetMandant.MandantNr
	}

	// Falls Port bereits offen ist: direkt Rückmeldung
	if isPortOpen(port) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"url":     fmt.Sprintf("http://localhost:%d", port),
			"already": true,
		})
		return
	}

	// Befehl zum Starten ausführen cmd.Start()
	execName := "HuhnLite-Server.exe"
	if serverExec != "" {
		execName = strings.Trim(strings.TrimSpace(serverExec), "\"'")
	}


	// Falls relativer Pfad oder nur Dateiname, prüfe Fallbacks
	if !filepath.IsAbs(execName) {
		candidates := []string{
			execName,
			filepath.Join("C:\\Program Files\\HuhnLite", execName),
			filepath.Join("C:\\Program Files\\HuhnLite", "HuhnLite-Server.exe"),
			filepath.Join("C:\\Program Files\\HuhnLite", "huhnlite-wails.exe"),
			"huhnlite-wails.exe",
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				execName = cand
				break
			}
		}
	}

	cmd := exec.Command(execName, "-port", strconv.Itoa(port), "-mandant", strconv.Itoa(targetMandant.MandantNr))
	if filepath.IsAbs(execName) {
		cmd.Dir = filepath.Dir(execName)
	} else {
		cwd, _ := os.Getwd()
		cmd.Dir = cwd
	}

	log.Printf("[HuhnLite-Select] Starte Mandant %d (%s) auf Port %d mit Befehl: %s (Dir: %s)\n", targetMandant.MandantNr, targetMandant.Name, port, cmd.String(), cmd.Dir)

	if err := cmd.Start(); err != nil {
		log.Printf("Fehler beim Ausführen von cmd.Start(): %v\n", err)
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

	w.Header().Set("Content-Type", "application/json")
	if started {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"url":     fmt.Sprintf("http://localhost:%d", port),
		})
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"url":     fmt.Sprintf("http://localhost:%d", port),
			"warning": "Server startet noch...",
		})
	}
}
