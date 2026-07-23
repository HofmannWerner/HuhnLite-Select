$ErrorActionPreference = "Stop"

Write-Host "Kompiliere HuhnLite-Select für Windows (amd64)..." -ForegroundColor Cyan
$env:GOOS="windows"
$env:GOARCH="amd64"
go build -ldflags="-s -w" -trimpath -o "build/HuhnLite-Select-windows-amd64.exe" main.go

Write-Host "Kompiliere HuhnLite-Select für Linux (amd64)..." -ForegroundColor Cyan
$env:GOOS="linux"
$env:GOARCH="amd64"
go build -ldflags="-s -w" -trimpath -o "build/HuhnLite-Select-linux-amd64" main.go

Write-Host "Kompiliere HuhnLite-Select für macOS Intel (amd64)..." -ForegroundColor Cyan
$env:GOOS="darwin"
$env:GOARCH="amd64"
go build -ldflags="-s -w" -trimpath -o "build/HuhnLite-Select-darwin-amd64" main.go

Write-Host "Kompiliere HuhnLite-Select für macOS Apple Silicon (arm64)..." -ForegroundColor Cyan
$env:GOOS="darwin"
$env:GOARCH="arm64"
go build -ldflags="-s -w" -trimpath -o "build/HuhnLite-Select-darwin-arm64" main.go

Write-Host "Alle Builds erfolgreich abgeschlossen! Die Dateien liegen im Ordner 'build'." -ForegroundColor Green
