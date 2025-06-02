package integration

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"
)

// ReportFormat represents the output format for reports
type ReportFormat string

const (
	FormatTXT  ReportFormat = "txt"
	FormatJSON ReportFormat = "json"
	FormatHTML ReportFormat = "html"
)

// Reporter handles generating reports in different formats
type Reporter struct {
	report *TestReport
}

// NewReporter creates a new reporter
func NewReporter(report *TestReport) *Reporter {
	return &Reporter{report: report}
}

// GenerateReport generates a report in the specified format and saves it to the given directory
func (r *Reporter) GenerateReport(format ReportFormat, outputDir string) error {
	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s-report.%s", timestamp, string(format))
	fpath := filepath.Join(outputDir, filename)

	switch format {
	case FormatTXT:
		return r.generateTextReport(fpath)
	case FormatJSON:
		return r.generateJSONReport(fpath)
	case FormatHTML:
		return r.generateHTMLReport(fpath)
	default:
		return fmt.Errorf("unsupported report format: %s", format)
	}
}

// generateTextReport generates a plain text report
func (r *Reporter) generateTextReport(fpath string) error {
	file, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Header
	fmt.Fprintf(file, "PTAH MIGRATION LIBRARY INTEGRATION TEST REPORT\n")
	fmt.Fprintf(file, "===============================================\n\n")
	fmt.Fprintf(file, "Generated: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "Test Period: %s - %s\n",
		r.report.StartTime.Format("15:04:05"),
		r.report.EndTime.Format("15:04:05"))
	fmt.Fprintf(file, "Duration: %v\n\n",
		r.report.EndTime.Sub(r.report.StartTime).Round(time.Millisecond))

	// Summary
	fmt.Fprintf(file, "SUMMARY\n")
	fmt.Fprintf(file, "-------\n")
	fmt.Fprintf(file, "%s\n\n", r.report.Summary)

	// Statistics
	fmt.Fprintf(file, "STATISTICS\n")
	fmt.Fprintf(file, "----------\n")
	fmt.Fprintf(file, "Total Tests: %d\n", r.report.TotalTests)
	fmt.Fprintf(file, "Passed: %d\n", r.report.PassedTests)
	fmt.Fprintf(file, "Failed: %d\n", r.report.FailedTests)
	if r.report.TotalTests > 0 {
		successRate := float64(r.report.PassedTests) / float64(r.report.TotalTests) * 100
		fmt.Fprintf(file, "Success Rate: %.1f%%\n", successRate)
	}
	fmt.Fprintf(file, "\n")

	// Detailed Results
	fmt.Fprintf(file, "DETAILED RESULTS\n")
	fmt.Fprintf(file, "----------------\n")

	for _, result := range r.report.Results {
		status := "✅ PASS"
		if !result.Success {
			status = "❌ FAIL"
		}

		fmt.Fprintf(file, "%s %s (%s) - %v\n",
			status, result.Name, result.Database, result.Duration.Round(time.Millisecond))
		fmt.Fprintf(file, "    Description: %s\n", result.Description)

		if !result.Success && result.Error != "" {
			fmt.Fprintf(file, "    Error: %s\n", result.Error)
		}
		fmt.Fprintf(file, "\n")
	}

	// Failed Tests Summary
	if r.report.FailedTests > 0 {
		fmt.Fprintf(file, "FAILED TESTS SUMMARY\n")
		fmt.Fprintf(file, "--------------------\n")
		for _, result := range r.report.Results {
			if !result.Success {
				fmt.Fprintf(file, "❌ %s (%s)\n", result.Name, result.Database)
				fmt.Fprintf(file, "   Error: %s\n\n", result.Error)
			}
		}
	}

	return nil
}

// generateJSONReport generates a JSON report
func (r *Reporter) generateJSONReport(fpath string) error {
	file, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(r.report)
}

