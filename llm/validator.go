// llm/validator.go
package llm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func ValidateSQL(query string) error {
	q := strings.ToLower(strings.TrimSpace(query))
	fmt.Println("query:", q)

	// Disallow dangerous operations
	blocked := []string{"drop", "truncate", "alter", "delete", "create", "grant", "revoke"}

	// Check for blocked operations
	for _, keyword := range blocked {
		if strings.Contains(q, keyword) {
			return errors.New("query contains forbidden operation: " + keyword)
		}
	}

	// Ensure the query starts with a valid SQL operation
	validOps := []string{"select", "insert", "update"}
	isValid := false
	for _, op := range validOps {
		if strings.HasPrefix(q, op) {
			isValid = true
			break
		}
	}

	if !isValid {
		return errors.New("you are not allowed to perform this operation")
	}

	return nil
}

// ValidateMongo validates a MongoDB query JSON
func ValidateMongo(query string) error {
	// Parse the JSON
	var mongoQuery map[string]interface{}
	if err := json.Unmarshal([]byte(query), &mongoQuery); err != nil {
		return fmt.Errorf("invalid JSON format: %w", err)
	}

	// Check required fields
	operation, ok := mongoQuery["operation"]
	if !ok {
		return errors.New("missing required field: operation")
	}

	collection, ok := mongoQuery["collection"]
	if !ok {
		return errors.New("missing required field: collection")
	}

	// Validate operation type
	if opStr, ok := operation.(string); ok {
		// Allowed operations
		allowedOps := []string{"find", "insert", "update", "aggregate"}
		isValidOp := false
		for _, allowed := range allowedOps {
			if opStr == allowed {
				isValidOp = true
				break
			}
		}
		if !isValidOp {
			return fmt.Errorf("invalid operation: %s. Allowed operations: %v", opStr, allowedOps)
		}

		// Validate additional required fields based on operation
		switch opStr {
		case "find":
			if _, ok := mongoQuery["filter"]; !ok {
				return errors.New("find operation requires filter field")
			}
		case "insert":
			if _, ok := mongoQuery["document"]; !ok {
				return errors.New("insert operation requires document field")
			}
		case "update":
			if _, ok := mongoQuery["filter"]; !ok {
				return errors.New("update operation requires filter field")
			}
			if _, ok := mongoQuery["update"]; !ok {
				return errors.New("update operation requires update field")
			}
		case "aggregate":
			if _, ok := mongoQuery["pipeline"]; !ok {
				return errors.New("aggregate operation requires pipeline field")
			}
		}
	} else {
		return errors.New("operation must be a string")
	}

	// Collection name must be a string
	if _, ok := collection.(string); !ok {
		return errors.New("collection must be a string")
	}

	return nil
}
