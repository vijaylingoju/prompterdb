// engine/router.go
package engine

import "strings"

// RoutePrompt determines which DB to use based on prompt content
func RoutePrompt(prompt string) string {
	lower := strings.ToLower(prompt)

	switch {
	case strings.Contains(lower, "student"):
		return "students_db" // maps to Postgres
	case strings.Contains(lower, "course"), strings.Contains(lower, "lms"):
		return "lms_db" // maps to Mongo
	default:
		return "unknown"
	}
}
