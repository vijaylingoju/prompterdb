package templates

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// parseTemplatePath parses a template file path into its components
func (tm *TemplateManager) parseTemplatePath(path string) (templateType TemplateType, dbType, templateName string, err error) {
	if tm.root == "" {
		return "", "", "", errors.New("template root directory not set")
	}

	// Get relative path from root
	relPath, err := filepath.Rel(tm.root, path)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to get relative path from %s to %s: %w", tm.root, path, err)
	}

	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	// Split the path into components
	parts := strings.Split(relPath, "/")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("invalid template path structure, expected: <type>/[subtype/]<db_type>/<file>, got: %s", relPath)
	}

	// Determine template type based on path
	switch {
	case strings.HasPrefix(relPath, "system_prompts/mongo"):
		templateType = MongoSystemPrompt
	case strings.HasPrefix(relPath, "system_prompts"):
		templateType = SystemPrompt
	case strings.HasPrefix(relPath, "response_formats/mongo"):
		templateType = MongoResponseFormat
	case strings.HasPrefix(relPath, "response_formats"):
		templateType = ResponseFormat
	default:
		return "", "", "", fmt.Errorf("unknown template type in path: %s", relPath)
	}

	// The database type is the directory right before the filename
	dbType = parts[len(parts)-2]

	// The template name is the filename without extension
	filename := parts[len(parts)-1]
	templateName = strings.TrimSuffix(filename, filepath.Ext(filename))

	// For MongoDB templates, we need to handle the nested structure
	if templateType == MongoSystemPrompt || templateType == MongoResponseFormat {
		// The database type is "mongo" for MongoDB templates
		dbType = "mongo"
	}

	return templateType, dbType, templateName, nil
}

// processTemplateFile processes a single template file
func (tm *TemplateManager) processTemplateFile(path string, readFile func(string) ([]byte, error)) error {
	// Skip non-template files
	if filepath.Ext(path) != ".tmpl" {
		return nil
	}

	templateType, dbType, templateName, err := tm.parseTemplatePath(path)
	if err != nil {
		return fmt.Errorf("invalid template path %s: %w", path, err)
	}

	content, err := readFile(path)
	if err != nil {
		return fmt.Errorf("failed to read template file %s: %w", path, err)
	}

	// Validate content is not empty
	if len(content) == 0 {
		return fmt.Errorf("template file is empty: %s", path)
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Initialize the map for this template type if it doesn't exist
	if tm.templates[templateType] == nil {
		tm.templates[templateType] = make(map[string]string)
	}

	// Use dbType/templateName as the key
	key := filepath.Join(dbType, templateName)
	tm.templates[templateType][key] = string(content)

	return nil
}

// LoadTemplatesFromFS loads templates from an embed.FS
func (tm *TemplateManager) LoadTemplatesFromFS(efs embed.FS, root string) error {
	if root == "" {
		root = "."
	}

	tm.mu.Lock()
	tm.root = root // Store root for relative path resolution
	tm.mu.Unlock()

	// List all files in the embedded FS
	entries, err := efs.ReadDir(root)
	if err != nil {
		return fmt.Errorf("failed to read embedded directory %s: %w", root, err)
	}

	// Process each entry in the root directory
	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively process subdirectories
			subDir := filepath.Join(root, entry.Name())
			if err := tm.LoadTemplatesFromFS(efs, subDir); err != nil {
				return err
			}
			continue
		}

		// Process file
		filePath := filepath.Join(root, entry.Name())
		content, err := efs.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read embedded file %s: %w", filePath, err)
		}

		// Process the template file
		if err := tm.processTemplateFile(filePath, func(_ string) ([]byte, error) {
			return content, nil
		}); err != nil {
			return fmt.Errorf("error processing template %s: %w", filePath, err)
		}
	}

	return nil
}

// LoadTemplatesFromDir loads templates from a directory on the filesystem
func (tm *TemplateManager) LoadTemplatesFromDir(root string) error {
	if root == "" {
		return errors.New("root directory cannot be empty")
	}

	// Convert to absolute path for consistency
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", root, err)
	}

	tm.mu.Lock()
	tm.root = absRoot // Store absolute path as root
	tm.mu.Unlock()

	// Check if the root directory exists and is accessible
	fileInfo, err := os.Stat(absRoot)
	if err != nil {
		return fmt.Errorf("failed to access template directory %s: %w", absRoot, err)
	}
	if !fileInfo.IsDir() {
		return fmt.Errorf("template path is not a directory: %s", absRoot)
	}

	return filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking path %s: %w", path, err)
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process .tmpl files
		if filepath.Ext(path) != ".tmpl" {
			return nil
		}

		// Process the template file
		return tm.processTemplateFile(path, os.ReadFile)
	})
}
