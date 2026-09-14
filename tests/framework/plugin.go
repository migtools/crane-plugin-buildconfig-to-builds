package framework

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/konveyor/crane-lib/transform"
	"github.com/migtools/crane-plugin-buildconfig-to-shipwright/buildconfig"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RunPluginOnYAML executes the plugin on a YAML file and returns ALL generated resources.
// The extras parameter contains optional flags like imagestream-mapping and registry-mapping.
// Returns: Build, ServiceAccount, ConfigMap - whatever the plugin generates.
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
	var allResources []*unstructured.Unstructured
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

		// Collect ALL generated resources (Build, ServiceAccount, ConfigMap)
		for _, newRes := range response.NewResources {
			allResources = append(allResources, &newRes)
		}
	}

	return allResources, nil
}

// RunPluginAndGetResponse executes the plugin on a YAML file and returns the full plugin response.
// This includes NewResources, Patches, and other response data.
// The extras parameter contains optional flags like imagestream-mapping and registry-mapping.
func RunPluginAndGetResponse(yamlPath string, extras map[string]string) (*transform.PluginResponse, error) {
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

	// Process first BuildConfig found
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

		// Run plugin and return full response
		response, err := plugin.Run(request)
		if err != nil {
			return nil, fmt.Errorf("plugin execution failed: %w", err)
		}

		return &response, nil
	}

	return nil, fmt.Errorf("no BuildConfig found in file")
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

// LoadExpectedAnnotations loads expected annotation values from a JSON file.
// Returns nil if the file doesn't exist (not an error).
func LoadExpectedAnnotations(annotationsPath string) (map[string]string, error) {
	// Check if annotations file exists
	if _, err := os.Stat(annotationsPath); os.IsNotExist(err) {
		return nil, nil // No annotations file, return nil (not an error)
	}

	// Read annotations file
	data, err := os.ReadFile(annotationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read annotations file %s: %w", annotationsPath, err)
	}

	// Parse JSON
	var annotations map[string]string
	if err := json.Unmarshal(data, &annotations); err != nil {
		return nil, fmt.Errorf("failed to parse annotations file %s: %w", annotationsPath, err)
	}

	return annotations, nil
}

// ExtractAnnotationsFromPatches extracts annotations from JSON Patch operations.
// The plugin adds annotations via Patches when a BuildConfig is rejected.
func ExtractAnnotationsFromPatches(patches interface{}) (map[string]string, error) {
	annotations := make(map[string]string)

	// Use reflection to handle jsonpatch.Patch (which is a slice)
	v := reflect.ValueOf(patches)
	if v.Kind() != reflect.Slice {
		return annotations, nil
	}

	// Iterate over patch operations
	for i := 0; i < v.Len(); i++ {
		op := v.Index(i)

		// Call Kind() method
		kindMethod := op.MethodByName("Kind")
		if !kindMethod.IsValid() {
			continue
		}
		kindResult := kindMethod.Call(nil)
		if len(kindResult) == 0 {
			continue
		}
		kind := kindResult[0].String()

		if kind != "add" {
			continue
		}

		// Call Path() method
		pathMethod := op.MethodByName("Path")
		if !pathMethod.IsValid() {
			continue
		}
		pathResults := pathMethod.Call(nil)
		if len(pathResults) < 2 {
			continue
		}
		if !pathResults[1].IsNil() { // error
			continue
		}
		path := pathResults[0].String()

		// Call ValueInterface() method
		valueMethod := op.MethodByName("ValueInterface")
		if !valueMethod.IsValid() {
			continue
		}
		valueResults := valueMethod.Call(nil)
		if len(valueResults) < 2 {
			continue
		}
		if !valueResults[1].IsNil() { // error
			continue
		}
		value := valueResults[0].Interface()

		// Extract annotations from patch operations
		if strings.HasPrefix(path, "/metadata/annotations/") {
			// Single annotation: /metadata/annotations/key
			key := strings.TrimPrefix(path, "/metadata/annotations/")
			// Unescape JSON Pointer (~ becomes ~0, / becomes ~1)
			key = strings.ReplaceAll(key, "~1", "/")
			key = strings.ReplaceAll(key, "~0", "~")
			if str, ok := value.(string); ok {
				annotations[key] = str
			}
		} else if path == "/metadata/annotations" {
			// Whole annotations map
			if annotMap, ok := value.(map[string]interface{}); ok {
				for k, v := range annotMap {
					if str, ok := v.(string); ok {
						annotations[k] = str
					}
				}
			}
		}
	}

	return annotations, nil
}
