package report

import "encoding/json"

// ExportJSON returns the report as indented JSON bytes.
func ExportJSON(report *ScenarioRunReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
