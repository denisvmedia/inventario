package integration

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// TestGetAllScenarios verifies that dynamic scenarios are included
func TestGetAllScenarios(t *testing.T) {
	c := qt.New(t)

	scenarios := GetAllScenarios()
	
	// Should have both static and dynamic scenarios
	c.Assert(len(scenarios) > 10, qt.IsTrue, qt.Commentf("Expected more than 10 scenarios, got %d", len(scenarios)))

	// Check that dynamic scenarios are included
	scenarioNames := make(map[string]bool)
	for _, scenario := range scenarios {
		scenarioNames[scenario.Name] = true
	}

	// Verify some key dynamic scenarios are present
	dynamicScenarios := []string{
		"dynamic_basic_evolution",
		"dynamic_skip_versions", 
		"dynamic_idempotency",
		"dynamic_partial_apply",
		"dynamic_schema_diff",
		"dynamic_migration_sql_generation",
	}

	for _, scenarioName := range dynamicScenarios {
		c.Assert(scenarioNames[scenarioName], qt.IsTrue, qt.Commentf("Dynamic scenario %s should be included", scenarioName))
	}

	// Verify some original scenarios are still present
	originalScenarios := []string{
		"apply_incremental_migrations",
		"rollback_migrations",
		"upgrade_to_specific_version",
	}

	for _, scenarioName := range originalScenarios {
		c.Assert(scenarioNames[scenarioName], qt.IsTrue, qt.Commentf("Original scenario %s should still be included", scenarioName))
	}
}

// TestGetDynamicScenarios verifies the dynamic scenarios function
func TestGetDynamicScenarios(t *testing.T) {
	c := qt.New(t)

	scenarios := GetDynamicScenarios()
	
	// Should have exactly 6 dynamic scenarios
	c.Assert(len(scenarios), qt.Equals, 6)

	// Verify all scenarios have required fields
	for _, scenario := range scenarios {
		c.Assert(scenario.Name, qt.Not(qt.Equals), "", qt.Commentf("Scenario name should not be empty"))
		c.Assert(scenario.Description, qt.Not(qt.Equals), "", qt.Commentf("Scenario description should not be empty"))
		c.Assert(scenario.TestFunc, qt.IsNotNil, qt.Commentf("Scenario test function should not be nil"))
	}
}
