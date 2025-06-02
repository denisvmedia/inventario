@echo off
REM Ptah Test Runner - CMD Wrapper with Reporting
REM
REM Usage:
REM   test-ptah.cmd                    - Run all tests (unit + integration) with databases
REM   test-ptah.cmd unit               - Run unit tests only (fast, no databases)
REM   test-ptah.cmd integration        - Run integration tests only (with databases)
REM   test-ptah.cmd pattern TestName   - Run specific test pattern
REM   test-ptah.cmd package core/ast   - Test specific package
REM   test-ptah.cmd keep               - Run tests and keep databases
REM
REM Examples:
REM   test-ptah.cmd
REM   test-ptah.cmd unit
REM   test-ptah.cmd integration
REM   test-ptah.cmd pattern TestDropIndex
REM   test-ptah.cmd package core/renderer
REM   test-ptah.cmd keep

setlocal enabledelayedexpansion

REM Check if we're in the ptah directory
if not exist "docker-compose.yaml" (
    echo [ERROR] docker-compose.yaml not found. Please run this script from the ptah directory.
    exit /b 1
)

REM Parse command line arguments
set "UNIT_ONLY="
set "INTEGRATION_ONLY="
set "PATTERN="
set "PACKAGE="
set "KEEP_DATABASES="
set "SKIP_INTEGRATION="

:parse_args
if "%~1"=="" goto :run_tests

if /i "%~1"=="unit" (
    set "UNIT_ONLY=-UnitOnly"
    shift
    goto :parse_args
)

if /i "%~1"=="integration" (
    set "INTEGRATION_ONLY=-IntegrationOnly"
    shift
    goto :parse_args
)

if /i "%~1"=="pattern" (
    if "%~2"=="" (
        echo [ERROR] Pattern argument requires a value
        exit /b 1
    )
    set "PATTERN=-Pattern %~2"
    shift
    shift
    goto :parse_args
)

if /i "%~1"=="package" (
    if "%~2"=="" (
        echo [ERROR] Package argument requires a value
        exit /b 1
    )
    set "PACKAGE=-Package %~2"
    shift
    shift
    goto :parse_args
)

if /i "%~1"=="keep" (
    set "KEEP_DATABASES=-KeepDatabases"
    shift
    goto :parse_args
)

if /i "%~1"=="skipint" (
    set "SKIP_INTEGRATION=-SkipIntegration"
    shift
    goto :parse_args
)

if /i "%~1"=="help" (
    goto :show_help
)

if /i "%~1"=="-h" (
    goto :show_help
)

if /i "%~1"=="--help" (
    goto :show_help
)

echo [ERROR] Unknown argument: %~1
goto :show_help

:run_tests
echo [INFO] Running ptah tests...
echo [INFO] Arguments: %UNIT_ONLY% %INTEGRATION_ONLY% %PATTERN% %PACKAGE% %KEEP_DATABASES% %SKIP_INTEGRATION%

REM Execute PowerShell script
powershell -ExecutionPolicy Bypass -File "test-ptah.ps1" %UNIT_ONLY% %INTEGRATION_ONLY% %PATTERN% %PACKAGE% %KEEP_DATABASES% %SKIP_INTEGRATION%
exit /b %ERRORLEVEL%

:show_help
echo.
echo Ptah Test Runner - CMD Wrapper with Reporting
echo.
echo Usage:
echo   test-ptah.cmd [options]
echo.
echo Options:
echo   unit                    Run unit tests only (no databases, no integration tests)
echo   integration             Run integration tests only (with databases)
echo   pattern ^<name^>          Run tests matching pattern
echo   package ^<path^>          Test specific package (e.g., core/ast)
echo   keep                    Keep databases running after tests
echo   skipint                 Skip integration folder tests (legacy)
echo   help                    Show this help
echo.
echo Examples:
echo   test-ptah.cmd                      # All tests (unit + integration) with databases and reports
echo   test-ptah.cmd unit                 # Unit tests only (fast, no databases)
echo   test-ptah.cmd integration          # Integration tests only (with databases)
echo   test-ptah.cmd pattern TestDropIndex # Specific test pattern
echo   test-ptah.cmd package core/renderer # Specific package
echo   test-ptah.cmd keep                 # Keep databases for debugging
echo.
exit /b 0
