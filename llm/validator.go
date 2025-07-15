// llm/validator.go
package llm

import (
	"errors"
	"strings"
)

func ValidateSQL(query string) error {
	q := strings.ToLower(strings.TrimSpace(query))

	// Disallow dangerous operations
	blocked := []string{"drop", "truncate", "alter"}

	// Check for blocked operations
	for _, keyword := range blocked {
		if strings.Contains(q, keyword) {
			return errors.New("query contains forbidden operation: " + keyword)
		}
	}

	// Ensure the query starts with a valid SQL operation
	validOps := []string{"select", "insert", "update", "delete"}
	isValid := false
	for _, op := range validOps {
		if strings.HasPrefix(q, op) {
			isValid = true
			break
		}
	}

	if !isValid {
		return errors.New("query must start with SELECT, INSERT, UPDATE, or DELETE")
	}

	return nil
}
