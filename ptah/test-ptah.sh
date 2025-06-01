#!/bin/bash
# Ptah Test Runner - Bash Script with Reporting
#
# Usage:
#   ./test-ptah.sh                    - Run all tests (unit + integration) with databases
#   ./test-ptah.sh unit               - Run unit tests only (fast, no databases)
#   ./test-ptah.sh integration        - Run integration tests only (with databases)
#   ./test-ptah.sh pattern TestName   - Run specific test pattern
#   ./test-ptah.sh package core/ast   - Test specific package
#   ./test-ptah.sh keep               - Run tests and keep databases

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Database connection strings
export POSTGRES_TEST_DSN="postgres://ptah_user:ptah_password@localhost:5432/ptah_test?sslmode=disable"
export MYSQL_TEST_DSN="ptah_user:ptah_password@tcp(localhost:3310)/ptah_test"
export MARIADB_TEST_DSN="ptah_user:ptah_password@tcp(localhost:3307)/ptah_test"

# Default reports directory
REPORTS_DIR="test-reports"

function print_header() {
    echo ""
    echo -e "${CYAN}======================================================================${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}======================================================================${NC}"
    echo ""
}

function print_section() {
    echo ""
    echo -e "${YELLOW}--------------------------------------------------${NC}"
    echo -e "${YELLOW}  $1${NC}"
    echo -e "${YELLOW}--------------------------------------------------${NC}"
    echo ""
}

function print_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

function print_success() {
    echo -e "${GREEN}[OK]${NC} $1"
}

function print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

function print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

function initialize_reports_directory() {
    print_step "Initializing test reports directory: $REPORTS_DIR"

    if [ ! -d "$REPORTS_DIR" ]; then
        mkdir -p "$REPORTS_DIR"
        print_success "Reports directory created: $REPORTS_DIR"
    else
        print_success "Reports directory already exists: $REPORTS_DIR"
        local existing_count=$(find "$REPORTS_DIR" -type f | wc -l)
        echo -e "${NC}Found $existing_count existing report files${NC}"
    fi
}

