package templates

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type TemplateType string

// TemplateType represents the type of template
const (
	// SystemPrompt is the template type for SQL system prompts
	SystemPrompt TemplateType = "system_prompts"
	// MongoSystemPrompt is the template type for MongoDB system prompts
	MongoSystemPrompt TemplateType = "system_prompts/mongo"
	// ResponseFormat is the template type for SQL response formats
	ResponseFormat TemplateType = "response_formats"
	// MongoResponseFormat is the template type for MongoDB response formats
	MongoResponseFormat TemplateType = "response_formats/mongo"
)

type TemplateManager struct {
	templates map[TemplateType]map[string]string
	mu        sync.RWMutex
	root      string // root directory for template loading
}

func NewTemplateManager() *TemplateManager {
	return &TemplateManager{
		templates: make(map[TemplateType]map[string]string),
	}
}

// HasTemplate checks if a template exists for the given type and name
func (tm *TemplateManager) HasTemplate(templateType TemplateType, dbType, templateName string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	dbTemplates, ok := tm.templates[templateType]
	if !ok {
		return false
	}

	_, ok = dbTemplates[filepath.Join(dbType, templateName)]
	return ok
}

// LoadTemplates loads all templates from the given directory
// Deprecated: Use LoadTemplatesFromDir instead
func (tm *TemplateManager) LoadTemplates(templateDir string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	err := filepath.Walk(templateDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(templateDir, path)
		if err != nil {
			return err
		}

		// Extract template type and name from path
		// Expected structure: <template_type>/<db_type>/<template_name>.tmpl
		parts := filepath.SplitList(relPath)
		if len(parts) < 3 {
			return nil // Skip files not in the expected directory structure
		}

		templateType := TemplateType(parts[0])
		dbType := parts[1]
		templateName := filepath.Base(path)
		templateName = templateName[:len(templateName)-len(filepath.Ext(templateName))] // Remove extension

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Initialize the map for this template type if it doesn't exist
		if tm.templates[templateType] == nil {
			tm.templates[templateType] = make(map[string]string)
		}

		// Use dbType/templateName as the key
		key := filepath.Join(dbType, templateName)
		tm.templates[templateType][key] = string(content)

		return nil
	})

	return err
}

// GetTemplate returns the template content for the given type and name
func (tm *TemplateManager) GetTemplate(templateType TemplateType, dbType, templateName string) (string, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	key := filepath.Join(dbType, templateName)
	if templates, ok := tm.templates[templateType]; ok {
		if content, exists := templates[key]; exists {
			return content, nil
		}
	}

	return "", errors.New("template not found")
}

// AddTemplate adds or updates a template in the manager
func (tm *TemplateManager) AddTemplate(templateType TemplateType, dbType, templateName, content string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.templates[templateType] == nil {
		tm.templates[templateType] = make(map[string]string)
	}

	key := filepath.Join(dbType, templateName)
	tm.templates[templateType][key] = content
}

// ExecuteTemplate executes a template with the given data
func (tm *TemplateManager) ExecuteTemplate(templateType TemplateType, dbType, templateName string, data interface{}) (string, error) {
	tmplContent, err := tm.GetTemplate(templateType, dbType, templateName)
	if err != nil {
		return "", fmt.Errorf("template %s/%s/%s not found: %w", templateType, dbType, templateName, err)
	}

	// Create a new template with helper functions
	funcMap := template.FuncMap{
		"toJson": func(v interface{}) (string, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
	}

	tmpl, err := template.New(templateName).Funcs(funcMap).Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("error parsing template: %w", err)
	}

	var result string
	buf := &strings.Builder{}
	if err := tmpl.Execute(buf, data); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	result = buf.String()
	return result, nil
}
