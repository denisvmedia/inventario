# PowerShell script to fix EntityID references in test files using the helper function

$files = @(
    "go\backup\export\service_test.go",
    "go\backup\export\streaming_test.go",
    "go\backup\restore\service_test.go",
    "go\apiserver\apiserver_test.go",
    "go\apiserver\areas_test.go",
    "go\apiserver\commodities_test.go",
    "go\registry\commonsql\areas_test.go",
    "go\registry\commonsql\commodities_test.go",
    "go\registry\commonsql\images_test.go",
    "go\registry\commonsql\invoices_test.go",
    "go\registry\commonsql\locations_test.go",
    "go\registry\commonsql\manuals_test.go"
)

foreach ($file in $files) {
    if (Test-Path $file) {
        Write-Host "Processing $file..."
        
        # Read the file content
        $content = Get-Content $file -Raw
        
        # Replace EntityID: models.EntityID{ID: "..."} with TenantAwareEntityID: models.WithTenantAwareEntityID("...", "default-tenant")
        $content = $content -replace 'EntityID:\s*models\.EntityID\{ID:\s*"([^"]+)"\}', 'TenantAwareEntityID: models.WithTenantAwareEntityID("$1", "default-tenant")'
        
        # Write the content back
        Set-Content $file -Value $content -NoNewline
        
        Write-Host "Fixed $file"
    } else {
        Write-Host "File not found: $file"
    }
}

Write-Host "All files processed!"
