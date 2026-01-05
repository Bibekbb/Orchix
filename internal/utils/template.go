package utils

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

// TemplateEngine handles template rendering
type TemplateEngine struct {
	funcs      template.FuncMap
	delimiters []string
}

// TemplateData holds data for template rendering
type TemplateData struct {
	Variables map[string]interface{}
	Secrets   map[string]string
	Component map[string]interface{}
	Env       map[string]string
	System    map[string]interface{}
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine() *TemplateEngine {
	engine := &TemplateEngine{
		funcs: make(template.FuncMap),
		delimiters: []string{"{{", "}}"},
	}

	// Register built-in functions
	engine.registerBuiltinFuncs()

	return engine
}

// SetDelimiters sets custom template delimiters
func (e *TemplateEngine) SetDelimiters(left, right string) {
	e.delimiters = []string{left, right}
}

// RegisterFunc registers a custom template function
func (e *TemplateEngine) RegisterFunc(name string, fn interface{}) {
	e.funcs[name] = fn
}

// Render renders a template with the given data
func (e *TemplateEngine) Render(templateStr string, data TemplateData) (string, error) {
	// Create template
	tmpl, err := template.New("template").
		Delims(e.delimiters[0], e.delimiters[1]).
		Funcs(e.funcs).
		Parse(templateStr)
	
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// RenderFile renders a template file
func (e *TemplateEngine) RenderFile(filePath string, data TemplateData) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file: %w", err)
	}

	return e.Render(string(content), data)
}

// RenderYAML renders a YAML template
func (e *TemplateEngine) RenderYAML(yamlContent string, data TemplateData) (string, error) {
	rendered, err := e.Render(yamlContent, data)
	if err != nil {
		return "", err
	}

	// Validate that the rendered content is valid YAML
	var dummy interface{}
	if err := yaml.Unmarshal([]byte(rendered), &dummy); err != nil {
		return "", fmt.Errorf("rendered content is not valid YAML: %w", err)
	}

	return rendered, nil
}

// RenderYAMLFile renders a YAML file with templates
func (e *TemplateEngine) RenderYAMLFile(filePath string, data TemplateData) (map[string]interface{}, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	rendered, err := e.RenderYAML(string(content), data)
	if err != nil {
		return nil, err
	}

	// Parse rendered YAML
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(rendered), &result); err != nil {
		return nil, fmt.Errorf("failed to parse rendered YAML: %w", err)
	}

	return result, nil
}

// ProcessVariables processes template variables in a map
func (e *TemplateEngine) ProcessVariables(vars map[string]interface{}, data TemplateData) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range vars {
		switch v := value.(type) {
		case string:
			// Render string templates
			rendered, err := e.Render(v, data)
			if err != nil {
				return nil, fmt.Errorf("failed to render variable %s: %w", key, err)
			}
			result[key] = rendered
		case map[string]interface{}:
			// Recursively process nested maps
			nestedData := data
			nestedData.Variables = v
			processed, err := e.ProcessVariables(v, nestedData)
			if err != nil {
				return nil, err
			}
			result[key] = processed
		case []interface{}:
			// Process arrays
			processedArray, err := e.processArray(v, data)
			if err != nil {
				return nil, err
			}
			result[key] = processedArray
		default:
			// Keep non-string values as-is
			result[key] = value
		}
	}

	return result, nil
}

// Helper method to process arrays
func (e *TemplateEngine) processArray(arr []interface{}, data TemplateData) ([]interface{}, error) {
	result := make([]interface{}, len(arr))

	for i, item := range arr {
		switch v := item.(type) {
		case string:
			rendered, err := e.Render(v, data)
			if err != nil {
				return nil, err
			}
			result[i] = rendered
		case map[string]interface{}:
			nestedData := data
			nestedData.Variables = v
			processed, err := e.ProcessVariables(v, nestedData)
			if err != nil {
				return nil, err
			}
			result[i] = processed
		case []interface{}:
			processed, err := e.processArray(v, data)
			if err != nil {
				return nil, err
			}
			result[i] = processed
		default:
			result[i] = item
		}
	}

	return result, nil
}

