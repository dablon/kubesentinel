# Install kubesentinel globally from local source
$ErrorActionPreference = "Stop"

Write-Host "Installing kubesentinel globally from local source..." -ForegroundColor Cyan

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    Write-Host "Error: Go is not installed or not in PATH." -ForegroundColor Red
    exit 1
}

Push-Location $PSScriptRoot
try {
    go install ./cmd/kubesentinel/
    if ($LASTEXITCODE -ne 0) {
        Write-Host "Error: go install failed." -ForegroundColor Red
        exit 1
    }
} finally {
    Pop-Location
}

$binPath = Join-Path (go env GOPATH) "bin" "kubesentinel.exe"
Write-Host "kubesentinel installed successfully to: $binPath" -ForegroundColor Green
Write-Host "Make sure '$(go env GOPATH)\bin' is in your PATH." -ForegroundColor Yellow
