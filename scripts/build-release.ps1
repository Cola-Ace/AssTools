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
    @{ GOOS = "windows"; GOARCH = "amd64"; Extension = ".exe"; Archive = "zip" },
    @{ GOOS = "windows"; GOARCH = "arm64"; Extension = ".exe"; Archive = "zip" },
    @{ GOOS = "linux"; GOARCH = "amd64"; Extension = ""; Archive = "tar.gz" },
    @{ GOOS = "linux"; GOARCH = "arm64"; Extension = ""; Archive = "tar.gz" },
    @{ GOOS = "darwin"; GOARCH = "amd64"; Extension = ""; Archive = "tar.gz" },
    @{ GOOS = "darwin"; GOARCH = "arm64"; Extension = ""; Archive = "tar.gz" }
)

$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
$oldCgo = $env:CGO_ENABLED
try {
    $env:CGO_ENABLED = "0"
    foreach ($target in $targets) {
        $name = "asst$($target.Extension)"
        $stage = Join-Path $dist "$($target.GOOS)_$($target.GOARCH)"
        New-Item -ItemType Directory -Path $stage | Out-Null
        $env:GOOS = $target.GOOS
        $env:GOARCH = $target.GOARCH
        go build -trimpath -ldflags="-s -w" -o (Join-Path $stage $name) ./cmd/asst
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $($target.GOOS)/$($target.GOARCH)."
        }
        Copy-Item (Join-Path $root "LICENSE") $stage
        Copy-Item (Join-Path $root "README.md") $stage
        Copy-Item (Join-Path $root "README.zh-CN.md") $stage
        $archiveBase = "asst_${releaseVersion}_$($target.GOOS)_$($target.GOARCH)"
        if ($target.Archive -eq "zip") {
            Compress-Archive -Path (Join-Path $stage "*") -DestinationPath (Join-Path $dist "$archiveBase.zip")
        } else {
            tar -czf (Join-Path $dist "$archiveBase.tar.gz") -C $stage .
            if ($LASTEXITCODE -ne 0) {
                throw "tar failed for $($target.GOOS)/$($target.GOARCH)."
            }
        }
        Remove-Item -LiteralPath $stage -Recurse -Force
    }
} finally {
    $env:GOOS = $oldGoos
    $env:GOARCH = $oldGoarch
    $env:CGO_ENABLED = $oldCgo
}

$checksums = foreach ($archive in Get-ChildItem -LiteralPath $dist -File | Sort-Object Name) {
    $hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $archive.FullName).Hash.ToLowerInvariant()
    "$hash  $($archive.Name)"
}
Set-Content -LiteralPath (Join-Path $dist "SHA256SUMS") -Value $checksums -Encoding ascii
Write-Host "Release artifacts written to $dist"
