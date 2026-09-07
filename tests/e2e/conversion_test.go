package e2e

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/konveyor/crane-plugin-buildconfig-to-shipwright/tests/framework"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildConfig to Shipwright Conversion", func() {

	// Table-driven test: one entry per BuildConfig YAML
	// expectedOutcome values:
	//   "pass"  - expects Build generated and matches golden file
	//   "empty" - expects no Build generated (correct behavior for unsupported/incomplete)
	//   "skip"  - skip test (incomplete test data, need to fix)
	DescribeTable("should convert BuildConfig to Shipwright Build correctly",
		func(testFile, issueNumber, description, expectedOutcome string) {
			// Setup paths
			testDataPath := filepath.Join(projectRoot, "tests", "testdata", "buildconfig_yamls", testFile)

			// Handle different expected outcomes
			switch expectedOutcome {
			case "skip":
				Skip(fmt.Sprintf("Skipping %s - %s", testFile, description))
				return

			case "empty":
				// Test expects NO Build generated (e.g., JenkinsPipeline, missing output)
				By(fmt.Sprintf("Running plugin on %s (expecting no output)", testFile))
				builds, err := framework.RunPluginOnYAML(testDataPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(builds).To(BeEmpty(), fmt.Sprintf("Expected no Build for %s, but got %d", testFile, len(builds)))

			case "pass":
				// Test expects Build generated and matching golden file
				expectedFile := strings.TrimSuffix(testFile, ".yaml") + "-expected.yaml"
				expectedPath := filepath.Join(projectRoot, "tests", "testdata", "expected_output", expectedFile)

				By(fmt.Sprintf("Running plugin on %s", testFile))
				builds, err := framework.RunPluginOnYAML(testDataPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(builds).NotTo(BeEmpty(), "No Build resources generated")

				By(fmt.Sprintf("Validating %d generated Build(s)", len(builds)))
				for _, buildObj := range builds {
					By(fmt.Sprintf("Comparing with golden file: %s", expectedFile))

					// No variable expansion needed for our tests
					vars := map[string]string{}
					diffs, err := framework.CompareWithGoldenFile(buildObj, expectedPath, vars)
					Expect(err).NotTo(HaveOccurred())

					if len(diffs) > 0 {
						Fail(fmt.Sprintf("Build differs from expected output:\n  %s",
							strings.Join(diffs, "\n  ")))
					}
				}

			default:
				Fail(fmt.Sprintf("Invalid expectedOutcome: %s (must be 'pass', 'empty', or 'skip')", expectedOutcome))
			}
		},

		// Test cases - one Entry per BuildConfig YAML file
		// Format: Entry(label, file, issue, description, expectedOutcome)

		// Templates/Lists - skip until we add unwrapping support
		Entry("[#833] datagrid-hotrod", "01-datagrid-hotrod.yaml", "833", "Template wrapper - need to extract BuildConfig", "skip"),
		Entry("[#834] cakephp-mysql", "02-cakephp-mysql.yaml", "834", "List wrapper - need to extract BuildConfig", "skip"),
		Entry("[#835] docker-and-s2i", "03-docker-and-s2i.yaml", "835", "Multi-BuildConfig file - deleted golden file", "skip"),
		Entry("[#839] custom-strategy", "07-custom-strategy.yaml", "839", "List wrapper - need to extract BuildConfig", "skip"),
		Entry("[#845] generic-test-build", "13-generic-test-build.yaml", "845", "List wrapper - need to extract BuildConfig", "skip"),
		Entry("[#846] docker-postcommit", "14-docker-postcommit.yaml", "846", "List wrapper - need to extract BuildConfig", "skip"),
		Entry("[#847] build-with-proxy", "15-build-with-proxy.yaml", "847", "List wrapper - need to extract BuildConfig", "skip"),
		Entry("[#848] imagesource-cross-namespace", "16-imagesource-cross-namespace.yaml", "848", "List wrapper - need to extract BuildConfig", "skip"),

		// Missing required fields - expect empty (plugin correctly skips)
		Entry("[#843] s2i-with-volumes", "11-s2i-with-volumes.yaml", "843", "Missing spec.output.to - plugin correctly returns empty", "empty"),
		Entry("[#844] pullsecret-nodejs", "12-pullsecret-nodejs.yaml", "844", "Missing spec.output.to - plugin correctly returns empty", "empty"),

		// Unsupported strategies - expect empty (plugin correctly skips)
		Entry("[#838] jenkins-pipeline", "06-jenkins-pipeline.yaml", "838", "JenkinsPipeline strategy - plugin correctly returns empty", "empty"),

		// Valid conversions - expect pass
		Entry("[#836] webapp-docker", "04-webapp-docker.yaml", "836", "webapp-docker", "pass"),
		Entry("[#837] api-s2i", "05-api-s2i.yaml", "837", "api-s2i", "pass"),
		Entry("[#840] docker-with-envvars", "08-docker-with-envvars.yaml", "840", "docker-with-envvars", "pass"),
		Entry("[#841] s2i-with-envvars", "09-s2i-with-envvars.yaml", "841", "s2i-with-envvars", "pass"),
		Entry("[#842] docker-with-volumes", "10-docker-with-volumes.yaml", "842", "docker-with-volumes", "pass"),
		Entry("[#849] docker-nocache", "17-docker-nocache.yaml", "849", "docker-nocache", "pass"),
		Entry("[#850] serviceaccount-override", "18-serviceaccount-override.yaml", "850", "serviceaccount-override", "pass"),
		Entry("[PR#60] docker-imagestream-ruby", "19-docker-imagestream-ruby.yaml", "PR60", "docker-imagestream-ruby from dev's cluster tests", "pass"),
		Entry("[PR#60] s2i-imagestream-nodejs", "20-s2i-imagestream-nodejs.yaml", "PR60", "s2i-imagestream-nodejs from dev's cluster tests", "pass"),
	)
})
