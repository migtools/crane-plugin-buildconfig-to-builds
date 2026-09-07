package main

import (
	"bytes"
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
	inputDir := "../../tests/testdata/buildconfig_yamls"
	outputDir := "../../tests/testdata/expected_output"

	// Get all YAML files
	files, err := filepath.Glob(filepath.Join(inputDir, "*.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing files: %v\n", err)
		os.Exit(1)
	}
	sort.Strings(files)

	logger, _ := test.NewNullLogger()
	plugin := &buildconfig.BuildConfigTransformPlugin{Log: logger}

	for _, file := range files {
		basename := filepath.Base(file)
		name := strings.TrimSuffix(basename, ".yaml")
		outputFile := filepath.Join(outputDir, name+"-expected.yaml")

		fmt.Printf("Processing: %s\n", basename)

		// Read BuildConfig YAML
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("  SKIP: Failed to read file: %v\n", err)
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

		// Run plugin
		request := transform.PluginRequest{Unstructured: *buildConfigFound}
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

		fmt.Printf("  ✓ Generated: %s-expected.yaml\n", name)
	}

	fmt.Println("\nDone! Generated expected outputs")
}
