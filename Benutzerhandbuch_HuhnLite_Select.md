# HuhnLite-Select – Benutzerhandbuch & Programmübersicht

## Was ist HuhnLite-Select?
**HuhnLite-Select** ist der vorgeschaltete Launcher (Startbildschirm) für die HuhnLite-Software. Er dient als intelligenter "Mandanten-Auswähler" und ermöglicht es dem Benutzer, vor dem eigentlichen Start der Hauptanwendung den gewünschten Mandanten (Betrieb/Datenbestand) auszuwählen.

Das Programm fungiert als schlanker Webserver mit einer modernen, eingebetteten Benutzeroberfläche und verbindet sich nahtlos mit den bestehenden Konfigurationsdateien von HuhnLite.

---

## Hauptfunktionen

### 1. Mandanten-Verwaltung
* **Automatische Erkennung:** Das Programm liest selbstständig die `settings_server.json` (z. B. aus `%APPDATA%\HuhnLite`) aus und erkennt alle dort konfigurierten Mandanten (z. B. "Otto Dotter", "Gustav Hahn").
* **Status-Prüfung:** Der Launcher prüft in Echtzeit (via Port-Scan), ob der Server für einen bestimmten Mandanten bereits im Hintergrund läuft (Status: *Aktiv* / *Inaktiv*).
* **Dynamischer Start:** Wird ein inaktiver Mandant ausgewählt, startet HuhnLite-Select automatisch den zugehörigen HuhnLite-Server-Prozess mit den korrekten Startparametern (Port und Mandanten-ID) und leitet den Benutzer anschließend direkt zur entsprechenden Anwendungsoberfläche weiter.

### 2. Moderne & anpassbare Oberfläche
* **Mehrsprachigkeit:** Die Benutzeroberfläche kann nahtlos zwischen **Deutsch, Englisch und Italienisch** umgeschaltet werden. Die gewählte Sprache wird für zukünftige Starts gespeichert.
* **Dark Mode:** Ein integrierter Tag-/Nachtmodus (Light/Dark Mode) schont die Augen in dunklen Arbeitsumgebungen und sorgt für ein modernes Erscheinungsbild.
* **Klarheit & Design:** Eine übersichtliche Kachel-Ansicht (Cards) sorgt dafür, dass sofort ersichtlich ist, welche Mandanten zur Verfügung stehen und welche gerade aktiv genutzt werden.

---

## Technischer Hintergrund (Für Administratoren)
* **Keine externe Installation:** Das gesamte Frontend (HTML, CSS, JavaScript auf Basis von Vue 3 und Quasar) ist fest in die ausführbare `.exe`-Datei (geschrieben in der Programmiersprache Go) einkompiliert. Dadurch ist HuhnLite-Select extrem performant und benötigt keine zusätzlichen Dateien (Single-Binary).
* **Cache-Sicherheit:** Um zu verhindern, dass alte Versionen im Browser festhängen, ist eine strikte Cache-Kontrolle implementiert, die immer die aktuellste Oberfläche erzwingt.
* **Cross-Plattform:** Das Programm kann problemlos für Windows (amd64), macOS (Intel & Apple Silicon) sowie Linux kompiliert werden, da es keine komplexen Systemabhängigkeiten besitzt.

---

## Bedienung (Schritt-für-Schritt)
1. **Starten:** Führen Sie die Datei `HuhnLite-Select.exe` (bzw. das entsprechende Icon auf Ihrem Desktop) aus.
2. **Browser öffnet sich:** Die Oberfläche lädt sich in der Regel automatisch in Ihrem Standard-Webbrowser (unter `http://localhost:8080`).
3. **Auswählen:** Klicken Sie auf die Kachel des gewünschten Mandanten.
4. **Verbinden:** 
   * Ist der Mandant bereits *Aktiv*, werden Sie sofort weitergeleitet.
   * Ist der Mandant *Inaktiv*, sehen Sie einen kurzen Ladebildschirm ("Server wird gestartet..."), bis die Anwendung einsatzbereit ist.
5. **Einstellungen anpassen:** Über das Menü oben rechts können Sie bei Bedarf jederzeit die Sprache oder den Dark Mode umschalten.
