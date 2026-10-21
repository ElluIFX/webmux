[CmdletBinding()]
param(
    [string]$OutputPath = 'bin/webmux.exe',
    [string]$Version = 'dev'
)

$ErrorActionPreference = 'Stop'
$repoRoot = $PSScriptRoot
$go = Get-Command go.exe -CommandType Application -ErrorAction SilentlyContinue
$node = Get-Command node.exe -CommandType Application -ErrorAction SilentlyContinue
if (-not $go) { throw 'Go 1.26 or newer must be available on PATH.' }
if (-not $node) { throw 'Node.js 24 or newer must be available on PATH.' }
$goVersion = & $go.Source version
if ($LASTEXITCODE -ne 0 -or $goVersion -notmatch 'go(\d+)\.(\d+)') {
    throw "Cannot determine Go version: $goVersion"
}
if ([version]"$($Matches[1]).$($Matches[2])" -lt [version]'1.26') {
    throw "Go 1.26 or newer is required: $goVersion"
}
$nodeVersion = & $node.Source --version
if ($LASTEXITCODE -ne 0 -or $nodeVersion -notmatch '^v(\d+)\.') {
    throw "Cannot determine Node.js version: $nodeVersion"
}
if ([int]$Matches[1] -lt 24) { throw "Node.js 24 or newer is required: $nodeVersion" }

$corepack = Get-Command corepack.cmd -CommandType Application -ErrorAction SilentlyContinue
if ($corepack) {
    $packageRunner = $corepack.Source
    $packageArgs = @('pnpm')
} else {
    $pnpm = Get-Command pnpm.cmd -CommandType Application -ErrorAction SilentlyContinue
    if (-not $pnpm) { throw 'Install Corepack or the pnpm version pinned in web/package.json.' }
    $packageRunner = $pnpm.Source
    $packageArgs = @()
}
if (-not [IO.Path]::IsPathRooted($OutputPath)) {
    $OutputPath = Join-Path $repoRoot $OutputPath
}
$OutputPath = [IO.Path]::GetFullPath($OutputPath)
$previousCgo = $env:CGO_ENABLED
$previousGoos = $env:GOOS
$previousGoarch = $env:GOARCH
Push-Location (Join-Path $repoRoot 'web')
try {
    $manifest = Get-Content package.json -Raw | ConvertFrom-Json
    $expectedPnpm = ($manifest.packageManager -split '@')[1]
    $actualPnpm = & $packageRunner @packageArgs --version
    if ($LASTEXITCODE -ne 0 -or $actualPnpm.Trim() -ne $expectedPnpm) {
        throw "pnpm $expectedPnpm is required; found $actualPnpm."
    }
    & $packageRunner @packageArgs install --frozen-lockfile
    if ($LASTEXITCODE -ne 0) { throw 'Frontend dependency installation failed.' }
    & $packageRunner @packageArgs run build
    if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
    Set-Location $repoRoot
    $env:CGO_ENABLED = '0'
    $env:GOOS = 'windows'
    $env:GOARCH = & $go.Source env GOHOSTARCH
    if ($LASTEXITCODE -ne 0) { throw 'Cannot determine the native Go architecture.' }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutputPath) | Out-Null
    & $go.Source build -trimpath -ldflags "-s -w -X main.version=$Version" -o $OutputPath ./cmd/webmux
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed. A running executable must be stopped before replacing it, or use -OutputPath.' }
    Write-Host "Built $OutputPath"
} finally {
    $env:CGO_ENABLED = $previousCgo
    $env:GOOS = $previousGoos
    $env:GOARCH = $previousGoarch
    Pop-Location
}
