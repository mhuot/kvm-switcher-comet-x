# Fetch what sign.ps1 needs into windows\packaging\signing\: the Windows SDK build tools (signtool.exe, a version
# the Artifact Signing dlib supports) and the Artifact Signing client (the dlib), both from NuGet at the versions
# azure/artifact-signing-action pins, plus metadata.json naming the signing account and certificate profile.
# Authentication is left to the Azure CLI (azure/login in CI, `az login` locally); the other credential kinds
# DefaultAzureCredential would try first are excluded so signing doesn't wait on them.
param(
    [Parameter(Mandatory = $true)][string]$Endpoint,
    [Parameter(Mandatory = $true)][string]$Account,
    [Parameter(Mandatory = $true)][string]$CertificateProfile,
    [string]$BuildToolsVersion = "10.0.26100.4188",
    [string]$ClientVersion = "1.0.128"
)

$ErrorActionPreference = "Stop"
$root = Join-Path $PSScriptRoot "signing"
New-Item -ItemType Directory -Force -Path $root | Out-Null
nuget install Microsoft.Windows.SDK.BuildTools -Version $BuildToolsVersion -x -OutputDirectory $root -NonInteractive
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
nuget install Microsoft.ArtifactSigning.Client -Version $ClientVersion -x -OutputDirectory $root -NonInteractive
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
@{
    Endpoint = $Endpoint
    CodeSigningAccountName = $Account
    CertificateProfileName = $CertificateProfile
    ExcludeCredentials = @(
        "EnvironmentCredential", "WorkloadIdentityCredential", "ManagedIdentityCredential", "SharedTokenCacheCredential",
        "VisualStudioCredential", "VisualStudioCodeCredential", "AzurePowerShellCredential",
        "AzureDeveloperCliCredential", "InteractiveBrowserCredential"
    )
} | ConvertTo-Json | Set-Content -Path (Join-Path $root "metadata.json") -Encoding utf8
"signing tools ready in $root"
