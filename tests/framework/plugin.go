package framework

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/konveyor/crane-lib/transform"
	"github.com/migtools/crane-plugin-buildconfig-to-shipwright/buildconfig"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RunPluginOnYAML executes the plugin on a YAML file and returns generated Build resources.
// The extras parameter contains optional flags like imagestream-mapping and registry-mapping.
func RunPluginOnYAML(yamlPath string, extras map[string]string) ([]*unstructured.Unstructured, error) {
	// Read and parse YAML file
	resources, err := ParseYAML(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Create plugin instance with null logger (suppress logs)
	logger, _ := logrustest.NewNullLogger()
	plugin := &buildconfig.BuildConfigTransformPlugin{
		Log: logger,
	}

	// Run plugin on each resource
	var builds []*unstructured.Unstructured
	for _, res := range resources {
		// Only process BuildConfig resources
		if res.GetKind() != "BuildConfig" {
			continue
		}

		// Create plugin request with optional flags
		request := transform.PluginRequest{
			Unstructured: *res,
			Extras:       extras,
		}

		// Run plugin
		response, err := plugin.Run(request)
		if err != nil {
			return nil, fmt.Errorf("plugin execution failed: %w", err)
		}

		// Extract Build resources from response
		for _, newRes := range response.NewResources {
			if newRes.GetKind() == "Build" && strings.Contains(newRes.GetAPIVersion(), "shipwright") {
				builds = append(builds, &newRes)
			}
		}
	}

	return builds, nil
}

// ParseYAML reads a multi-document YAML file and returns unstructured resources.
func ParseYAML(yamlPath string) ([]*unstructured.Unstructured, error) {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var resources []*unstructured.Unstructured
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))

	for {
		var doc map[string]interface{}
		if err := decoder.Decode(&doc); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode YAML: %w", err)
		}

		if len(doc) == 0 {
			continue
		}

		// Convert to unstructured
		obj := &unstructured.Unstructured{Object: doc}
		resources = append(resources, obj)
	}

	return resources, nil
}

// UnstructuredToYAML converts an unstructured resource to YAML bytes.
func UnstructuredToYAML(obj *unstructured.Unstructured) ([]byte, error) {
	return yaml.Marshal(obj.Object)
}

// LoadOptionalFlags loads optional plugin flags from a JSON file.
// Returns nil if the flags file doesn't exist (not an error).
func LoadOptionalFlags(flagsPath string) (map[string]string, error) {
	// Check if flags file exists
	if _, err := os.Stat(flagsPath); os.IsNotExist(err) {
		return nil, nil // No flags file, return empty map (not an error)
	}

	// Read flags file
	data, err := os.ReadFile(flagsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read flags file %s: %w", flagsPath, err)
	}

	// Parse JSON
	var flags map[string]string
	if err := json.Unmarshal(data, &flags); err != nil {
		return nil, fmt.Errorf("failed to parse flags file %s: %w", flagsPath, err)
	}

	return flags, nil
}
