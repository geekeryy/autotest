package builtin

import "testing"

// TestCatalogToolsIncludesConditionalTools guards the embedding/catalog
// enumeration path: tools registered behind a non-nil dependency check must
// still appear in CatalogTools so find_tools vector search can surface them.
// Regression test for the embeddings generator previously using empty Deps,
// which silently dropped the natural-language scenario tools.
func TestCatalogToolsIncludesConditionalTools(t *testing.T) {
	present := map[string]bool{}
	for _, tl := range CatalogTools() {
		present[tl.Name] = true
	}

	for _, name := range []string{
		"plan_scenario_from_prompt",
		"append_step_from_description",
		"describe_scenario_in_natural_language",
		"generate_and_verify_scenarios",
	} {
		if !present[name] {
			t.Errorf("CatalogTools() is missing conditionally-registered tool %q", name)
		}
	}
}

// TestCatalogToolsIsSupersetOfEmptyDeps ensures CatalogTools never drops a
// tool that the empty-deps domain set already includes (find_tools /
// describe_tools / ask_question are intentionally not part of the domain
// catalog and are excluded from this comparison).
func TestCatalogToolsIsSupersetOfEmptyDeps(t *testing.T) {
	catalog := map[string]bool{}
	for _, tl := range CatalogTools() {
		catalog[tl.Name] = true
	}

	domain := append(ReadOnly(Deps{}), Mutating(Deps{})...)
	for _, tl := range domain {
		if !catalog[tl.Name] {
			t.Errorf("CatalogTools() dropped domain tool %q present with empty Deps", tl.Name)
		}
	}
}
