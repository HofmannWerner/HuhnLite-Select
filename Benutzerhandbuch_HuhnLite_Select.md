# HuhnLite-Select – Benutzerhandbuch & Programmübersicht

> **Die zentrale Mandantenverwaltung & Launcher-Oberfläche für HuhnLite**

---

## Was ist HuhnLite-Select?

**HuhnLite-Select** ist der vorgeschaltete Launcher (Startbildschirm) für die HuhnLite-Software. Er dient als intelligenter "Mandanten-Auswähler" und ermöglicht es dem Benutzer, vor dem eigentlichen Start der Hauptanwendung den gewünschten Mandanten (Betrieb/Datenbestand) auszuwählen.

Das Programm fungiert als schlanker Webserver mit einer modernen, eingebetteten Benutzeroberfläche und verbindet sich nahtlos mit den bestehenden Konfigurationsdateien von HuhnLite.

---

## Hauptfunktionen

### 1. Szenarien & Funktionsweise
* **Szenario 1 (Remote-Weiterleitung):** Existiert eine `settings_select.json`, liest HuhnLite-Select die Informationen (IP-Adresse und Port bzw. `baseLink`), öffnet automatisch den Browser mit der Zieladresse und beendet sich unmittelbar danach ("verabschiedet sich").
* **Szenario 2 (Standard & MariaDB):**
  * `HuhnLite-Select.exe` sucht nach `settings_server.json` (im Aufrufverzeichnis und alternativ im Roaming-Verzeichnis `%APPDATA%/HuhnLite`).
  * `HuhnLite-Select-Mariadb.exe` sucht nach `settings_server_mariadb.json` (im Aufrufverzeichnis und alternativ im Roaming-Verzeichnis `%APPDATA%/HuhnLite-MariaDB`).
* **Szenario 3 (Postgres):**
  * `HuhnLite-Select-Postgres.exe` sucht nach `settings_server_postgres.json` (im Aufrufverzeichnis und alternativ im Roaming-Verzeichnis `%APPDATA%/HuhnLite-Postgres`).

### 2. Saubere Trennung & Strikte Übergabeparameter
* **Aufgaben von HuhnLite-Select:** Das Select-Programm liest aus den Settings ausschließlich `basePort`, `serverExec` und die Mandanten-Liste (zur Anzeige in der Weboberfläche).
* **Übergabe an die Server-Anwendung:** Beim Klick auf einen Mandanten startet HuhnLite-Select die konfigurierte `serverExec` mit **ausschließlich** folgenden Parametern:
  * `-port <basePort + mandantNr>`
  * `-darkmode <true|false>`
  * `-language <de|en|it>` (bzw. `-lng`)
* **Eigenständigkeit des Server-Programms:** Alle weiteren Einstellungen (Datenbankverbindungen, Engines, Backups, etc.) liest das Server-Programm selbstständig aus seiner eigenen Konfigurationsdatei.
* **Fehlerbehandlung:** Bei fehlenden oder ungültigen Einstellungen in den Settings bricht HuhnLite-Select sofort kontrolliert mit einer entsprechenden Fehlermeldung ab.

---

## Bedienung (Schritt-für-Schritt)

1. **Starten:** Führen Sie die Datei `HuhnLite-Select.exe` (bzw. das entsprechende Anwendungsicon auf Ihrem Desktop) aus.
2. **Browser öffnet sich:** Die Oberfläche lädt sich in der Regel automatisch in Ihrem Standard-Webbrowser (unter `http://localhost:8080`).
3. **Auswählen:** Klicken Sie auf die Kachel des gewünschten Mandanten.
4. **Verbinden:** 
   * Ist der Mandant bereits *Aktiv*, werden Sie sofort weitergeleitet.
   * Ist der Mandant *Inaktiv*, sehen Sie einen kurzen Ladebildschirm ("Server wird gestartet..."), bis die Anwendung einsatzbereit ist.
5. **Einstellungen anpassen:** Über das Menü oben rechts können Sie bei Bedarf jederzeit die Sprache oder den Dark Mode umschalten.

---

## Programmsymbole & Grafiken

Das Verzeichnis enthält ein passend gestaltetes Programm-Icon in verschiedenen Formaten:
* **`app-icon.png`**: Hochauflösendes App-Icon (512x512 PNG)
* **`app-icon.svg`**: Vektorgrafik für skalierbare Einbindung
* **`icon.ico`**: Windows-Icon-Datei für Verknüpfungen und Executable-Icons
* **`frontend/favicon.png`**: Favicon für die Browser-Leiste

---

## Technischer Hintergrund (Für Administratoren)

* **Keine externe Installation:** Das gesamte Frontend (HTML, CSS, JavaScript auf Basis von Vue 3 und Quasar) ist fest in die ausführbare `.exe`-Datei (geschrieben in Go) einkompiliert. Dadurch ist HuhnLite-Select extrem performant und benötigt keine zusätzlichen Dateien (Single-Binary).
* **Cache-Sicherheit:** Um zu verhindern, dass alte Versionen im Browser festhängen, ist eine strikte Cache-Kontrolle implementiert, die immer die aktuellste Oberfläche erzwingt.
* **Cross-Plattform:** Das Programm kann problemlos für Windows (amd64), macOS (Intel & Apple Silicon) sowie Linux kompiliert werden, da es keine komplexen Systemabhängigkeiten besitzt.
