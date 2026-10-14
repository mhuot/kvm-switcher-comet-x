#!/bin/zsh
# Builds self-contained single-file KVMSwitcher.exe for Windows (x64 and Arm64)
# using the .NET SDK container, so no .NET install is needed on the build machine.
# Output: windows/publish/<runtime>/KVMSwitcher.exe
set -euo pipefail
cd "${0:A:h}"

dotnet_sdk_image="mcr.microsoft.com/dotnet/sdk:10.0"
for runtime_identifier in win-x64 win-arm64; do
    docker run --rm -v "$PWD":/src -w /src/KvmSwitcher "$dotnet_sdk_image" \
        dotnet publish -c Release -r "$runtime_identifier" --self-contained true \
        -p:PublishSingleFile=true \
        -p:EnableCompressionInSingleFile=true \
        -p:IncludeNativeLibrariesForSelfExtract=true \
        -p:DebugType=none \
        -o "/src/publish/$runtime_identifier"
done
ls -lh publish/*/KVMSwitcher.exe
