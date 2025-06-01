package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/ptah/integration"
)

var (
	reportFormat string
	outputDir    string
	databases    []string
	scenarios    []string
	verbose      bool

	// List command flags
	showStatic  bool
	showDynamic bool
	showAll     bool
)

var rootCmd = &cobra.Command{
	Use:   "ptah-integration-test",
	Short: "Run Ptah migration library integration tests",
	Long: `Run comprehensive integration tests for the Ptah migration library.

This tool tests migration functionality across multiple database backends
including PostgreSQL, MySQL, and MariaDB. It validates basic functionality,
idempotency, concurrency, failure recovery, and more.

The tests use Docker containers for database backends and generate detailed
reports in multiple formats.`,
	RunE: runIntegrationTests,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available test scenarios",
	Long: `List all available integration test scenarios with their descriptions.

This command displays all static and dynamic test scenarios that can be run
with the integration test suite. Use this to see what scenarios are available
before running specific tests with the --scenarios flag.`,
	RunE: listScenarios,
}

func init() {
	// Root command flags
	rootCmd.Flags().StringVar(&reportFormat, "report", "txt", "Report format: txt, json, or html")
	rootCmd.Flags().StringVar(&outputDir, "output", "/app/reports", "Output directory for reports")
	rootCmd.Flags().StringSliceVar(&databases, "databases", []string{"postgres", "mysql", "mariadb"}, "Databases to test against")
	rootCmd.Flags().StringSliceVar(&scenarios, "scenarios", []string{}, "Specific scenarios to run (empty = all)")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	// List command flags
	listCmd.Flags().BoolVar(&showStatic, "static", false, "Show only static scenarios")
	listCmd.Flags().BoolVar(&showDynamic, "dynamic", false, "Show only dynamic scenarios")
	listCmd.Flags().BoolVar(&showAll, "all", true, "Show all scenarios (default)")

	// Add subcommands
	rootCmd.AddCommand(listCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runIntegrationTests(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Validate report format
	format := integration.ReportFormat(reportFormat)
	switch format {
	case integration.FormatTXT, integration.FormatJSON, integration.FormatHTML:
		// Valid formats
	default:
		return fmt.Errorf("invalid report format: %s (must be txt, json, or html)", reportFormat)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Load test fixtures
	// Try Docker path first, then local development path
	fixturesPath := "/app/fixtures"
	if _, err := os.Stat(fixturesPath); os.IsNotExist(err) {
		// Fallback to local development path
		fixturesPath = "integration/fixtures"
	}
	fixturesFS := os.DirFS(fixturesPath)

	// Create test runner
	runner := integration.NewTestRunner(fixturesFS)

	// Add database connections from environment variables
	dbConnections := map[string]string{
		"postgres": os.Getenv("POSTGRES_URL"),
		"mysql":    os.Getenv("MYSQL_URL"),
		"mariadb":  os.Getenv("MARIADB_URL"),
	}

	// Filter databases based on command line arguments
	for _, dbName := range databases {
		if url, exists := dbConnections[dbName]; exists && url != "" {
			runner.AddDatabase(dbName, url)
			if verbose {
				fmt.Printf("Added database: %s\n", dbName)
			}
		} else {
			fmt.Printf("Warning: Database %s not available (missing URL)\n", dbName)
		}
	}

	// Get all scenarios
	allScenarios := integration.GetAllScenarios()

	// Filter scenarios if specific ones were requested
	var scenariosToRun []integration.TestScenario
	if len(scenarios) > 0 {
		scenarioMap := make(map[string]integration.TestScenario)
		for _, scenario := range allScenarios {
			scenarioMap[scenario.Name] = scenario
		}

		for _, scenarioName := range scenarios {
			if scenario, exists := scenarioMap[scenarioName]; exists {
				scenariosToRun = append(scenariosToRun, scenario)
			} else {
				return fmt.Errorf("unknown scenario: %s", scenarioName)
			}
		}
	} else {
		scenariosToRun = allScenarios
	}

	// Add scenarios to runner
	for _, scenario := range scenariosToRun {
		runner.AddScenario(scenario)
		if verbose {
			fmt.Printf("Added scenario: %s\n", scenario.Name)
		}
	}

	fmt.Printf("🏛️  Ptah Migration Library Integration Test Suite\n")
	fmt.Printf("================================================\n\n")
	fmt.Printf("Databases: %s\n", strings.Join(databases, ", "))
	fmt.Printf("Scenarios: %d\n", len(scenariosToRun))
	fmt.Printf("Report Format: %s\n", reportFormat)
	fmt.Printf("Output Directory: %s\n\n", outputDir)

	// Run all tests
	fmt.Printf("🚀 Starting integration tests...\n\n")
	start := time.Now()

	if err := runner.RunAll(ctx); err != nil {
		return fmt.Errorf("failed to run integration tests: %w", err)
	}

	duration := time.Since(start)
	fmt.Printf("✅ Integration tests completed in %v\n\n", duration.Round(time.Millisecond))

	// Generate report
	report := runner.GetReport()
	reporter := integration.NewReporter(report)

	if err := reporter.GenerateReport(format, outputDir); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Print summary
	fmt.Printf("📊 Test Summary:\n")
	fmt.Printf("   Total Tests: %d\n", report.TotalTests)
	fmt.Printf("   Passed: %d\n", report.PassedTests)
	fmt.Printf("   Failed: %d\n", report.FailedTests)

	if report.TotalTests > 0 {
		successRate := float64(report.PassedTests) / float64(report.TotalTests) * 100
		fmt.Printf("   Success Rate: %.1f%%\n", successRate)
	}

	fmt.Printf("\n📄 Report saved to: %s\n", outputDir)

	// Exit with error code if any tests failed
	if report.FailedTests > 0 {
		fmt.Printf("\n❌ Some tests failed. Check the report for details.\n")
		os.Exit(1)
	}

	fmt.Printf("\n🎉 All tests passed!\n")
	return nil
}

func listScenarios(cmd *cobra.Command, args []string) error {
	// Get all scenarios
	allScenarios := integration.GetAllScenarios()
	staticScenarios := getStaticScenarios()
	dynamicScenarios := integration.GetDynamicScenarios()

	// Determine which scenarios to show based on flags
	var scenariosToShow []integration.TestScenario
	var title string

	// Handle flag combinations
	if showStatic && showDynamic {
		// Both flags set - show all
		scenariosToShow = allScenarios
		title = "All Test Scenarios"
	} else if showStatic {
		// Only static
		scenariosToShow = staticScenarios
		title = "Static Test Scenarios"
	} else if showDynamic {
		// Only dynamic
		scenariosToShow = dynamicScenarios
		title = "Dynamic Test Scenarios"
	} else {
		// Default - show all
		scenariosToShow = allScenarios
		title = "All Test Scenarios"
	}

	// Print header
	fmt.Printf("🏛️  Ptah Migration Library - %s\n", title)
	fmt.Printf("%s\n\n", strings.Repeat("=", len(title)+35))

	// Group scenarios by type for better organization
	if !showStatic && !showDynamic {
		// Show both types with grouping
		fmt.Printf("📋 Static Scenarios (%d):\n", len(staticScenarios))
		printScenarios(staticScenarios, "  ")

		fmt.Printf("\n🔄 Dynamic Scenarios (%d):\n", len(dynamicScenarios))
		printScenarios(dynamicScenarios, "  ")

		fmt.Printf("\n📊 Summary:\n")
		fmt.Printf("  Total Scenarios: %d\n", len(allScenarios))
		fmt.Printf("  Static: %d\n", len(staticScenarios))
		fmt.Printf("  Dynamic: %d\n", len(dynamicScenarios))
	} else {
		// Show filtered scenarios
		printScenarios(scenariosToShow, "")
		fmt.Printf("\n📊 Total: %d scenarios\n", len(scenariosToShow))
	}

	fmt.Printf("\n💡 Usage:\n")
	fmt.Printf("  Run all scenarios:     ptah-integration-test\n")
	fmt.Printf("  Run specific scenario: ptah-integration-test --scenarios scenario_name\n")
	fmt.Printf("  Run multiple:          ptah-integration-test --scenarios scenario1,scenario2\n")

	return nil
}

// getStaticScenarios returns only the static scenarios (non-dynamic ones)
func getStaticScenarios() []integration.TestScenario {
	allScenarios := integration.GetAllScenarios()
	dynamicScenarios := integration.GetDynamicScenarios()

	// Create a map of dynamic scenario names for quick lookup
	dynamicNames := make(map[string]bool)
	for _, scenario := range dynamicScenarios {
		dynamicNames[scenario.Name] = true
	}

	// Filter out dynamic scenarios
	var staticScenarios []integration.TestScenario
	for _, scenario := range allScenarios {
		if !dynamicNames[scenario.Name] {
			staticScenarios = append(staticScenarios, scenario)
		}
	}

	return staticScenarios
}

// printScenarios prints a list of scenarios with formatting
func printScenarios(scenarios []integration.TestScenario, indent string) {
	for i, scenario := range scenarios {
		// Determine scenario type indicator
		typeIndicator := "📋"
		if strings.HasPrefix(scenario.Name, "dynamic_") {
			typeIndicator = "🔄"
		}

		// Determine if it has enhanced functionality (step recording)
		enhancedIndicator := ""
		if scenario.EnhancedTestFunc != nil {
			enhancedIndicator = " ✨"
		}

		fmt.Printf("%s%s %s%s\n", indent, typeIndicator, scenario.Name, enhancedIndicator)
		fmt.Printf("%s   %s\n", indent, scenario.Description)

		// Add spacing between scenarios except for the last one
		if i < len(scenarios)-1 {
			fmt.Printf("\n")
		}
	}
}
