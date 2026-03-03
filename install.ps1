$repo = "kubesentinel"
$dir = "$env:LOCALAPPDATA\Programs"
New-Item -ItemType Directory -Path "$dir" -Force | Out-Null
Invoke-WebRequest -Uri "https://github.com/dablon/$repo/releases/download/v1.0.0/kubesentinel-windows-amd64.exe" -OutFile "$dir\$repo.exe"
Write-Host "Installed to $dir\$repo.exe"
