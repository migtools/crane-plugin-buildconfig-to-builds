package e2e

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/migtools/crane-plugin-buildconfig-to-shipwright/tests/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildConfig to Shipwright Conversion", func() {

	// Table-driven test: one entry per test directory
	// All tests validate: actual plugin output matches expected_output.yaml
	DescribeTable("should convert BuildConfig to Shipwright Build correctly",
		func(testDir, issueNumber, description string) {
			// Setup paths
			testDirPath := filepath.Join(projectRoot, "tests", "testdata", testDir)
			buildConfigPath := filepath.Join(testDirPath, "buildconfig.yaml")
			flagsPath := filepath.Join(testDirPath, "flags.json")

			By(fmt.Sprintf("Running plugin on %s", testDir))

			// Load optional flags
			flags, err := framework.LoadOptionalFlags(flagsPath)
			Expect(err).NotTo(HaveOccurred())

			// Run plugin - returns ALL resources (Build, ServiceAccount, ConfigMap)
			resources, err := framework.RunPluginOnYAML(buildConfigPath, flags)
			Expect(err).NotTo(HaveOccurred())

			// Compare output with expected
			By(fmt.Sprintf("Comparing output (%d resource(s)) with expected", len(resources)))
			diffs, err := framework.CompareBuildsWithGoldenFile(resources, testDirPath)
			Expect(err).NotTo(HaveOccurred())

			if len(diffs) > 0 {
				Fail(fmt.Sprintf("Output differs from expected:\n  %s",
					strings.Join(diffs, "\n  ")))
			}

			// If expected_annotations.json exists, also validate annotations
			expectedAnnotationsPath := filepath.Join(testDirPath, "expected_annotations.json")
			expectedAnnotations, err := framework.LoadExpectedAnnotations(expectedAnnotationsPath)
			Expect(err).NotTo(HaveOccurred())

			if expectedAnnotations != nil {
				By("Validating outcome/reason annotations")

				// Get plugin response to check patches
				response, err := framework.RunPluginAndGetResponse(buildConfigPath, flags)
				Expect(err).NotTo(HaveOccurred())

				// Extract annotations from patches
				actualAnnotations, err := framework.ExtractAnnotationsFromPatches(response.Patches)
				Expect(err).NotTo(HaveOccurred())

				// Validate each expected annotation
				for key, expectedValue := range expectedAnnotations {
					actualValue, found := actualAnnotations[key]
					Expect(found).To(BeTrue(), fmt.Sprintf("Expected annotation %s not found", key))
					Expect(actualValue).To(Equal(expectedValue), fmt.Sprintf("Annotation %s mismatch", key))
				}
			}
		},

		// Test cases - one Entry per test directory
		// Format: Entry(label, testDir, issue, description)
		// All tests validate: actual output matches expected_output.yaml

		// Templates/Lists - expected output: empty (plugin ignores non-BuildConfigs)
		Entry("[#833] datagrid-hotrod", "01-datagrid-hotrod", "833", "Template wrapper - ignored"),
		Entry("[#834] cakephp-mysql", "02-cakephp-mysql", "834", "List wrapper - ignored"),
		Entry("[#839] custom-strategy", "07-custom-strategy", "839", "List wrapper - ignored"),
		Entry("[#845] generic-test-build", "13-generic-test-build", "845", "List wrapper - ignored"),
		Entry("[#846] docker-postcommit", "14-docker-postcommit", "846", "List wrapper - ignored"),
		Entry("[#847] build-with-proxy", "15-build-with-proxy", "847", "List wrapper - ignored"),
		Entry("[#848] imagesource-cross-namespace", "16-imagesource-cross-namespace", "848", "List wrapper - ignored"),

		// Unsupported BuildConfigs - expected output: empty (rejected with annotations)
		Entry("[#838] jenkins-pipeline", "06-jenkins-pipeline", "838", "JenkinsPipeline rejected"),
		Entry("[#843] s2i-with-volumes", "11-s2i-with-volumes", "843", "Binary archive rejected"),
		Entry("[#844] pullsecret-nodejs", "12-pullsecret-nodejs", "844", "Missing output rejected"),

		// Successful conversions - expected output: Build resources
		Entry("[#835] docker-and-s2i", "03-docker-and-s2i", "835", "Multi-BuildConfig (2 Builds)"),
		Entry("[#836] webapp-docker", "04-webapp-docker", "836", "Docker strategy"),
		Entry("[#837] api-s2i", "05-api-s2i", "837", "S2I strategy"),
		Entry("[#840] docker-with-envvars", "08-docker-with-envvars", "840", "Docker with envvars"),
		Entry("[#841] s2i-with-envvars", "09-s2i-with-envvars", "841", "S2I with envvars"),
		Entry("[#842] docker-with-volumes", "10-docker-with-volumes", "842", "Docker with volumes"),
		Entry("[#849] docker-nocache", "17-docker-nocache", "849", "Docker nocache"),
		Entry("[#850] serviceaccount-override", "18-serviceaccount-override", "850", "ServiceAccount override"),
		Entry("[PR#60] docker-imagestream-ruby", "19-docker-imagestream-ruby", "PR60", "Docker with ImageStream"),
		Entry("[PR#60] s2i-imagestream-nodejs", "20-s2i-imagestream-nodejs", "PR60", "S2I with ImageStream"),
	)
})
