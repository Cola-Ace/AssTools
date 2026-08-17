param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Version)) {
    if ($env:GITHUB_REF_NAME) {
        $Version = $env:GITHUB_REF_NAME
    } else {
        throw "Version is required (for example v0.1.0)."
    }
}
if ($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+$') {
    throw "Version must match vMAJOR.MINOR.PATCH."
}
$releaseVersion = $Version.Substring(1)
$root = (Resolve-Path (Join-Path $PSScriptRoot "..\")).Path
$dist = Join-Path $root "dist"
if (Test-Path $dist) {
    Remove-Item -LiteralPath $dist -Recurse -Force
}
New-Item -ItemType Directory -Path $dist | Out-Null

$targets = @(
    @{ GOOS = "windows"; GOARCH = "amd64"; Extension = ".exe" },
    @{ GOOS = "windows"; GOARCH = "arm64"; Extension = ".exe" },
    @{ GOOS = "linux"; GOARCH = "amd64"; Extension = "" },
    @{ GOOS = "linux"; GOARCH = "arm64"; Extension = "" },
    @{ GOOS = "darwin"; GOARCH = "amd64"; Extension = "" },
    @{ GOOS = "darwin"; GOARCH = "arm64"; Extension = "" }
)

$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
$oldCgo = $env:CGO_ENABLED
try {
    $env:CGO_ENABLED = "0"
    foreach ($target in $targets) {
        $name = "asst_${releaseVersion}_$($target.GOOS)_$($target.GOARCH)$($target.Extension)"
        $env:GOOS = $target.GOOS
        $env:GOARCH = $target.GOARCH
        go build -trimpath -ldflags="-s -w" -o (Join-Path $dist $name) ./cmd/asst
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $($target.GOOS)/$($target.GOARCH)."
        }
    }
} finally {
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    $env:CGO_ENABLED = $oldCgo
}

$checksums = foreach ($binary in Get-ChildItem -LiteralPath $dist -Filter "asst_*" -File | Sort-Object Name) {
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $binary.FullName).Hash.ToLowerInvariant()
    "$hash  $($binary.Name)"
}
Set-Content -LiteralPath (Join-Path $dist "SHA256SUMS") -Value $checksums -Encoding ascii
Write-Host "Release artifacts written to $dist"
