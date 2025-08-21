# PowerShell script to fix locations test params
$filePath = "go/apiserver/locations_test.go"

if (-not (Test-Path $filePath)) {
    Write-Host "File not found: $filePath"
    exit 1
}

$content = Get-Content $filePath -Raw

# Pattern to match the specific params creation pattern
$pattern = 'params := apiserver\.Params\{\s*\n\s*RegistrySet: &registry\.Set\{\s*\n\s*LocationRegistry: ([^,\}]+),?\s*\n\s*\},?\s*\n\s*\}'

# Replacement
$replacement = 'params := newParamsWithLocationRegistry($1)'

# Apply the replacement
$newContent = $content -replace $pattern, $replacement

# Write back to file
Set-Content -Path $filePath -Value $newContent -NoNewline

Write-Host "Updated $filePath with proper params"
