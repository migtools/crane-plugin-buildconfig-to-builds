package framework

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sigsyaml "sigs.k8s.io/yaml"
)

// CompareWithGoldenFile compares a Build against an expected golden YAML file.
// Variables in the golden file are expanded before comparison.
func CompareWithGoldenFile(buildObj *unstructured.Unstructured, goldenPath string, vars map[string]string) ([]string, error) {
	// Read golden file
	goldenData, err := os.ReadFile(goldenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read golden file: %w", err)
	}

	// Expand variables in golden file
	expandedGolden := expandVars(string(goldenData), vars)

	// Parse golden YAML
	var expectedMap map[string]interface{}
	if err := sigsyaml.Unmarshal([]byte(expandedGolden), &expectedMap); err != nil {
		return nil, fmt.Errorf("failed to parse golden YAML: %w", err)
	}

	// Get actual Build as map
	actualMap := buildObj.Object

	// Compare and collect differences
	var diffs []string
	compareMaps("", expectedMap, actualMap, &diffs)

	return diffs, nil
}

// expandVars expands ${VAR} references in a string using the provided vars map.
func expandVars(s string, vars map[string]string) string {
	result := s
	for key, value := range vars {
		result = strings.ReplaceAll(result, fmt.Sprintf("${%s}", key), value)
	}
	return result
}

// compareMaps recursively compares two maps and collects differences.
func compareMaps(path string, expected, actual map[string]interface{}, diffs *[]string) {
	// Check for missing keys in actual
	for key := range expected {
		currentPath := key
		if path != "" {
			currentPath = path + "." + key
		}

		expectedVal, _ := expected[key]
		actualVal, exists := actual[key]

		if !exists {
			*diffs = append(*diffs, fmt.Sprintf("missing field: %s", currentPath))
			continue
		}

		// Compare values recursively
		compareValues(currentPath, expectedVal, actualVal, diffs)
	}
}

// compareValues compares two values recursively.
func compareValues(path string, expected, actual interface{}, diffs *[]string) {
	switch expectedVal := expected.(type) {
	case map[string]interface{}:
		actualMap, ok := actual.(map[string]interface{})
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: type mismatch (expected map, got %T)", path, actual))
			return
		}
		compareMaps(path, expectedVal, actualMap, diffs)

	case []interface{}:
		actualSlice, ok := actual.([]interface{})
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: type mismatch (expected array, got %T)", path, actual))
			return
		}
		if len(expectedVal) != len(actualSlice) {
			*diffs = append(*diffs, fmt.Sprintf("%s: length mismatch (expected %d, got %d)", path, len(expectedVal), len(actualSlice)))
			return
		}
		for i := range expectedVal {
			compareValues(fmt.Sprintf("%s[%d]", path, i), expectedVal[i], actualSlice[i], diffs)
		}

	default:
		// Compare scalar values
		if fmt.Sprintf("%v", expected) != fmt.Sprintf("%v", actual) {
			*diffs = append(*diffs, fmt.Sprintf("%s: expected '%v', got '%v'", path, expected, actual))
		}
	}
}

// BuildToYAML converts a Build unstructured object to formatted YAML string.
func BuildToYAML(buildObj *unstructured.Unstructured) (string, error) {
	yamlBytes, err := sigsyaml.Marshal(buildObj.Object)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Build to YAML: %w", err)
	}
	return string(yamlBytes), nil
}

// NormalizeYAML normalizes YAML for comparison (removes formatting differences).
func NormalizeYAML(yamlStr string) (string, error) {
	var data interface{}
	if err := sigsyaml.Unmarshal([]byte(yamlStr), &data); err != nil {
		return "", err
	}

	normalized, err := sigsyaml.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(normalized), nil
}

// DiffYAML compares two YAML strings and returns a diff.
func DiffYAML(expected, actual string) (string, error) {
	// Normalize both YAMLs
	normExpected, err := NormalizeYAML(expected)
	if err != nil {
		return "", fmt.Errorf("failed to normalize expected YAML: %w", err)
	}

	normActual, err := NormalizeYAML(actual)
	if err != nil {
		return "", fmt.Errorf("failed to normalize actual YAML: %w", err)
	}

	if normExpected == normActual {
		return "", nil // No diff
	}

	// Simple line-by-line diff
	expectedLines := strings.Split(normExpected, "\n")
	actualLines := strings.Split(normActual, "\n")

	var diff bytes.Buffer
	maxLines := len(expectedLines)
	if len(actualLines) > maxLines {
		maxLines = len(actualLines)
	}

	for i := 0; i < maxLines; i++ {
		var expLine, actLine string
		if i < len(expectedLines) {
			expLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actLine = actualLines[i]
		}

		if expLine != actLine {
			if expLine != "" {
				diff.WriteString(fmt.Sprintf("- %s\n", expLine))
			}
			if actLine != "" {
				diff.WriteString(fmt.Sprintf("+ %s\n", actLine))
			}
		}
	}

	return diff.String(), nil
}
