$ErrorActionPreference = "Stop"
$archiveRoot = Split-Path -Parent $PSScriptRoot
$binary = Join-Path $archiveRoot "cloak-agent.exe"
if (-not (Test-Path $binary)) {
    throw "Extract the complete release archive before running install.ps1."
}
& $binary install --from $archiveRoot
if ($LASTEXITCODE -ne 0) {
    throw "Installation failed with exit code $LASTEXITCODE. The prior managed release remains selected."
}
$installRoot = if ($env:CLOAK_AGENT_INSTALL_DIR) { $env:CLOAK_AGENT_INSTALL_DIR } else { Join-Path $HOME ".cloak-agent" }
Write-Host "Installation complete. Add $(Join-Path $installRoot 'bin') to PATH."
