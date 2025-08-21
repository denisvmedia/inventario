# PowerShell script to add authentication headers to API tests
param(
    [string]$FilePath
)

if (-not $FilePath) {
    Write-Host "Usage: .\fix_auth_tests.ps1 <file_path>"
    exit 1
}

if (-not (Test-Path $FilePath)) {
    Write-Host "File not found: $FilePath"
    exit 1
}

$content = Get-Content $FilePath -Raw

# Pattern to match HTTP request creation followed by assertion and recorder
$pattern = '(\s+)(req, err := http\.NewRequest\([^)]+\))\s*\n(\s+)(c\.Assert\(err, qt\.IsNil\))\s*\n(\s+)(rr := httptest\.NewRecorder\(\))'

# Replacement with authentication header added
$replacement = '$1$2' + "`n" + '$3$4' + "`n" + '$3addTestUserAuthHeader(req)' + "`n" + '$5$6'

# Apply the replacement
$newContent = $content -replace $pattern, $replacement

# Write back to file
Set-Content -Path $FilePath -Value $newContent -NoNewline

Write-Host "Updated $FilePath with authentication headers"
