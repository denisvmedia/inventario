# PowerShell script to fix test files after ID generation security changes

$testFiles = @(
    "go/apiserver/security_test.go",
    "go/apiserver/system_test.go",
    "go/apiserver/uploads_test.go"
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

        # Write the updated content back to the file
        Set-Content $file -Value $content -NoNewline

        Write-Host "Updated $file successfully"
    } else {
        Write-Host "File $file not found, skipping..."
    }
}

Write-Host "All remaining test files updated!"