// Register built-in template functions
func (e *TemplateEngine) registerBuiltinFuncs() {
	e.funcs = template.FuncMap{
		// String functions
		"upper":      strings.ToUpper,
		"lower":      strings.ToLower,
		"title":      strings.Title,
		"trim":       strings.TrimSpace,
		"replace":    strings.Replace,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"split":      strings.Split,
		"join":       strings.Join,
		"trimPrefix": strings.TrimPrefix,
		"trimSuffix": strings.TrimSuffix,

		// Path functions
		"base":     filepath.Base,
		"dir":      filepath.Dir,
		"ext":      filepath.Ext,
		"clean":    filepath.Clean,
		"joinPath": filepath.Join,

		// Math functions
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mod": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a % b
		},

		// Comparison functions
		"eq":  func(a, b interface{}) bool { return a == b },
		"ne":  func(a, b interface{}) bool { return a != b },
		"gt":  func(a, b int) bool { return a > b },
		"lt":  func(a, b int) bool { return a < b },
		"gte": func(a, b int) bool { return a >= b },
		"lte": func(a, b int) bool { return a <= b },

		// Logical functions
		"and": func(a, b bool) bool { return a && b },
		"or":  func(a, b bool) bool { return a || b },
		"not": func(a bool) bool { return !a },

		// Array functions
		"len":   func(arr []interface{}) int { return len(arr) },
		"first": func(arr []interface{}) interface{} {
			if len(arr) > 0 {
				return arr[0]
			}
			return nil
		},
		"last": func(arr []interface{}) interface{} {
			if len(arr) > 0 {
				return arr[len(arr)-1]
			}
			return nil
		},
		"slice": func(arr []interface{}, start, end int) []interface{} {
			if start < 0 {
				start = 0
			}
			if end > len(arr) {
				end = len(arr)
			}
			if start >= end {
				return []interface{}{}
			}
			return arr[start:end]
		},

		// Map functions
		"keys": func(m map[string]interface{}) []string {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			return keys
		},
		"values": func(m map[string]interface{}) []interface{} {
			values := make([]interface{}, 0, len(m))
			for _, v := range m {
				values = append(values, v)
			}
			return values
		},
		"hasKey": func(m map[string]interface{}, key string) bool {
			_, ok := m[key]
			return ok
		},
		"get": func(m map[string]interface{}, key string) interface{} {
			return m[key]
		},

		// Type conversion
		"toString": func(v interface{}) string {
			return fmt.Sprintf("%v", v)
		},
		"toInt": func(v interface{}) int {
			switch val := v.(type) {
			case int:
				return val
			case float64:
				return int(val)
			case string:
				var i int
				fmt.Sscanf(val, "%d", &i)
				return i
			default:
				return 0
			}
		},
		"toFloat": func(v interface{}) float64 {
			switch val := v.(type) {
			case float64:
				return val
			case int:
				return float64(val)
			case string:
				var f float64
				fmt.Sscanf(val, "%f", &f)
				return f
			default:
				return 0.0
			}
		},

		// Environment functions
		"env": func(key string) string {
			return os.Getenv(key)
		},
		"envOrDefault": func(key, defaultValue string) string {
			if value := os.Getenv(key); value != "" {
				return value
			}
			return defaultValue
		},

		// Secret functions
		"secret": func(name string) string {
			// This would be replaced with actual secret lookup
			return fmt.Sprintf("{{secret:%s}}", name)
		},

		// Formatting functions
		"indent": func(spaces int, text string) string {
			prefix := strings.Repeat(" ", spaces)
			lines := strings.Split(text, "\n")
			for i, line := range lines {
				lines[i] = prefix + line
			}
			return strings.Join(lines, "\n")
		},
		"nindent": func(spaces int, text string) string {
			return "\n" + e.funcs["indent"].(func(int, string) string)(spaces, text)
		},
		"quote": func(text string) string {
			return "\"" + strings.ReplaceAll(text, "\"", "\\\"") + "\""
		},
		"squote": func(text string) string {
			return "'" + strings.ReplaceAll(text, "'", "\\'") + "'"
		},

		// Utility functions
		"default": func(defaultValue, value interface{}) interface{} {
			if value == nil || value == "" {
				return defaultValue
			}
			return value
		},
		"required": func(msg string, value interface{}) (interface{}, error) {
			if value == nil || value == "" {
				return nil, fmt.Errorf("required value missing: %s", msg)
			}
			return value, nil
		},
		"fail": func(msg string) (string, error) {
			return "", fmt.Errorf(msg)
		},

		// JSON/YAML functions
		"toYaml": func(v interface{}) (string, error) {
			data, err := yaml.Marshal(v)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
		"fromYaml": func(yamlStr string) (map[string]interface{}, error) {
			var result map[string]interface{}
			if err := yaml.Unmarshal([]byte(yamlStr), &result); err != nil {
				return nil, err
			}
			return result, nil
		},

		// Conditional functions
		"ternary": func(condition bool, trueVal, falseVal interface{}) interface{} {
			if condition {
				return trueVal
			}
			return falseVal
		},
		"coalesce": func(values ...interface{}) interface{} {
			for _, v := range values {
				if v != nil && v != "" {
					return v
				}
			}
			return nil
		},
	}
}

// Global template engine instance
var (
	globalTemplateEngine *TemplateEngine
	engineOnce           sync.Once
)

// GetTemplateEngine returns the global template engine
func GetTemplateEngine() *TemplateEngine {
	engineOnce.Do(func() {
		globalTemplateEngine = NewTemplateEngine()
	})
	return globalTemplateEngine
}

// RenderString renders a template string using the global engine
func RenderString(templateStr string, data TemplateData) (string, error) {
	return GetTemplateEngine().Render(templateStr, data)
}

// RenderFile renders a template file using the global engine
func RenderFile(filePath string, data TemplateData) (string, error) {
	return GetTemplateEngine().RenderFile(filePath, data)
}

// ProcessMapVariables processes variables in a map using the global engine
func ProcessMapVariables(vars map[string]interface{}, data TemplateData) (map[string]interface{}, error) {
	return GetTemplateEngine().ProcessVariables(vars, data)
}

// SimpleTemplateData creates simple template data
func SimpleTemplateData(variables map[string]interface{}) TemplateData {
	return TemplateData{
		Variables: variables,
		Secrets:   make(map[string]string),
		Component: make(map[string]interface{}),
		Env:       make(map[string]string),
		System: map[string]interface{}{
			"arch":    runtime.GOARCH,
			"os":      runtime.GOOS,
			"cpus":    runtime.NumCPU(),
			"tempDir": os.TempDir(),
		},
	}
}