function generate_html_report() {
    local json_file="$1"
    local output_file="$2"
    local test_type="$3"

    if [ ! -f "$json_file" ]; then
        print_warning "JSON file not found: $json_file"
        return
    fi

    print_step "Generating HTML report for $test_type tests..."

    # Parse JSON to extract test results and build failures
    local total_items=0
    local passed_tests=0
    local failed_tests=0
    local skipped_tests=0
    local build_failures=0

    # Count test results and build failures from JSON
    if command -v jq &> /dev/null; then
        # Use jq if available for better JSON parsing
        passed_tests=$(jq -r 'select(.Action=="pass" and .Test) | .Test' "$json_file" 2>/dev/null | wc -l)
        failed_tests=$(jq -r 'select(.Action=="fail" and .Test) | .Test' "$json_file" 2>/dev/null | wc -l)
        skipped_tests=$(jq -r 'select(.Action=="skip" and .Test) | .Test' "$json_file" 2>/dev/null | wc -l)
        build_failures=$(jq -r 'select(.Action=="fail" and (.Test | not)) | .Package' "$json_file" 2>/dev/null | wc -l)
    else
        # Fallback to grep (less precise but works)
        passed_tests=$(grep '"Action":"pass"' "$json_file" | grep '"Test":' | wc -l 2>/dev/null || echo "0")
        failed_tests=$(grep '"Action":"fail"' "$json_file" | grep '"Test":' | wc -l 2>/dev/null || echo "0")
        skipped_tests=$(grep '"Action":"skip"' "$json_file" | grep '"Test":' | wc -l 2>/dev/null || echo "0")
        build_failures=$(grep '"Action":"fail"' "$json_file" | grep -v '"Test":' | wc -l 2>/dev/null || echo "0")
    fi

    total_items=$((passed_tests + failed_tests + skipped_tests + build_failures))

    # Enhanced HTML report generation
    cat > "$output_file" << EOF
<!DOCTYPE html>
<html>
<head>
    <title>Ptah $test_type Test Report</title>
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
        .test-output { background: #f8f9fa; padding: 15px; font-family: 'Courier New', monospace; white-space: pre-wrap; border-radius: 4px; border: 1px solid #e9ecef; max-height: 600px; overflow-y: auto; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Ptah $test_type Test Report</h1>
            <p><strong>Generated:</strong> $(date)</p>
        </div>

        <div class="summary">
            <div class="summary-item">
                <h3>$total_items</h3>
                <p>Total Items</p>
            </div>
            <div class="summary-item passed">
                <h3>$passed_tests</h3>
                <p>Passed</p>
            </div>
            <div class="summary-item failed">
                <h3>$failed_tests</h3>
                <p>Failed Tests</p>
            </div>
            <div class="summary-item failed">
                <h3>$build_failures</h3>
                <p>Build Failures</p>
            </div>
            <div class="summary-item skipped">
                <h3>$skipped_tests</h3>
                <p>Skipped</p>
            </div>
        </div>

        <h2>Test Details</h2>

        $(if [ "$build_failures" -gt 0 ]; then
            echo '<h3 style="color: #dc3545; margin-top: 30px;">Build Failures</h3>'
            if command -v jq &> /dev/null; then
                jq -r 'select(.Action=="fail" and (.Test | not)) | "<div style=\"border: 1px solid #dc3545; margin: 10px 0; padding: 15px; border-radius: 5px; background: #fff5f5;\"><strong>BUILD FAILURE:</strong> " + .Package + "<br><pre style=\"margin-top: 10px; background: #f8f9fa; padding: 10px; border-radius: 4px;\">" + (.Output // "No output") + "</pre></div>"' "$json_file" 2>/dev/null
            else
                echo '<div style="border: 1px solid #dc3545; margin: 10px 0; padding: 15px; border-radius: 5px; background: #fff5f5;"><strong>BUILD FAILURES DETECTED</strong><br>See raw output below for details.</div>'
            fi
        fi)

        $(if [ "$failed_tests" -gt 0 ] || [ "$passed_tests" -gt 0 ] || [ "$skipped_tests" -gt 0 ]; then
            echo '<h3 style="margin-top: 30px;">Test Results Summary</h3>'
            echo '<div style="background: #f8f9fa; padding: 15px; border-radius: 5px; margin: 10px 0;">'
            echo "<p><strong>Passed:</strong> $passed_tests | <strong>Failed:</strong> $failed_tests | <strong>Skipped:</strong> $skipped_tests</p>"
            echo '</div>'
        fi)

        <h3 style="margin-top: 30px;">Raw Test Output</h3>
        <div class="test-output">$(cat "$json_file" 2>/dev/null | head -1000 || echo "No test output available")</div>
    </div>
</body>
</html>
EOF

    print_success "HTML report generated: $output_file"
}

function show_help() {
    echo ""
    echo "Ptah Test Runner - Bash Script with Reporting"
    echo ""
    echo "Usage:"
    echo "  ./test-ptah.sh [options]"
    echo ""
    echo "Options:"
    echo "  unit                    Run unit tests only (no databases, no integration tests)"
    echo "  integration             Run integration tests only (with databases)"
    echo "  pattern <name>          Run tests matching pattern"
    echo "  package <path>          Test specific package (e.g., core/ast)"
    echo "  keep                    Keep databases running after tests"
    echo "  debug                   Enable debug output for troubleshooting"
    echo "  help                    Show this help"
    echo ""
    echo "Examples:"
    echo "  ./test-ptah.sh                      # All tests (unit + integration) with databases and reports"
    echo "  ./test-ptah.sh unit                 # Unit tests only (fast, no databases)"
    echo "  ./test-ptah.sh integration          # Integration tests only (with databases)"
    echo "  ./test-ptah.sh pattern TestDropIndex # Specific test pattern"
    echo "  ./test-ptah.sh package core/renderer # Specific package"
    echo "  ./test-ptah.sh keep                 # Keep databases for debugging"
    echo ""
    exit 0
}

function check_prerequisites() {
    print_step "Checking prerequisites..."
    
    # Check if we're in the ptah directory
    if [ ! -f "docker-compose.yaml" ]; then
        print_error "docker-compose.yaml not found. Please run this script from the ptah directory."
        exit 1
    fi
    
    # Check Go
    if ! command -v go &> /dev/null; then
        print_error "Go not found. Please install Go."
        exit 1
    fi
    
    GO_VERSION=$(go version)
    print_success "Go found: $GO_VERSION"
    
    # Check Docker (only if not unit-only)
    if [ "$UNIT_ONLY" != "true" ]; then
        if ! command -v docker &> /dev/null || ! command -v docker-compose &> /dev/null; then
            print_error "Docker or docker-compose not found. Please install Docker."
            exit 1
        fi
        print_success "Docker and docker-compose found"
    fi
}

function start_databases() {
    if [ "$UNIT_ONLY" = "true" ]; then
        print_warning "Skipping database setup (unit tests only)"
        return
    fi
    
    print_step "Starting databases (PostgreSQL, MySQL, MariaDB)..."
    docker-compose up -d postgres mysql mariadb
    
    print_step "Waiting for databases to be healthy..."
    local max_wait=60
    local waited=0
    local interval=3
    
    while [ $waited -lt $max_wait ]; do
        sleep $interval
        waited=$((waited + interval))
        
        # Check if all databases are healthy
        local status=$(docker-compose ps --format json 2>/dev/null || echo "[]")
        
        if [ "$status" != "[]" ]; then
            # Simple check - if containers are running, assume healthy after initial wait
            if [ $waited -gt 15 ]; then
                print_success "Databases should be ready!"
                break
            fi
        fi
        
        printf "."
    done
    
    if [ $waited -ge $max_wait ]; then
        echo ""
        print_error "Timeout waiting for databases"
        exit 1
    fi
    echo ""
}

function stop_databases() {
    if [ "$UNIT_ONLY" = "true" ] || [ "$KEEP_DATABASES" = "true" ]; then
        if [ "$KEEP_DATABASES" = "true" ]; then
            print_warning "Keeping databases running (use 'docker-compose down' to stop them)"
            echo -e "${YELLOW}Database connections:${NC}"
            echo "  POSTGRES_TEST_DSN = $POSTGRES_TEST_DSN"
            echo "  MYSQL_TEST_DSN = $MYSQL_TEST_DSN"
            echo "  MARIADB_TEST_DSN = $MARIADB_TEST_DSN"
        fi
        return
    fi
    
    print_step "Stopping and removing databases..."
    docker-compose down
    if [ $? -eq 0 ]; then
        print_success "Databases stopped and removed"
    else
        print_warning "Failed to stop databases cleanly"
    fi
}

function run_unit_tests() {
    print_section "Running Unit Tests"

    # Find all integration test files to exclude them
    local search_path
    if [ -n "$PACKAGE" ]; then
        search_path="./$PACKAGE"
        echo -e "${CYAN}Searching for unit tests in package: $PACKAGE (excluding integration tests)${NC}"
    else
        search_path="."
        echo -e "${CYAN}Searching for unit tests in all packages (excluding integration tests and gonative folder)${NC}"
    fi

    # Find all *_integration_test.go files to exclude
    local integration_files
    integration_files=$(find "$search_path" -name "*_integration_test.go" -type f 2>/dev/null)

    if [ -n "$integration_files" ]; then
        local file_count=$(echo "$integration_files" | wc -l)
        echo -e "${YELLOW}Found $file_count integration test file(s) to exclude:${NC}"
        echo "$integration_files" | while read -r file; do
            echo -e "${NC}  - $file${NC}"
        done
    fi

    # Build test command for unit tests (exclude integration tests and gonative folder)
    local test_args=("test")

    # Add package specification - exclude integration/gonative folder
    if [ -n "$PACKAGE" ]; then
        test_args+=("./$PACKAGE/...")
    else
        # Use go list to get all packages but exclude integration/gonative
        local all_packages
        all_packages=$(go list ./... 2>/dev/null | grep -v "integration/gonative" || echo "./...")

        if [ "$all_packages" = "./..." ]; then
            # Fallback to ./... if go list fails
            test_args+=("./...")
        else
            # Add each package individually
            while IFS= read -r pkg; do
                if [[ "$pkg" != *"integration/gonative"* ]]; then
                    test_args+=("$pkg")
                fi
            done <<< "$all_packages"
        fi
    fi

    # Add test pattern if specified
    if [ -n "$PATTERN" ]; then
        test_args+=("-run" "$PATTERN")
        echo -e "${CYAN}Test pattern: $PATTERN${NC}"
    fi

    # Add verbose output, JSON for reporting, and timeout
    test_args+=("-v" "-json" "-timeout" "10m")

    # Output files with timestamp
    local timestamp=$(date +"%Y%m%d-%H%M%S")
    local json_output="$REPORTS_DIR/unit-tests-$timestamp.json"
    local text_output="$REPORTS_DIR/unit-tests-$timestamp.txt"

    echo ""
    print_step "Executing: go ${test_args[*]}"
    echo -e "${YELLOW}Note: Unit tests exclude files matching *_integration_test.go pattern${NC}"
    echo ""

    # Run tests
    local start_time=$(date +%s)
    # Note: Go will naturally exclude integration test files since they have build tags
    # and we're not specifying the integration tag
    go "${test_args[@]}" > "$json_output" 2> "$text_output"
    local test_exit_code=$?
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    echo ""
    echo -e "${BLUE}Unit test execution completed in ${duration}s${NC}"

    # Generate HTML report
    local html_output="$REPORTS_DIR/unit-tests-$timestamp.html"
    generate_html_report "$json_output" "$html_output" "Unit"

    # Track generated reports (add to global array)
    newly_generated_reports+=("unit-tests-$timestamp.json")
    newly_generated_reports+=("unit-tests-$timestamp.txt")
    newly_generated_reports+=("unit-tests-$timestamp.html")

    return $test_exit_code
}

function run_integration_tests() {
    print_section "Running Integration Tests"

    # Only run integration tests from the gonative folder
    local gonative_search_path="./integration/gonative"
    if [ -n "$PACKAGE" ]; then
        # If a specific package is requested, check if it's the gonative folder or contains it
        if [[ "$PACKAGE" == *"integration/gonative"* ]]; then
            local gonative_package="./$PACKAGE"
            echo -e "${CYAN}Running integration tests in gonative package: $PACKAGE${NC}"
        else
            print_warning "Integration tests are only available in the integration/gonative folder. Skipping."
            return 0
        fi
    else
        local gonative_package="$gonative_search_path"
        echo -e "${CYAN}Running integration tests in gonative folder: $gonative_search_path${NC}"
    fi

    # Check if gonative folder exists
    if [ ! -d "$gonative_package" ]; then
        print_warning "Integration test folder not found: $gonative_package"
        return 0
    fi

    # Build test command for integration tests in gonative folder
    local start_time=$(date +%s)
    local all_output=""
    local overall_exit_code=0

    # Output files with timestamp
    local timestamp=$(date +"%Y%m%d-%H%M%S")
    local json_output="$REPORTS_DIR/integration-tests-$timestamp.json"
    local text_output="$REPORTS_DIR/integration-tests-$timestamp.txt"

    # Build test command for gonative package
    local test_args=("test")

    # Add the gonative package
    test_args+=("$gonative_package")

    # Add build tags for integration tests
    test_args+=("-tags" "integration")

    # Add verbose output, JSON for reporting, and timeout
    test_args+=("-v" "-json" "-timeout" "10m")

    # Add test pattern if specified
    if [ -n "$PATTERN" ]; then
        test_args+=("-run" "$PATTERN")
    fi

    echo ""
    print_step "Executing: go ${test_args[*]}"
    echo ""

    # Run tests for gonative package
    local package_output
    package_output=$(go "${test_args[@]}" 2>&1)
    local exit_code=$?

    if [ $exit_code -ne 0 ]; then
        overall_exit_code=$exit_code
    fi

    all_output="$package_output"

    if [ "$DEBUG" = "true" ]; then
        echo -e "${YELLOW}Debug: Gonative package exit code: $exit_code${NC}"
    fi

    # Write combined output to files
    echo "$all_output" > "$json_output"
    echo "$all_output" > "$text_output"

    if [ "$DEBUG" = "true" ]; then
        echo -e "${YELLOW}Debug: Overall exit code: $overall_exit_code${NC}"
        echo -e "${YELLOW}Debug: Total output lines captured: $(echo "$all_output" | wc -l)${NC}"
    fi

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    echo ""
    echo -e "${BLUE}Integration test execution completed in ${duration}s${NC}"

    # Generate HTML report
    local html_output="$REPORTS_DIR/integration-tests-$timestamp.html"
    generate_html_report "$json_output" "$html_output" "Integration"

    # Track generated reports (add to global array)
    newly_generated_reports+=("integration-tests-$timestamp.json")
    newly_generated_reports+=("integration-tests-$timestamp.txt")
    newly_generated_reports+=("integration-tests-$timestamp.html")

    return $overall_exit_code
}

# Parse command line arguments
UNIT_ONLY=""
INTEGRATION_ONLY=""
PATTERN=""
PACKAGE=""
KEEP_DATABASES=""
DEBUG=""

while [[ $# -gt 0 ]]; do
    case $1 in
        unit)
            UNIT_ONLY="true"
            shift
            ;;
        integration)
            INTEGRATION_ONLY="true"
            shift
            ;;
        pattern)
            if [ -z "$2" ]; then
                print_error "Pattern argument requires a value"
                exit 1
            fi
            PATTERN="$2"
            shift 2
            ;;
        package)
            if [ -z "$2" ]; then
                print_error "Package argument requires a value"
                exit 1
            fi
            PACKAGE="$2"
            shift 2
            ;;
        keep)
            KEEP_DATABASES="true"
            shift
            ;;
        debug)
            DEBUG="true"
            shift
            ;;
        help|-h|--help)
            show_help
            ;;
        *)
            print_error "Unknown argument: $1"
            show_help
            ;;
    esac
