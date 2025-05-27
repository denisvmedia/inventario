package main

import (
	"fmt"
	"os"

	"github.com/denisvmedia/inventario/cmd/migrator/migratorlib/examples"
)

func main() {
	fmt.Println("SQL AST Generator Demo")
	fmt.Println("=====================")
	fmt.Println()

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "demo":
		examples.DemonstrateASTApproach()
	case "compare":
		examples.CompareApproaches()
	case "advanced":
		examples.ShowAdvancedFeatures()
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: go run main.go [command]")
	fmt.Println()
	fmt.Println("Available commands:")
	fmt.Println("  demo      - Show basic AST-based SQL generation examples")
	fmt.Println("  compare   - Compare old string-based vs new AST-based approaches")
	fmt.Println("  advanced  - Show advanced AST features")
}