// generateHTMLReport generates an HTML report
func (r *Reporter) generateHTMLReport(fpath string) error {
	file, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer file.Close()

	tmpl := template.Must(template.New("report").Funcs(template.FuncMap{
		"formatDuration": func(d time.Duration) string {
			return d.Round(time.Millisecond).String()
		},
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05")
		},
		"successRate": func() float64 {
			if r.report.TotalTests == 0 {
				return 0
			}
			return float64(r.report.PassedTests) / float64(r.report.TotalTests) * 100
		},
		"statusIcon": func(success bool) string {
			if success {
				return "✅"
			}
			return "❌"
		},
		"statusClass": func(success bool) string {
			if success {
				return "success"
			}
			return "failure"
		},
	}).Parse(htmlTemplate))

	return tmpl.Execute(file, r.report)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Ptah Migration Library Integration Test Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background-color: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 2px solid #007acc; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .summary { background: #e8f4fd; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 20px 0; }
        .stat-card { background: #f8f9fa; padding: 15px; border-radius: 5px; text-align: center; border-left: 4px solid #007acc; }
        .stat-value { font-size: 2em; font-weight: bold; color: #007acc; }
        .stat-label { color: #666; margin-top: 5px; }
        table { width: 100%; border-collapse: collapse; margin: 20px 0; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        th { background-color: #f8f9fa; font-weight: bold; }
        .success { color: #28a745; }
        .failure { color: #dc3545; }
        .error-details { background: #f8d7da; padding: 10px; border-radius: 3px; margin-top: 5px; font-family: monospace; font-size: 0.9em; }
        .duration { color: #666; font-size: 0.9em; }
        .database-badge { background: #007acc; color: white; padding: 2px 8px; border-radius: 12px; font-size: 0.8em; }
        .progress-bar { width: 100%; height: 20px; background: #e9ecef; border-radius: 10px; overflow: hidden; }
        .progress-fill { height: 100%; background: linear-gradient(90deg, #28a745, #20c997); transition: width 0.3s ease; }
        .steps-container { margin-top: 10px; }
        .step-item { margin-left: 20px; padding: 5px 0; border-left: 2px solid #e9ecef; padding-left: 10px; }
        .step-header { display: flex; align-items: center; gap: 8px; }
        .step-name { font-weight: bold; }
        .step-duration { color: #666; font-size: 0.9em; }
        .step-description { color: #666; font-size: 0.9em; margin-top: 2px; }
        .step-error { background: #f8d7da; padding: 5px; border-radius: 3px; margin-top: 5px; font-family: monospace; font-size: 0.8em; }
        .expandable { cursor: pointer; user-select: none; }
        .expandable:hover { background-color: #f8f9fa; }
        .expand-icon { transition: transform 0.2s; }
        .expanded .expand-icon { transform: rotate(90deg); }
    </style>
</head>
<body>
    <div class="container">
        <h1>🏛️ Ptah Migration Library Integration Test Report</h1>
        
        <div class="summary">
            <h2>📊 Summary</h2>
            <p><strong>{{.Summary}}</strong></p>
            <p><strong>Test Period:</strong> {{formatTime .StartTime}} - {{formatTime .EndTime}}</p>
            <p><strong>Total Duration:</strong> {{formatDuration (.EndTime.Sub .StartTime)}}</p>
        </div>

        <div class="stats">
            <div class="stat-card">
                <div class="stat-value">{{.TotalTests}}</div>
                <div class="stat-label">Total Tests</div>
            </div>
            <div class="stat-card">
                <div class="stat-value success">{{.PassedTests}}</div>
                <div class="stat-label">Passed</div>
            </div>
            <div class="stat-card">
                <div class="stat-value failure">{{.FailedTests}}</div>
                <div class="stat-label">Failed</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">{{printf "%.1f%%" successRate}}</div>
                <div class="stat-label">Success Rate</div>
            </div>
        </div>

        <div class="progress-bar">
            <div class="progress-fill" style="width: {{successRate}}%"></div>
        </div>

        <h2>📋 Detailed Results</h2>
        <table>
            <thead>
                <tr>
                    <th>Status</th>
                    <th>Test Name</th>
                    <th>Database</th>
                    <th>Duration</th>
                    <th>Description</th>
                </tr>
            </thead>
            <tbody>
                {{range .Results}}
                <tr class="{{if .Steps}}expandable{{end}}" onclick="{{if .Steps}}toggleSteps('{{.Name}}_{{.Database}}'){{end}}">
                    <td class="{{statusClass .Success}}">
                        {{if .Steps}}<span class="expand-icon">▶</span>{{end}}
                        {{statusIcon .Success}}
                    </td>
                    <td>{{.Name}}</td>
                    <td><span class="database-badge">{{.Database}}</span></td>
                    <td class="duration">{{formatDuration .Duration}}</td>
                    <td>
                        {{.Description}}
                        {{if not .Success}}
                            <div class="error-details">{{.Error}}</div>
                        {{end}}
                    </td>
                </tr>
                {{if .Steps}}
                <tr id="steps_{{.Name}}_{{.Database}}" style="display: none;">
                    <td colspan="5">
                        <div class="steps-container">
                            {{range .Steps}}
                            <div class="step-item">
                                <div class="step-header">
                                    <span class="{{if .Success}}success{{else}}failure{{end}}">{{if .Success}}✅{{else}}❌{{end}}</span>
                                    <span class="step-name">{{.Name}}</span>
                                    <span class="step-duration">({{formatDuration .Duration}})</span>
                                </div>
                                <div class="step-description">{{.Description}}</div>
                                {{if not .Success}}
                                    <div class="step-error">{{.Error}}</div>
                                {{end}}
                            </div>
                            {{end}}
                        </div>
                    </td>
                </tr>
                {{end}}
                {{end}}
            </tbody>
        </table>

        <footer style="margin-top: 40px; padding-top: 20px; border-top: 1px solid #ddd; color: #666; text-align: center;">
            <p>Generated by Ptah Migration Library Integration Test Suite</p>
            <p>Report generated at {{formatTime .EndTime}}</p>
        </footer>
    </div>

    <script>
        function toggleSteps(testId) {
            const stepsRow = document.getElementById('steps_' + testId);
            const expandIcon = event.currentTarget.querySelector('.expand-icon');

            if (stepsRow.style.display === 'none') {
                stepsRow.style.display = 'table-row';
                expandIcon.style.transform = 'rotate(90deg)';
                event.currentTarget.classList.add('expanded');
            } else {
                stepsRow.style.display = 'none';
                expandIcon.style.transform = 'rotate(0deg)';
                event.currentTarget.classList.remove('expanded');
            }
        }
    </script>
</body>
</html>`
