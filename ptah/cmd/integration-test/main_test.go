package main

import (
	"testing"

	"github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/integration"
)

func TestGetStaticScenarios(t *testing.T) {
	c := quicktest.New(t)

	staticScenarios := getStaticScenarios()
	dynamicScenarios := integration.GetDynamicScenarios()
	allScenarios := integration.GetAllScenarios()

	// Static scenarios should not be empty
	c.Assert(len(staticScenarios) > 0, quicktest.IsTrue, quicktest.Commentf("Expected static scenarios to exist"))

	// Static + Dynamic should equal All scenarios
	c.Assert(len(staticScenarios)+len(dynamicScenarios), quicktest.Equals, len(allScenarios))

	// Verify no dynamic scenarios are in static list
	dynamicNames := make(map[string]bool)
	for _, scenario := range dynamicScenarios {
		dynamicNames[scenario.Name] = true
	}

	for _, scenario := range staticScenarios {
		c.Assert(dynamicNames[scenario.Name], quicktest.IsFalse, quicktest.Commentf("Static scenario %s should not be in dynamic list", scenario.Name))
	}

	// Verify all static scenarios have required fields
	for _, scenario := range staticScenarios {
		c.Assert(scenario.Name, quicktest.Not(quicktest.Equals), "", quicktest.Commentf("Static scenario name should not be empty"))
		c.Assert(scenario.Description, quicktest.Not(quicktest.Equals), "", quicktest.Commentf("Static scenario description should not be empty"))
		
		// Should have either TestFunc or EnhancedTestFunc
		hasTestFunc := scenario.TestFunc != nil || scenario.EnhancedTestFunc != nil
		c.Assert(hasTestFunc, quicktest.IsTrue, quicktest.Commentf("Static scenario %s should have a test function", scenario.Name))
	}
}

func TestStaticScenarioNaming(t *testing.T) {
	c := quicktest.New(t)

	staticScenarios := getStaticScenarios()

	// Verify that static scenarios don't have "dynamic_" prefix
	for _, scenario := range staticScenarios {
		c.Assert(scenario.Name[:8] != "dynamic_", quicktest.IsTrue, quicktest.Commentf("Static scenario %s should not have 'dynamic_' prefix", scenario.Name))
	}
}

func TestDynamicScenarioIdentification(t *testing.T) {
	c := quicktest.New(t)

	dynamicScenarios := integration.GetDynamicScenarios()

	// All dynamic scenarios should have "dynamic_" prefix
	for _, scenario := range dynamicScenarios {
		c.Assert(scenario.Name[:8], quicktest.Equals, "dynamic_", quicktest.Commentf("Dynamic scenario %s should have 'dynamic_' prefix", scenario.Name))
	}

	// All dynamic scenarios should have EnhancedTestFunc (based on current implementation)
	for _, scenario := range dynamicScenarios {
		c.Assert(scenario.EnhancedTestFunc, quicktest.IsNotNil, quicktest.Commentf("Dynamic scenario %s should have EnhancedTestFunc", scenario.Name))
	}
}
