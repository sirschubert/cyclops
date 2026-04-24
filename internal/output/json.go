package output

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirschubert/cyclops/pkg/models"
)

// JSONFormatter formats scan results as JSON
type JSONFormatter struct{}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter() *JSONFormatter {
	return &JSONFormatter{}
}

// Format converts the scan result to JSON
func (jf *JSONFormatter) Format(result models.Result) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

// WriteToFile writes the JSON result to a file
func (jf *JSONFormatter) WriteToFile(result models.Result, filename string) error {
	data, err := jf.Format(result)
	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// WriteToStdout writes the JSON result to stdout
func (jf *JSONFormatter) WriteToStdout(result models.Result) error {
	data, err := jf.Format(result)
	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	fmt.Println(string(data))
	return nil
}