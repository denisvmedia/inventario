#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Comprehensive test runner for ptah with database integration and reporting

.DESCRIPTION
    Runs all Go tests in the ptah directory recursively with proper database setup.
    Automatically starts PostgreSQL, MySQL, and MariaDB using Docker Compose,
    sets up environment variables, runs unit and integration tests separately,
    generates test reports in multiple formats, and cleans up.

.PARAMETER Pattern
    Optional test pattern to run specific tests (e.g., "TestDropIndex", "TestCreateType")

.PARAMETER Package
    Specific package to test (e.g., "core/ast", "core/renderer")

.PARAMETER UnitOnly
    Run only unit tests (no database setup required, no integration tests)

.PARAMETER IntegrationOnly
    Run only integration tests (requires database setup)

.PARAMETER SkipIntegration
    Skip integration folder tests (legacy parameter, use UnitOnly instead)

.PARAMETER KeepDatabases
    Keep databases running after tests complete

.PARAMETER Timeout
    Test timeout in minutes (default: 10)

.PARAMETER ReportsDir
    Directory to store test reports (default: test-reports)

.EXAMPLE
    .\test-ptah.ps1
    Run all tests (unit + integration) with database setup and generate reports

.EXAMPLE
    .\test-ptah.ps1 -UnitOnly
    Run only unit tests (fast, no databases, no integration tests)

.EXAMPLE
    .\test-ptah.ps1 -IntegrationOnly
    Run only integration tests with database setup

.EXAMPLE
    .\test-ptah.ps1 -Pattern "TestDropIndex"
    Run specific tests matching pattern in both unit and integration tests

.EXAMPLE
    .\test-ptah.ps1 -Package "core/renderer"
    Test specific package only (both unit and integration)
#>

param(
    [string]$Pattern = "",
    [string]$Package = "",
    [switch]$UnitOnly,
    [switch]$IntegrationOnly,
    [switch]$SkipIntegration,
    [switch]$KeepDatabases,
    [int]$Timeout = 10,
    [string]$ReportsDir = "test-reports",
    [switch]$Debug
)

$ErrorActionPreference = "Stop"

# Database connection strings
$DatabaseConnections = @{
    POSTGRES_TEST_DSN = "postgres://ptah_user:ptah_password@localhost:5432/ptah_test?sslmode=disable"
    MYSQL_TEST_DSN = "ptah_user:ptah_password@tcp(localhost:3310)/ptah_test"
    MARIADB_TEST_DSN = "ptah_user:ptah_password@tcp(localhost:3307)/ptah_test"
}

function Write-Header {
    param([string]$Message)
    Write-Host ""
    Write-Host ("=" * 70) -ForegroundColor Cyan
    Write-Host "  $Message" -ForegroundColor Cyan
    Write-Host ("=" * 70) -ForegroundColor Cyan
    Write-Host ""
}

function Write-Section {
    param([string]$Message)
    Write-Host ""
    Write-Host ("-" * 50) -ForegroundColor Yellow
    Write-Host "  $Message" -ForegroundColor Yellow
    Write-Host ("-" * 50) -ForegroundColor Yellow
    Write-Host ""
}

function Write-Step {
    param([string]$Message)
    Write-Host "[STEP] $Message" -ForegroundColor Blue
}

function Write-Success {
    param([string]$Message)
    Write-Host "[OK] $Message" -ForegroundColor Green
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARN] $Message" -ForegroundColor Yellow
}

function Initialize-ReportsDirectory {
    Write-Step "Initializing test reports directory: $ReportsDir"

    if (-not (Test-Path $ReportsDir)) {
        New-Item -ItemType Directory -Path $ReportsDir -Force | Out-Null
        Write-Success "Reports directory created: $ReportsDir"
    } else {
        Write-Success "Reports directory already exists: $ReportsDir"
        $existingReports = Get-ChildItem $ReportsDir -File | Measure-Object
        Write-Host "Found $($existingReports.Count) existing report files" -ForegroundColor Gray
    }
}