done

# Global array to track newly generated reports
declare -a newly_generated_reports=()

# Main execution
main() {
    local start_time=$(date +%s)

    print_header "Ptah Comprehensive Test Runner with Reporting"

    # Determine test mode
    local test_mode
    if [ "$UNIT_ONLY" = "true" ]; then
        test_mode="Unit Tests Only"
    elif [ "$INTEGRATION_ONLY" = "true" ]; then
        test_mode="Integration Tests Only"
    else
        test_mode="Unit + Integration Tests"
    fi

    echo -e "${CYAN}Mode: $test_mode${NC}"
    echo -e "${CYAN}Pattern: ${PATTERN:-All tests}${NC}"
    echo -e "${CYAN}Package: ${PACKAGE:-All packages}${NC}"
    echo -e "${CYAN}Reports Directory: $REPORTS_DIR${NC}"

    # Set up cleanup trap
    trap stop_databases EXIT

    # Check prerequisites
    check_prerequisites

    # Initialize reports directory
    initialize_reports_directory

    # Start databases if needed (not for unit-only tests)
    start_databases

    # Run tests based on mode
    local unit_result=0
    local integration_result=0

    if [ "$INTEGRATION_ONLY" != "true" ]; then
        unit_result=$(run_unit_tests; echo $?)
    fi

    if [ "$UNIT_ONLY" != "true" ]; then
        integration_result=$(run_integration_tests; echo $?)
    fi

    # Calculate overall result
    local overall_result=$((unit_result > integration_result ? unit_result : integration_result))

    # Results
    local end_time=$(date +%s)
    local total_duration=$((end_time - start_time))

    print_header "Test Results Summary"

    if [ "$INTEGRATION_ONLY" != "true" ]; then
        if [ $unit_result -eq 0 ]; then
            print_success "Unit tests: PASSED"
        else
            print_error "Unit tests: FAILED (exit code: $unit_result)"
        fi
    fi

    if [ "$UNIT_ONLY" != "true" ]; then
        if [ $integration_result -eq 0 ]; then
            print_success "Integration tests: PASSED"
        else
            print_error "Integration tests: FAILED (exit code: $integration_result)"
        fi
    fi

    echo ""
    echo -e "${BLUE}Total duration: ${total_duration}s${NC}"
    echo -e "${YELLOW}Reports generated in: $REPORTS_DIR${NC}"

    # List newly generated reports
    if [ ${#newly_generated_reports[@]} -gt 0 ]; then
        echo ""
        echo -e "${YELLOW}Generated reports:${NC}"
        for report in "${newly_generated_reports[@]}"; do
            echo "  - $report"
        done
    fi

    if [ $overall_result -ne 0 ]; then
        echo ""
        print_error "Some tests failed. Check the reports for details."
        exit $overall_result
    else
        echo ""
        print_success "All tests passed!"
    fi
}

# Run main function
main "$@"
