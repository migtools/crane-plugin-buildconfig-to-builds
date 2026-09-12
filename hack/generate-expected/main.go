package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/konveyor/crane-lib/transform"
	"github.com/migtools/crane-plugin-buildconfig-to-shipwright/buildconfig"
	"github.com/sirupsen/logrus/hooks/test"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	sigsyaml "sigs.k8s.io/yaml"
)

func main() {
	testdataDir := "../../tests/testdata"

	// Get all test directories (exclude old buildconfig_yamls and expected_output)
	entries, err := os.ReadDir(testdataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading testdata directory: %v\n", err)
		os.Exit(1)
	}

	var testDirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip old directories and special directories
		name := entry.Name()
		if name == "buildconfig_yamls" || name == "expected_output" ||
		   strings.HasPrefix(name, "e2e-") || strings.HasPrefix(name, ".") {
			continue
		}
		testDirs = append(testDirs, name)
	}
	sort.Strings(testDirs)

	logger, _ := test.NewNullLogger()
	plugin := &buildconfig.BuildConfigTransformPlugin{Log: logger}

	for _, testDir := range testDirs {
		testDirPath := filepath.Join(testdataDir, testDir)
		buildConfigPath := filepath.Join(testDirPath, "buildconfig.yaml")
		flagsPath := filepath.Join(testDirPath, "flags.json")
		outputFile := filepath.Join(testDirPath, "expected_output.yaml")

		fmt.Printf("Processing: %s\n", testDir)

		// Check if buildconfig.yaml exists
		if _, err := os.Stat(buildConfigPath); os.IsNotExist(err) {
			fmt.Printf("  SKIP: No buildconfig.yaml found\n")
			continue
		}

		// Read BuildConfig YAML
		data, err := os.ReadFile(buildConfigPath)
		if err != nil {
			fmt.Printf("  SKIP: Failed to read buildconfig.yaml: %v\n", err)
			continue
		}

		// Parse multi-document YAML
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		var buildConfigFound *unstructured.Unstructured

		for {
			var obj map[string]interface{}
			if err := decoder.Decode(&obj); err != nil {
				if err == io.EOF {
					break
				}
				continue // Skip unparseable documents
			}

			if len(obj) == 0 {
				continue
			}

			resource := &unstructured.Unstructured{Object: obj}

			// Look for BuildConfig
			if resource.GetKind() == "BuildConfig" {
				buildConfigFound = resource
				break
			}
		}

		if buildConfigFound == nil {
			fmt.Printf("  SKIP: No BuildConfig found in file\n")
			continue
		}

		// Load optional flags (imagestream-mapping, registry-mapping)
		flags := loadOptionalFlags(flagsPath)

		// Run plugin with flags
		request := transform.PluginRequest{
			Unstructured: *buildConfigFound,
			Extras:       flags,
		}
		response, err := plugin.Run(request)
		if err != nil {
			fmt.Printf("  SKIP: Plugin error: %v\n", err)
			continue
		}

		// Check if any Build was generated
		if len(response.NewResources) == 0 {
			fmt.Printf("  SKIP: No Build generated (likely skipped strategy)\n")
			continue
		}

		// Take the first Build (most BuildConfigs generate one Build)
		build := response.NewResources[0]

		// Convert to YAML
		outputYAML, err := sigsyaml.Marshal(build.Object)
		if err != nil {
			fmt.Printf("  ERROR: Failed to marshal output: %v\n", err)
			continue
		}

		// Write to file
		if err := os.WriteFile(outputFile, outputYAML, 0644); err != nil {
			fmt.Printf("  ERROR: Failed to write output: %v\n", err)
			continue
		}

		fmt.Printf("  ✓ Generated: expected_output.yaml\n")
	}

	fmt.Println("\nDone! Generated expected outputs")
}

// loadOptionalFlags loads optional plugin flags from a JSON file.
// Returns nil if the flags file doesn't exist (not an error).
func loadOptionalFlags(flagsPath string) map[string]string {
	// Check if flags file exists
	if _, err := os.Stat(flagsPath); os.IsNotExist(err) {
		return nil // No flags file, return nil (not an error)
	}

	// Read flags file
	data, err := os.ReadFile(flagsPath)
	if err != nil {
		fmt.Printf("  WARNING: Failed to read flags file: %v\n", err)
		return nil
	}

	// Parse JSON
	var flags map[string]string
	if err := json.Unmarshal(data, &flags); err != nil {
		fmt.Printf("  WARNING: Failed to parse flags file: %v\n", err)
		return nil
	}

	return flags
}