function Generate-HTMLReport {
    param(
        [string]$JsonFile,
        [string]$OutputFile,
        [string]$TestType
    )

    if (-not (Test-Path $JsonFile)) {
        Write-Warning "JSON file not found: $JsonFile"
        return
    }

    Write-Step "Generating HTML report for $TestType tests..."

    if ($Debug) {
        # Debug: Show first few lines of JSON file
        Write-Host "Debug: First 10 lines of JSON file:" -ForegroundColor Yellow
        Get-Content $JsonFile | Select-Object -First 10 | ForEach-Object { Write-Host "  $_" -ForegroundColor Gray }
    }

    $jsonContent = Get-Content $JsonFile -Raw
    $lines = $jsonContent -split "`n" | Where-Object { $_.Trim() -ne "" }

    if ($Debug) {
        Write-Host "Debug: Total lines in JSON: $($lines.Count)" -ForegroundColor Yellow
    }

    $testEvents = @()
    foreach ($line in $lines) {
        try {
            $event = ConvertFrom-Json $line
            $testEvents += $event
        } catch {
            if ($Debug) {
                Write-Host "Debug: Failed to parse line: $line" -ForegroundColor Red
            }
        }
    }

    if ($Debug) {
        Write-Host "Debug: Parsed $($testEvents.Count) events" -ForegroundColor Yellow
    }

    $testResults = @{}
    $buildFailures = @{}
    $summary = @{
        Total = 0
        Passed = 0
        Failed = 0
        Skipped = 0
        BuildFailed = 0
        Duration = 0
    }

    # Parse test events more carefully
    foreach ($event in $testEvents) {
        # Debug: Show event details for failed tests and build failures
        if ($Debug -and $event.Action -eq "fail") {
            if ($event.Test) {
                Write-Host "Debug: Found failed test: $($event.Test) in package $($event.Package)" -ForegroundColor Red
            } else {
                Write-Host "Debug: Found build failure in package: $($event.Package)" -ForegroundColor Red
            }
        }

        # Handle build failures (no Test field, but has Action=fail)
        if (-not $event.Test -and $event.Action -eq "fail" -and $event.Package) {
            $packageName = $event.Package
            if (-not $buildFailures.ContainsKey($packageName)) {
                $buildFailures[$packageName] = @{
                    Package = $event.Package
                    Action = "fail"
                    Output = @()
                    Elapsed = if ($event.Elapsed) { $event.Elapsed } else { 0 }
                    FailedBuild = $event.FailedBuild
                    Type = "BuildFailure"
                }
                $summary.BuildFailed++
                $summary.Total++
            }

            if ($event.Output) {
                $buildFailures[$packageName].Output += $event.Output
                if ($Debug) {
                    Write-Host "Debug: Adding build failure output for package $packageName`: $($event.Output)" -ForegroundColor Red
                }
            }
        }

        # Handle regular test events
        if ($event.Test -and $event.Action) {
            $testName = $event.Test
            if (-not $testResults.ContainsKey($testName)) {
                $testResults[$testName] = @{
                    Package = $event.Package
                    Action = "run"
                    Output = @()
                    Elapsed = 0
                    StartTime = $null
                    EndTime = $null
                    Type = "Test"
                }
            }

            # Track the final action for this test
            if ($event.Action -in @("pass", "fail", "skip")) {
                $testResults[$testName].Action = $event.Action
                if ($Debug) {
                    Write-Host "Debug: Set test $testName action to $($event.Action)" -ForegroundColor Cyan
                }
            }

            if ($event.Elapsed) {
                $testResults[$testName].Elapsed = $event.Elapsed
            }

            if ($event.Output) {
                $testResults[$testName].Output += $event.Output
                # Debug: Show output for failed tests
                if ($Debug -and ($event.Action -eq "fail" -or $testResults[$testName].Action -eq "fail")) {
                    Write-Host "Debug: Adding output for test $testName`: $($event.Output)" -ForegroundColor Red
                }
            }

            if ($event.Time) {
                if ($event.Action -eq "run") {
                    $testResults[$testName].StartTime = $event.Time
                } elseif ($event.Action -in @("pass", "fail", "skip")) {
                    $testResults[$testName].EndTime = $event.Time
                }
            }
        }
    }

    # Debug: Show final test results
    if ($Debug) {
        Write-Host "Debug: Final test results:" -ForegroundColor Yellow
        foreach ($testName in $testResults.Keys) {
            $test = $testResults[$testName]
            Write-Host "  $testName`: $($test.Action) (Output lines: $($test.Output.Count))" -ForegroundColor Gray
        }

        Write-Host "Debug: Build failures:" -ForegroundColor Yellow
        foreach ($packageName in $buildFailures.Keys) {
            $failure = $buildFailures[$packageName]
            Write-Host "  $packageName`: BUILD FAILED (Output lines: $($failure.Output.Count))" -ForegroundColor Red
        }
    }

    # Calculate summary for regular tests
    foreach ($test in $testResults.Values) {
        $summary.Total++
        $summary.Duration += $test.Elapsed
        switch ($test.Action) {
            "pass" { $summary.Passed++ }
            "fail" { $summary.Failed++ }
            "skip" { $summary.Skipped++ }
        }
    }

    # Build failures are already counted in the parsing loop

    # Generate HTML
    $html = @"
<!DOCTYPE html>
<html>
<head>
    <title>Ptah $TestType Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f8f9fa; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { background: #f5f5f5; padding: 20px; border-radius: 5px; margin-bottom: 20px; border-left: 4px solid #007bff; }
        .summary { display: flex; gap: 20px; margin-bottom: 20px; flex-wrap: wrap; }
        .summary-item { background: #e9ecef; padding: 15px; border-radius: 5px; text-align: center; min-width: 120px; flex: 1; }
        .summary-item h3 { margin: 0; font-size: 2em; }
        .summary-item p { margin: 5px 0 0 0; font-weight: bold; }
        .passed { background: #d4edda; color: #155724; border-left: 4px solid #28a745; }
        .failed { background: #f8d7da; color: #721c24; border-left: 4px solid #dc3545; }
        .skipped { background: #fff3cd; color: #856404; border-left: 4px solid #ffc107; }
        .test-item { border: 1px solid #ddd; margin: 10px 0; border-radius: 5px; overflow: hidden; }
        .test-header { padding: 15px; background: #f8f9fa; cursor: pointer; transition: background-color 0.2s; }
        .test-header:hover { background: #e9ecef; }
        .test-header.passed { border-left: 4px solid #28a745; }
        .test-header.failed { border-left: 4px solid #dc3545; background: #fff5f5; }
        .test-header.skipped { border-left: 4px solid #ffc107; }
        .test-content { padding: 15px; display: none; background: #fff; border-top: 1px solid #ddd; }
        .test-output { background: #f8f9fa; padding: 15px; font-family: 'Courier New', monospace; white-space: pre-wrap; border-radius: 4px; border: 1px solid #e9ecef; max-height: 400px; overflow-y: auto; }
        .show { display: block; }
        .status-badge { float: right; padding: 4px 8px; border-radius: 4px; font-size: 0.8em; font-weight: bold; }
        .status-badge.passed { background: #28a745; color: white; }
        .status-badge.failed { background: #dc3545; color: white; }
        .status-badge.skipped { background: #ffc107; color: #212529; }
        .test-meta { font-size: 0.9em; color: #6c757d; margin-top: 5px; }
        .no-tests { text-align: center; padding: 40px; color: #6c757d; }
    </style>
    <script>
        function toggleTest(id) {
            var content = document.getElementById('content-' + id);
            content.classList.toggle('show');
        }
        function expandAll() {
            var contents = document.querySelectorAll('.test-content');
            contents.forEach(function(content) { content.classList.add('show'); });
        }
        function collapseAll() {
            var contents = document.querySelectorAll('.test-content');
            contents.forEach(function(content) { content.classList.remove('show'); });
        }
    </script>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Ptah $TestType Test Report</h1>
            <p><strong>Generated:</strong> $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')</p>
            <p><strong>Total Duration:</strong> $([math]::Round($summary.Duration, 2)) seconds</p>
        </div>

        <div class="summary">
            <div class="summary-item">
                <h3>$($summary.Total)</h3>
                <p>Total Items</p>
            </div>
            <div class="summary-item passed">
                <h3>$($summary.Passed)</h3>
                <p>Passed</p>
            </div>
            <div class="summary-item failed">
                <h3>$($summary.Failed)</h3>
                <p>Failed Tests</p>
            </div>
            <div class="summary-item failed">
                <h3>$($summary.BuildFailed)</h3>
                <p>Build Failures</p>
            </div>
            <div class="summary-item skipped">
                <h3>$($summary.Skipped)</h3>
                <p>Skipped</p>
            </div>
        </div>

        <div style="margin-bottom: 20px;">
            <button onclick="expandAll()" style="margin-right: 10px; padding: 8px 16px; background: #007bff; color: white; border: none; border-radius: 4px; cursor: pointer;">Expand All</button>
            <button onclick="collapseAll()" style="padding: 8px 16px; background: #6c757d; color: white; border: none; border-radius: 4px; cursor: pointer;">Collapse All</button>
        </div>

        <h2>Test Details</h2>
"@

    if ($testResults.Count -eq 0 -and $buildFailures.Count -eq 0) {
        $html += @"
        <div class="no-tests">
            <h3>No tests or build results found</h3>
            <p>No test results or build failures were parsed from the JSON output.</p>
        </div>
"@
    } else {
        $testIndex = 0

        # Show build failures first
        if ($buildFailures.Count -gt 0) {
            $html += @"
        <h3 style="color: #dc3545; margin-top: 30px;">Build Failures</h3>
"@
            foreach ($packageName in $buildFailures.Keys | Sort-Object) {
                $failure = $buildFailures[$packageName]
                $outputText = if ($failure.Output.Count -gt 0) {
                    $failure.Output -join ""
                } else {
                    "No build output captured."
                }

                $html += @"
        <div class="test-item">
            <div class="test-header failed" onclick="toggleTest($testIndex)">
                <strong>BUILD FAILURE: $packageName</strong>
                <span class="status-badge failed">BUILD FAILED</span>
                <div class="test-meta">
                    <strong>Package:</strong> $($failure.Package) |
                    <strong>Duration:</strong> $([math]::Round($failure.Elapsed, 3))s |
                    <strong>Failed Build:</strong> $($failure.FailedBuild)
                </div>
            </div>
            <div id="content-$testIndex" class="test-content">
                <div class="test-output">$outputText</div>
            </div>
        </div>
"@
                $testIndex++
            }
        }

        # Show regular test results
        if ($testResults.Count -gt 0) {
            $html += @"
        <h3 style="margin-top: 30px;">Test Results</h3>
"@
            foreach ($testName in $testResults.Keys | Sort-Object) {
                $test = $testResults[$testName]
                $statusClass = switch ($test.Action) {
                    "pass" { "passed" }
                    "fail" { "failed" }
                    "skip" { "skipped" }
                    default { "unknown" }
                }

                $statusBadge = switch ($test.Action) {
                    "pass" { "PASSED" }
                    "fail" { "FAILED" }
                    "skip" { "SKIPPED" }
                    default { "UNKNOWN" }
                }

                $outputText = if ($test.Output.Count -gt 0) {
                    $test.Output -join ""
                } else {
                    "No output captured for this test."
                }

                $html += @"
        <div class="test-item">
            <div class="test-header $statusClass" onclick="toggleTest($testIndex)">
                <strong>$testName</strong>
                <span class="status-badge $statusClass">$statusBadge</span>
                <div class="test-meta">
                    <strong>Package:</strong> $($test.Package) |
                    <strong>Duration:</strong> $([math]::Round($test.Elapsed, 3))s
                </div>
            </div>
            <div id="content-$testIndex" class="test-content">
                <div class="test-output">$outputText</div>
            </div>
        </div>
"@
                $testIndex++
            }
        }
    }

    $html += @"
    </div>
</body>
</html>
"@

    $html | Out-File -FilePath $OutputFile -Encoding UTF8
    Write-Success "HTML report generated: $OutputFile"
}

function Test-JSONParsing {
    param([string]$JsonFile)

    if (-not (Test-Path $JsonFile)) {
        Write-Error "JSON file not found: $JsonFile"
        return
    }

    Write-Host "=== JSON Parsing Test ===" -ForegroundColor Cyan
    Write-Host "File: $JsonFile" -ForegroundColor Yellow

    $content = Get-Content $JsonFile -Raw
    $lines = $content -split "`n" | Where-Object { $_.Trim() -ne "" }

    Write-Host "Total lines: $($lines.Count)" -ForegroundColor Yellow

    $events = @()
    $lineNum = 0
    foreach ($line in $lines) {
        $lineNum++
        try {
            $event = ConvertFrom-Json $line
            $events += $event
            if ($event.Action -eq "fail") {
                Write-Host "Line $lineNum - FAILED TEST: $($event.Test)" -ForegroundColor Red
                Write-Host "  Package: $($event.Package)" -ForegroundColor Gray
                Write-Host "  Output: $($event.Output)" -ForegroundColor Gray
            }
        } catch {
            Write-Host "Line $lineNum - Parse error: $line" -ForegroundColor Red
        }
    }

    Write-Host "Total events parsed: $($events.Count)" -ForegroundColor Yellow

    $failedTests = $events | Where-Object { $_.Action -eq "fail" -and $_.Test }
    $buildFailures = $events | Where-Object { $_.Action -eq "fail" -and -not $_.Test }

    Write-Host "Failed test events: $($failedTests.Count)" -ForegroundColor Red
    Write-Host "Build failure events: $($buildFailures.Count)" -ForegroundColor Red

    foreach ($failed in $failedTests) {
        Write-Host "FAILED TEST: $($failed.Test) in $($failed.Package)" -ForegroundColor Red
    }

    foreach ($failed in $buildFailures) {
        Write-Host "BUILD FAILURE: $($failed.Package)" -ForegroundColor Red
        if ($failed.FailedBuild) {
            Write-Host "  Failed Build: $($failed.FailedBuild)" -ForegroundColor Gray
        }
    }
}

function Test-Prerequisites {
    Write-Step "Checking prerequisites..."
    
    # Check if we're in the ptah directory
    if (-not (Test-Path "docker-compose.yaml")) {
        Write-Error "docker-compose.yaml not found. Please run this script from the ptah directory."
        exit 1
    }
    
    # Check Go
    try {
        $goVersion = go version
        Write-Success "Go found: $goVersion"
    } catch {
        Write-Error "Go not found. Please install Go."
        exit 1
    }
    
    # Check Docker (only if not unit-only)
    if (-not $UnitOnly) {
        try {
            docker --version | Out-Null
            docker-compose --version | Out-Null
            Write-Success "Docker and docker-compose found"
        } catch {
            Write-Error "Docker or docker-compose not found. Please install Docker Desktop."
            exit 1
        }
    }
}

function Start-Databases {
    if ($UnitOnly) {
        Write-Warning "Skipping database setup (unit tests only)"
        return
    }

    Write-Step "Starting databases (PostgreSQL, MySQL, MariaDB)..."
    docker-compose up -d postgres mysql mariadb
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Failed to start databases"
        exit 1
    }
    
    Write-Step "Waiting for databases to be healthy..."
    $maxWait = 60
    $waited = 0
    $interval = 3
    
    do {
        Start-Sleep $interval
        $waited += $interval
        
        try {
            $status = docker-compose ps --format json | ConvertFrom-Json
            $healthy = $true
            
            foreach ($service in $status) {
                if ($service.Service -in @("postgres", "mysql", "mariadb")) {
                    if ($service.Health -ne "healthy") {
                        $healthy = $false
                        Write-Host "." -NoNewline
                        break
                    }
                }
            }
            
            if ($healthy) {
                Write-Host ""
                Write-Success "All databases are healthy!"
                break
            }
        } catch {
            Write-Host "." -NoNewline
        }
        
        if ($waited -ge $maxWait) {
            Write-Host ""
            Write-Error "Timeout waiting for databases to be healthy"
            Write-Warning "Database logs:"
            docker-compose logs --tail=20 postgres mysql mariadb
            exit 1
        }
    } while ($true)
}

function Set-TestEnvironment {
    if ($UnitOnly) {
        Write-Step "Setting up unit test environment..."
        return
    }
    
    Write-Step "Setting up test environment variables..."
    foreach ($key in $DatabaseConnections.Keys) {
        Set-Item -Path "env:$key" -Value $DatabaseConnections[$key]
        Write-Host "  $key = $($DatabaseConnections[$key])" -ForegroundColor Gray
    }
}

function Stop-Databases {
    if ($UnitOnly -or $KeepDatabases) {
        if ($KeepDatabases) {
            Write-Warning "Keeping databases running (use 'docker-compose down' to stop them)"
            Write-Host "Database connections:" -ForegroundColor Yellow
            foreach ($key in $DatabaseConnections.Keys) {
                Write-Host "  $key = $($DatabaseConnections[$key])" -ForegroundColor Gray
            }
        }
        return
    }

    Write-Step "Stopping and removing databases..."
    docker-compose down
    if ($LASTEXITCODE -eq 0) {
        Write-Success "Databases stopped and removed"
    } else {
        Write-Warning "Failed to stop databases cleanly"
    }
}

function Run-UnitTests {
    Write-Section "Running Unit Tests"

    # Find all integration test files to exclude them
    $integrationTestFiles = @()
    if ($Package) {
        $searchPath = "./$Package"
        Write-Host "Searching for unit tests in package: $Package (excluding integration tests)" -ForegroundColor Cyan
    } else {
        $searchPath = "."
        Write-Host "Searching for unit tests in all packages (excluding integration tests and gonative folder)" -ForegroundColor Cyan
    }

    # Find all *_integration_test.go files to exclude
    $integrationTestFiles = Get-ChildItem -Path $searchPath -Recurse -Filter "*_integration_test.go" | ForEach-Object { $_.FullName }

    if ($integrationTestFiles.Count -gt 0) {
        Write-Host "Found $($integrationTestFiles.Count) integration test file(s) to exclude:" -ForegroundColor Yellow
        foreach ($file in $integrationTestFiles) {
            $relativePath = Resolve-Path -Path $file -Relative
            Write-Host "  - $relativePath" -ForegroundColor Gray
        }
    }

    # Build test command for unit tests (exclude integration tests and gonative folder)
    $testArgs = @("test")

    # Add package specification - exclude integration/gonative folder
    if ($Package) {
        $testArgs += "./$Package/..."
    } else {
        # Use go list to get all packages but exclude integration/gonative
        try {
            $allPackages = & go list ./... 2>$null | Where-Object { $_ -notlike "*integration/gonative*" }
            if ($allPackages.Count -gt 0) {
                # Add each package individually
                foreach ($pkg in $allPackages) {
                    $testArgs += $pkg
                }
            } else {
                # Fallback to ./... if go list fails
                $testArgs += "./..."
            }
        } catch {
            # Fallback to ./... if go list command fails
            Write-Warning "Failed to get package list, using ./... (gonative tests may be included)"
            $testArgs += "./..."
        }
    }

    # Add test pattern if specified
    if ($Pattern) {
        $testArgs += "-run"
        $testArgs += $Pattern
        Write-Host "Test pattern: $Pattern" -ForegroundColor Cyan
    }

    # Add verbose output and JSON for reporting
    $testArgs += "-v"
    $testArgs += "-json"

    # Add timeout
    $testArgs += "-timeout"
    $testArgs += "${Timeout}m"

    # Output files with timestamp
    $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $jsonOutput = Join-Path $ReportsDir "unit-tests-$timestamp.json"
    $textOutput = Join-Path $ReportsDir "unit-tests-$timestamp.txt"

    Write-Host ""
    Write-Step "Executing: go $($testArgs -join ' ')"
    Write-Host "Note: Unit tests exclude files matching *_integration_test.go pattern" -ForegroundColor Yellow
    Write-Host ""

    # Run tests
    $startTime = Get-Date

    # Execute go test and capture output properly
    # Note: Go will naturally exclude integration test files since they have build tags
    # and we're not specifying the integration tag
    if ($Debug) {
        Write-Host "Debug: Running command: go $($testArgs -join ' ')" -ForegroundColor Yellow
    }

    # Use Invoke-Expression to capture output properly
    try {
        $output = & go $testArgs 2>&1
        $testExitCode = $LASTEXITCODE

        # Write JSON output to file
        $output | Out-File -FilePath $jsonOutput -Encoding UTF8

        # Also write to text file for debugging
        $output | Out-File -FilePath $textOutput -Encoding UTF8

        if ($Debug) {
            Write-Host "Debug: Command exit code: $testExitCode" -ForegroundColor Yellow
            Write-Host "Debug: Output lines captured: $($output.Count)" -ForegroundColor Yellow
        }

    } catch {
        Write-Error "Failed to execute go test: $_"
        $testExitCode = 1
    }

    $endTime = Get-Date
    $duration = $endTime - $startTime

    Write-Host ""
    Write-Host "Unit test execution completed in $($duration.ToString('mm\:ss'))" -ForegroundColor Blue

    # Generate HTML report
    $htmlOutput = Join-Path $ReportsDir "unit-tests-$timestamp.html"
    Generate-HTMLReport -JsonFile $jsonOutput -OutputFile $htmlOutput -TestType "Unit"

    # Track generated reports
    $script:newlyGeneratedReports += @(
        "unit-tests-$timestamp.json",
        "unit-tests-$timestamp.txt",
        "unit-tests-$timestamp.html"
    )

    return $testExitCode
}

function Run-IntegrationTests {
    Write-Section "Running Integration Tests"

    # Only run integration tests from the gonative folder
    $gonativeSearchPath = "./integration/gonative"
    if ($Package) {
        # If a specific package is requested, check if it's the gonative folder or contains it
        if ($Package -like "*integration/gonative*" -or $Package -like "*integration\gonative*") {
            $gonativePackage = "./$Package"
            Write-Host "Running integration tests in gonative package: $Package" -ForegroundColor Cyan
        } else {
            Write-Warning "Integration tests are only available in the integration/gonative folder. Skipping."
            return 0
        }
    } else {
        $gonativePackage = $gonativeSearchPath
        Write-Host "Running integration tests in gonative folder: $gonativeSearchPath" -ForegroundColor Cyan
    }

    # Check if gonative folder exists
    if (-not (Test-Path $gonativePackage)) {
        Write-Warning "Integration test folder not found: $gonativePackage"
        return 0
    }

    # Build test command for integration tests in gonative folder
    $startTime = Get-Date
    $allOutput = @()
    $overallExitCode = 0

    # Build test command for gonative package
    $testArgs = @("test")

    # Add the gonative package
    $testArgs += $gonativePackage

    # Add build tags for integration tests
    $testArgs += "-tags"
    $testArgs += "integration"

    # Add verbose output and JSON for reporting
    $testArgs += "-v"
    $testArgs += "-json"

    # Add timeout
    $testArgs += "-timeout"
    $testArgs += "${Timeout}m"

    # Add test pattern if specified
    if ($Pattern) {
        $testArgs += "-run"
        $testArgs += $Pattern
    }

    Write-Host ""
    Write-Host "Executing: go $($testArgs -join ' ')" -ForegroundColor Yellow
    Write-Host ""

    # Run tests for gonative package
    try {
        $output = & go $testArgs 2>&1
        $exitCode = $LASTEXITCODE

        if ($exitCode -ne 0) {
            $overallExitCode = $exitCode
        }

        $allOutput += $output

        if ($Debug) {
            Write-Host "Debug: Gonative package exit code: $exitCode" -ForegroundColor Yellow
        }

    } catch {
        Write-Error "Failed to execute go test for gonative package: $_"
        $overallExitCode = 1
    }

    # Output files with timestamp
    $timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
    $jsonOutput = Join-Path $ReportsDir "integration-tests-$timestamp.json"
    $textOutput = Join-Path $ReportsDir "integration-tests-$timestamp.txt"

    # Write combined output to files
    $allOutput | Out-File -FilePath $jsonOutput -Encoding UTF8
    $allOutput | Out-File -FilePath $textOutput -Encoding UTF8

    if ($Debug) {
        Write-Host "Debug: Overall exit code: $overallExitCode" -ForegroundColor Yellow
        Write-Host "Debug: Total output lines captured: $($allOutput.Count)" -ForegroundColor Yellow
    }

    $endTime = Get-Date
    $duration = $endTime - $startTime

    Write-Host ""
    Write-Host "Integration test execution completed in $($duration.ToString('mm\:ss'))" -ForegroundColor Blue

    # Generate HTML report
    $htmlOutput = Join-Path $ReportsDir "integration-tests-$timestamp.html"
    Generate-HTMLReport -JsonFile $jsonOutput -OutputFile $htmlOutput -TestType "Integration"

    # Track generated reports
    $script:newlyGeneratedReports += @(
        "integration-tests-$timestamp.json",
        "integration-tests-$timestamp.txt",
        "integration-tests-$timestamp.html"
    )

    return $overallExitCode
}

# Main execution
try {
    $startTime = Get-Date

    # Track newly generated reports
    $newlyGeneratedReports = @()

    Write-Header "Ptah Comprehensive Test Runner with Reporting"

    # Determine test mode
    $testMode = if ($UnitOnly) {
        'Unit Tests Only'
    } elseif ($IntegrationOnly) {
        'Integration Tests Only'
    } else {
        'Unit + Integration Tests'
    }

    Write-Host "Mode: $testMode" -ForegroundColor Cyan
    Write-Host "Pattern: $(if ($Pattern) { $Pattern } else { 'All tests' })" -ForegroundColor Cyan
    Write-Host "Package: $(if ($Package) { $Package } else { 'All packages' })" -ForegroundColor Cyan
    Write-Host "Timeout: $Timeout minutes" -ForegroundColor Cyan
    Write-Host "Reports Directory: $ReportsDir" -ForegroundColor Cyan

    # Check prerequisites
    Test-Prerequisites

    # Initialize reports directory
    Initialize-ReportsDirectory

    # Start databases if needed (not for unit-only tests)
    Start-Databases

    # Set up environment
    Set-TestEnvironment

    # Run tests based on mode
    $unitResult = 0
    $integrationResult = 0

    if (-not $IntegrationOnly) {
        $unitResult = Run-UnitTests
    }

    if (-not $UnitOnly) {
        $integrationResult = Run-IntegrationTests
    }

    # Calculate overall result
    $overallResult = [Math]::Max($unitResult, $integrationResult)

    # Results
    $endTime = Get-Date
    $totalDuration = $endTime - $startTime

    Write-Header "Test Results Summary"

    if (-not $IntegrationOnly) {
        if ($unitResult -eq 0) {
            Write-Success "Unit tests: PASSED"
        } else {
            Write-Error "Unit tests: FAILED (exit code: $unitResult)"
        }
    }

    if (-not $UnitOnly) {
        if ($integrationResult -eq 0) {
            Write-Success "Integration tests: PASSED"
        } else {
            Write-Error "Integration tests: FAILED (exit code: $integrationResult)"
        }
    }

    Write-Host ""
    Write-Host "Total duration: $($totalDuration.ToString('mm\:ss'))" -ForegroundColor Blue
    Write-Host "Reports generated in: $ReportsDir" -ForegroundColor Yellow

    # List newly generated reports
    if ($newlyGeneratedReports.Count -gt 0) {
        Write-Host ""
        Write-Host "Generated reports:" -ForegroundColor Yellow
        foreach ($report in $newlyGeneratedReports) {
            Write-Host "  - $report" -ForegroundColor Gray
        }
    }

    if ($overallResult -ne 0) {
        Write-Host ""
        Write-Error "Some tests failed. Check the reports for details."
        exit $overallResult
    } else {
        Write-Host ""
        Write-Success "All tests passed!"
    }

} catch {
    Write-Error "An error occurred: $_"
    exit 1
} finally {
    # Always try to clean up databases
    Stop-Databases
}
