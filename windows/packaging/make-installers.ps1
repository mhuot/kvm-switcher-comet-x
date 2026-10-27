# Build the KVM Switcher installers with Inno Setup: one per architecture folder found under -ExeRoot
# (win-x64 and win-arm64 from build.sh, or KVMSwitcher-win-x64 and KVMSwitcher-win-arm64 from CI artifacts),
# each holding KVMSwitcher.exe. Writes KVMSwitcher-<version>-win-<arch>-setup.exe to -OutDir.
#   pwsh windows\packaging\make-installers.ps1 [-ExeRoot <dir>] [-OutDir <dir>] [-Sign]
# -Sign has Inno Setup sign the installer and its uninstaller through sign.ps1: run sign-setup.ps1 and sign in
# to Azure (azure/login in CI, `az login` locally) first.
param(
    [string]$ExeRoot = (Join-Path $PSScriptRoot "..\publish"),
    [string]$OutDir = (Join-Path $PSScriptRoot "..\publish"),
    [switch]$Sign
)

$ErrorActionPreference = "Stop"
$projectFile = Join-Path $PSScriptRoot "..\KvmSwitcher\KvmSwitcher.csproj"
$appVersion = (Select-Xml -Path $projectFile -XPath "//Version").Node.InnerText

function Find-InnoCompiler {
    $searchPatterns = @("${env:ProgramFiles(x86)}\Inno Setup *\ISCC.exe", "$env:ProgramFiles\Inno Setup *\ISCC.exe")
    Get-ChildItem $searchPatterns -ErrorAction SilentlyContinue | Sort-Object FullName | Select-Object -Last 1
}

$innoCompiler = Find-InnoCompiler
if (-not $innoCompiler) {
    choco install innosetup -y --no-progress
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    $innoCompiler = Find-InnoCompiler
    if (-not $innoCompiler) { throw "Inno Setup (ISCC.exe) not found after installing it" }
}

$signArguments = @()
if ($Sign) {
    # $q and $f are Inno Setup's placeholders for a quote and the file to sign; the backticks keep PowerShell from
    # expanding them.
    $signScript = Join-Path $PSScriptRoot "sign.ps1"
    $signArguments = @("/DSign", "/Ssigntool=pwsh -NoProfile -File `$q$signScript`$q `$f")
}

$architectureFolders = Get-ChildItem $ExeRoot -Directory | Where-Object { $_.Name -match "win-(x64|arm64)$" }
if (-not $architectureFolders) { throw "No win-x64 or win-arm64 folder with KVMSwitcher.exe under $ExeRoot" }
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$scriptPath = Join-Path $PSScriptRoot "KvmSwitcher.iss"
foreach ($architectureFolder in $architectureFolders) {
    $architecture =($architectureFolder.Name -replace ".*win-", "")
    & $innoCompiler.FullName "/DAppVersion=$appVersion" "/DArch=$architecture" "/DExeDir=$($architectureFolder.FullName)" `
        "/DOutDir=$((Resolve-Path $OutDir).Path)" @signArguments $scriptPath
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
Get-ChildItem $OutDir -Filter "KVMSwitcher-$appVersion-win-*-setup.exe"
