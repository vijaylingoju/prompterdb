// llm/validator.go
package llm

import (
	"errors"
	"strings"
)

func ValidateSQL(query string) error {
	q := strings.ToLower(strings.TrimSpace(query))

	// Allow only SELECT queries
	if !strings.HasPrefix(q, "select") {
		return errors.New("only SELECT statements are allowed")
	}

	// Disallow dangerous keywords
	blocked := []string{"delete", "drop", "insert", "update", "truncate", "alter"}

	for _, keyword := range blocked {
		if strings.Contains(q, keyword) {
			return errors.New("query contains forbidden keyword: " + keyword)
		}
	}

	return nil
}
