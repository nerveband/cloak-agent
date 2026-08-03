$ErrorActionPreference = "Stop"

foreach ($command in @("node", "npm", "npx")) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) {
        throw "$command was not found in PATH. Install Node.js 20 or newer first."
    }
}

$archiveRoot = Split-Path -Parent $PSScriptRoot
$binary = Join-Path $archiveRoot "cloak-agent.exe"
$daemonManifest = Join-Path $archiveRoot "daemon\package.json"

if (-not (Test-Path $binary)) {
    throw "cloak-agent.exe was not found at $binary. Run this script from an extracted release archive."
}
if (-not (Test-Path $daemonManifest)) {
    throw "The bundled daemon manifest was not found at $daemonManifest. Extract the complete release archive."
}

& $binary install
if ($LASTEXITCODE -ne 0) {
    throw "cloak-agent install failed with exit code $LASTEXITCODE."
}

Write-Host "Installation complete. Keep cloak-agent.exe and the daemon directory together."
