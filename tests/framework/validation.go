package framework

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sigsyaml "sigs.k8s.io/yaml"
)

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

// CompareBuildsWithGoldenFile compares generated resources against kind-specific golden files.
// Pattern: expected_Build.yaml, expected_ServiceAccount.yaml, expected_ConfigMap.yaml
// Missing file = expects that kind NOT generated
func CompareBuildsWithGoldenFile(resources []*unstructured.Unstructured, testDirPath string) ([]string, error) {
	var diffs []string

	// Group resources by kind
	resourcesByKind := make(map[string][]*unstructured.Unstructured)
	for _, res := range resources {
		kind := res.GetKind()
		resourcesByKind[kind] = append(resourcesByKind[kind], res)
	}

	// Check each kind's golden file
	expectedKinds := []string{"Build", "ServiceAccount", "ConfigMap"}
	for _, kind := range expectedKinds {
		goldenPath := fmt.Sprintf("%s/expected_%s.yaml", testDirPath, kind)
		actualResources := resourcesByKind[kind]

		// Check if golden file exists
		if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
			// No golden file for this kind
			if len(actualResources) > 0 {
				diffs = append(diffs, fmt.Sprintf("Unexpected %s generated (no expected_%s.yaml expected)", kind, kind))
			}
			continue
		}

		// Golden file exists - read it
		expectedData, err := os.ReadFile(goldenPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", goldenPath, err)
		}

		expectedStr := strings.TrimSpace(string(expectedData))
		if expectedStr == "" {
			// Empty golden file - expect no resources of this kind
			if len(actualResources) > 0 {
				diffs = append(diffs, fmt.Sprintf("Expected no %s, but got %d", kind, len(actualResources)))
			}
			continue
		}

		// Expected content - validate resources
		if len(actualResources) == 0 {
			diffs = append(diffs, fmt.Sprintf("Expected %s (from expected_%s.yaml), but none generated", kind, kind))
			continue
		}

		// Compare actual resources with golden file
		var actualYAML bytes.Buffer
		for i, res := range actualResources {
			if i > 0 {
				actualYAML.WriteString("---\n")
			}
			yamlBytes, err := sigsyaml.Marshal(res.Object)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal %s to YAML: %w", kind, err)
			}
			actualYAML.Write(yamlBytes)
		}

		// Normalize and compare
		normExpected, err := NormalizeYAML(expectedStr)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize expected_%s.yaml: %w", kind, err)
		}

		normActual, err := NormalizeYAML(actualYAML.String())
		if err != nil {
			return nil, fmt.Errorf("failed to normalize actual %s: %w", kind, err)
		}

		if normExpected != normActual {
			diffs = append(diffs, fmt.Sprintf("%s mismatch (see expected_%s.yaml)", kind, kind))
			// Add detailed line diff
			expectedLines := strings.Split(normExpected, "\n")
			actualLines := strings.Split(normActual, "\n")
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
					diffs = append(diffs, fmt.Sprintf("  Line %d:", i+1))
					if expLine != "" {
						diffs = append(diffs, fmt.Sprintf("    Expected: %s", expLine))
					}
					if actLine != "" {
						diffs = append(diffs, fmt.Sprintf("    Actual:   %s", actLine))
					}
				}
			}
		}
	}

	return diffs, nil
}
