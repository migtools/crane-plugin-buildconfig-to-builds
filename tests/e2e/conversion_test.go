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

	// Table-driven test: one entry per test directory
	// expectedOutcome values:
	//   "pass"  - expects Build generated and matches golden file
	//   "empty" - expects no Build generated (correct behavior for unsupported/incomplete)
	//   "skip"  - skip test (incomplete test data, need to fix)
	DescribeTable("should convert BuildConfig to Shipwright Build correctly",
		func(testDir, issueNumber, description, expectedOutcome string) {
			// Setup paths - each test is in its own directory with buildconfig.yaml
			testDirPath := filepath.Join(projectRoot, "tests", "testdata", testDir)
			buildConfigPath := filepath.Join(testDirPath, "buildconfig.yaml")
			flagsPath := filepath.Join(testDirPath, "flags.json")
			expectedPath := filepath.Join(testDirPath, "expected_output.yaml")

			// Handle different expected outcomes
			switch expectedOutcome {
			case "skip":
				Skip(fmt.Sprintf("Skipping %s - %s", testDir, description))
				return

			case "empty":
				// Test expects NO Build generated (e.g., JenkinsPipeline, missing output)
				By(fmt.Sprintf("Running plugin on %s (expecting no output)", testDir))

				// Load optional flags (imagestream-mapping, registry-mapping)
				flags, err := framework.LoadOptionalFlags(flagsPath)
				Expect(err).NotTo(HaveOccurred())

				builds, err := framework.RunPluginOnYAML(buildConfigPath, flags)
				Expect(err).NotTo(HaveOccurred())
				Expect(builds).To(BeEmpty(), fmt.Sprintf("Expected no Build for %s, but got %d", testDir, len(builds)))

			case "pass":
				// Test expects Build generated and matching golden file
				By(fmt.Sprintf("Running plugin on %s", testDir))

				// Load optional flags (imagestream-mapping, registry-mapping)
				flags, err := framework.LoadOptionalFlags(flagsPath)
				Expect(err).NotTo(HaveOccurred())

				builds, err := framework.RunPluginOnYAML(buildConfigPath, flags)
				Expect(err).NotTo(HaveOccurred())
				Expect(builds).NotTo(BeEmpty(), "No Build resources generated")

				By(fmt.Sprintf("Validating %d generated Build(s)", len(builds)))
				for _, buildObj := range builds {
					By(fmt.Sprintf("Comparing with golden file: expected_output.yaml"))

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

		// Test cases - one Entry per test directory
		// Format: Entry(label, testDir, issue, description, expectedOutcome)

		// Templates/Lists - skip
		Entry("[#833] datagrid-hotrod", "01-datagrid-hotrod", "833", "Template wrapper - need to extract BuildConfig", "skip"),
		Entry("[#834] cakephp-mysql", "02-cakephp-mysql", "834", "List wrapper - need to extract BuildConfig", "skip"),
		Entry("[#835] docker-and-s2i", "03-docker-and-s2i", "835", "Multi-BuildConfig file - deleted golden file", "skip"),
		Entry("[#839] custom-strategy", "07-custom-strategy", "839", "List wrapper - need to extract BuildConfig", "skip"),
		Entry("[#845] generic-test-build", "13-generic-test-build", "845", "List wrapper - need to extract BuildConfig", "skip"),
		Entry("[#846] docker-postcommit", "14-docker-postcommit", "846", "List wrapper - need to extract BuildConfig", "skip"),
		Entry("[#847] build-with-proxy", "15-build-with-proxy", "847", "List wrapper - need to extract BuildConfig", "skip"),
		Entry("[#848] imagesource-cross-namespace", "16-imagesource-cross-namespace", "848", "List wrapper - need to extract BuildConfig", "skip"),

		// Missing required fields - expect empty (plugin correctly skips)
		Entry("[#843] s2i-with-volumes", "11-s2i-with-volumes", "843", "Missing spec.output.to - plugin correctly returns empty", "empty"),
		Entry("[#844] pullsecret-nodejs", "12-pullsecret-nodejs", "844", "Missing spec.output.to - plugin correctly returns empty", "empty"),

		// Unsupported strategies - expect empty (plugin correctly skips)
		Entry("[#838] jenkins-pipeline", "06-jenkins-pipeline", "838", "JenkinsPipeline strategy - plugin correctly returns empty", "empty"),

		// Valid conversions - expect pass
		Entry("[#836] webapp-docker", "04-webapp-docker", "836", "webapp-docker", "pass"),
		Entry("[#837] api-s2i", "05-api-s2i", "837", "api-s2i", "pass"),
		Entry("[#840] docker-with-envvars", "08-docker-with-envvars", "840", "docker-with-envvars", "pass"),
		Entry("[#841] s2i-with-envvars", "09-s2i-with-envvars", "841", "s2i-with-envvars", "pass"),
		Entry("[#842] docker-with-volumes", "10-docker-with-volumes", "842", "docker-with-volumes", "pass"),
		Entry("[#849] docker-nocache", "17-docker-nocache", "849", "docker-nocache", "pass"),
		Entry("[#850] serviceaccount-override", "18-serviceaccount-override", "850", "serviceaccount-override", "pass"),
		Entry("[PR#60] docker-imagestream-ruby", "19-docker-imagestream-ruby", "PR60", "docker-imagestream-ruby from dev's cluster tests", "pass"),
		Entry("[PR#60] s2i-imagestream-nodejs", "20-s2i-imagestream-nodejs", "PR60", "s2i-imagestream-nodejs from dev's cluster tests", "pass"),
	)
})
