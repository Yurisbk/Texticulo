# Audita o repositório em busca de padrões que podem indicar segredos commitados.
# Uso: na raiz do repo, .\scripts\check-no-secrets.ps1

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $root

$found = $false

function Test-ExcludeDoc {
    param([string]$RelPath)
    return ($RelPath -match '(?i)\\docs\\') -or ($RelPath -match '(?i)^README\.md$')
}

Write-Host "Scanning under: $root" -ForegroundColor Cyan

Get-ChildItem -Path $root -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object {
        $_.FullName -notmatch "\\node_modules\\|\\dist\\|\\.git\\" -and
        $_.Extension -match '\.(go|ts|tsx|json|env|yaml|yml)$|^\.env$'
    } |
    ForEach-Object {
        $path = $_.FullName
        $rel = $path.Substring($root.Length).TrimStart("\")
        if (Test-ExcludeDoc $rel) { return }

        $text = Get-Content -Path $path -Raw -ErrorAction SilentlyContinue
        if (-not $text) { return }

        # Mongo URI that looks like real credentials (not doc placeholder)
        if ($text -match 'mongodb\+srv://[^@\s]+@[^\s]+' -and $text -notmatch 'user:pass|placeholder|\.\.\.') {
            Write-Warning "Possible MongoDB URI with credentials in: $rel"
            $found = $true
        }
        if ($text -match 'sk_live_[a-zA-Z0-9]{10,}') {
            Write-Warning "Possible Stripe live key in: $rel"
            $found = $true
        }
    }

# Docs: warn on suspicious paste in markdown
Get-ChildItem -Path (Join-Path $root "docs") -Filter "*.md" -File -ErrorAction SilentlyContinue |
    ForEach-Object {
        $text = Get-Content -Path $_.FullName -Raw
        if ($text -match 'mongodb\+srv://[a-z0-9]{20,}:[^@\s]+@') {
            Write-Warning "Suspicious Mongo URI in doc (verify placeholder): $($_.Name)"
            $found = $true
        }
    }

foreach ($f in @(".env", "backend\.env", "frontend\.env")) {
    $p = Join-Path $root $f
    if (Test-Path $p) {
        Write-Warning "File $f exists - do not commit. Ensure .gitignore excludes it."
        $found = $true
    }
}

if ($found) {
    Write-Host "`nReview warnings before git push." -ForegroundColor Yellow
    exit 1
}
Write-Host "`nNo issues flagged. Manual review still recommended." -ForegroundColor Green
exit 0
