package main

import (
	"fmt"
	"os"
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
		DemonstrateASTApproach()
	case "compare":
		CompareApproaches()
	case "advanced":
		ShowAdvancedFeatures()
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
