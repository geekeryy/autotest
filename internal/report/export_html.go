package report

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/google/uuid"
)

//go:embed templates/scenario_run_report.html
var scenarioReportHTML embed.FS

type htmlExportData struct {
	Report    *ScenarioRunReport
	Generated string
}

// ExportScenarioRunHTML renders a standalone HTML report file.
func ExportScenarioRunHTML(report *ScenarioRunReport) ([]byte, error) {
	tmpl, err := template.New("scenario_run_report.html").Funcs(template.FuncMap{
		"statusLabel": func(s string) string {
			switch RunStatus(s) {
			case RunPassed:
				return "通过"
			case RunFailed:
				return "失败"
			case RunRunning:
				return "运行中"
			default:
				switch ResultStatus(s) {
				case ResultPassed:
					return "通过"
				case ResultFailed:
					return "失败"
				case ResultError:
					return "错误"
				default:
					return s
				}
			}
		},
		"statusClass": func(s string) string {
			switch RunStatus(s) {
			case RunPassed:
				return "passed"
			case RunFailed:
				return "failed"
			default:
				switch ResultStatus(s) {
				case ResultPassed:
					return "passed"
				case ResultFailed, ResultError:
					return "failed"
				default:
					return "neutral"
				}
			}
		},
		"fmtTime": func(t *time.Time) string {
			if t == nil {
				return "-"
			}
			return t.Local().Format("2006-01-02 15:04:05")
		},
		"fmtDuration": func(ms int64) string {
			if ms < 1000 {
				return fmt.Sprintf("%d ms", ms)
			}
			return fmt.Sprintf("%.2f s", float64(ms)/1000)
		},
		"jsonPre": func(v any) string {
			if v == nil {
				return ""
			}
			switch x := v.(type) {
			case string:
				return x
			default:
				b, err := json.MarshalIndent(v, "", "  ")
				if err != nil {
					return fmt.Sprint(v)
				}
				return string(b)
			}
		},
		"stepLabel": func(step *StepMeta, stepID *uuid.UUID) string {
			if step != nil {
				name := strings.TrimSpace(step.Name)
				if name != "" {
					return fmt.Sprintf("#%d %s", step.StepSeq, name)
				}
				return fmt.Sprintf("#%d (%s)", step.StepSeq, step.StepType)
			}
			if stepID != nil {
				return stepID.String()
			}
			return "未知步骤"
		},
	}).ParseFS(scenarioReportHTML, "templates/scenario_run_report.html")
	if err != nil {
		return nil, fmt.Errorf("parse html template: %w", err)
	}
	var buf bytes.Buffer
	data := htmlExportData{
		Report:    report,
		Generated: time.Now().Local().Format("2006-01-02 15:04:05"),
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render html report: %w", err)
	}
	return buf.Bytes(), nil
}
