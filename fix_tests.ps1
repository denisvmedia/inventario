# PowerShell script to fix test files after ID generation security changes

$testFiles = @(
    "go/apiserver/commodities_test.go",
    "go/apiserver/commodities_download_test.go",
    "go/apiserver/locations_test.go",
    "go/apiserver/files_test.go",
    "go/apiserver/exports_test.go",
    "go/apiserver/settings_test.go",
    "go/apiserver/commodity_recursive_delete_integration_test.go",
    "go/apiserver/debug_test.go",
    "go/apiserver/export_restores_integration_test.go"
)

foreach ($file in $testFiles) {
    if (Test-Path $file) {
        Write-Host "Updating $file..."
        
        # Read the file content
        $content = Get-Content $file -Raw
        
        # Replace params := newParams() with params, testUser := newParams()
        $content = $content -replace 'params := newParams\(\)', 'params, testUser := newParams()'
        
        # Replace addTestUserAuthHeader(req) with addTestUserAuthHeader(req, testUser.ID)
        $content = $content -replace 'addTestUserAuthHeader\(req\)', 'addTestUserAuthHeader(req, testUser.ID)'
        
        # Replace params := newParamsAreaRegistryOnly() with params, testUser := newParamsAreaRegistryOnly()
        $content = $content -replace 'params := newParamsAreaRegistryOnly\(\)', 'params, testUser := newParamsAreaRegistryOnly()'
        
        # Write the updated content back to the file
        Set-Content $file -Value $content -NoNewline
        
        Write-Host "Updated $file successfully"
    } else {
        Write-Host "File $file not found, skipping..."
    }
}

Write-Host "All test files updated!"
