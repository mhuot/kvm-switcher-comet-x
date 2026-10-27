# Sign one file with Azure Artifact Signing, for Inno Setup's SignTool while the installer is compiled: that is the
# only moment the uninstaller (unins000.exe) can be signed, so the signing can't be left to a later step.
#   iscc /DSign "/Ssigntool=pwsh -NoProfile -File <this file> $f" ...
# Needs windows\packaging\signing\ (made by sign-setup.ps1): the Windows SDK's signtool, the Artifact Signing dlib
# and metadata.json; and a signed-in Azure CLI (azure/login in CI), which the dlib uses through DefaultAzureCredential.
param([Parameter(Mandatory = $true)][string]$File)

$ErrorActionPreference = "Stop"
$root = Join-Path $PSScriptRoot "signing"
$signtool = Get-ChildItem "$root\Microsoft.Windows.SDK.BuildTools\bin\*\x64\signtool.exe" | Sort-Object FullName | Select-Object -Last 1
$dlib = Get-ChildItem "$root\Microsoft.ArtifactSigning.Client\bin\x64\Azure.CodeSigning.Dlib.dll" | Select-Object -First 1
if (-not $signtool -or -not $dlib) { throw "signing tools not set up: run windows\packaging\sign-setup.ps1 first" }
& $signtool.FullName sign /fd SHA256 /tr http://timestamp.acs.microsoft.com /td SHA256 `
    /dlib $dlib.FullName /dmdf (Join-Path $root "metadata.json") $File
exit $LASTEXITCODE
