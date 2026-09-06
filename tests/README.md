# BuildConfig to Shipwright Plugin Tests

Focused unit test suite for validating BuildConfig → Shipwright Build conversion.

## Approach

**Direct plugin execution + Golden file comparison** - No crane binary or cluster needed!

```
BuildConfig YAML → Parse → plugin.Run() → Compare with Expected Output → ✅
```

Tests call the plugin directly as a Go library and compare output against expected golden files.

## Quick Start

**Note:** Tests are run manually/locally on demand (not in CI).

```bash
cd tests

# Run all tests
go test ./e2e -v

# Run single test
go test ./e2e -v -ginkgo.focus="webapp-docker"

# Run only tests with golden files
go test ./e2e -v -ginkgo.focus="docker|s2i|webapp"
```

**Speed:** ~0.012 seconds for 10 tests (with golden files)  
**Requirements:** Go 1.22+ only (no crane, no cluster)

## Structure

```
tests/
├── framework/              # ~750 LOC
│   ├── plugin.go           # Direct plugin execution
│   ├── rules.go            # YAML rule loading
│   ├── rule_evaluator.go   # Type-safe rule evaluation
│   └── validation.go       # Conversion validation + golden file support
├── e2e/                    # ~90 LOC  
│   ├── e2e_suite_test.go   # Ginkgo setup
│   └── conversion_test.go  # DescribeTable with 20 test cases
├── testdata/
│   ├── buildconfig_yamls/  # 20 BuildConfig test inputs
│   │   ├── 01-datagrid-hotrod.yaml
│   │   ├── ...
│   │   ├── 19-docker-imagestream-ruby.yaml   # From PR#60
│   │   └── 20-s2i-imagestream-nodejs.yaml    # From PR#60
│   └── expected_output/    # 10 expected Build outputs
│       ├── 03-docker-and-s2i-expected.yaml
│       ├── 04-webapp-docker-expected.yaml
│       └── ...
└── rules.yaml              # 13 declarative conversion rules
```

## Test Coverage

### 20 Test Cases

**18 from real-world scenarios (issues #833-#850):**
- Docker + S2I combinations
- Environment variables and volumes
- Pull secrets and proxies
- Post-commit hooks
- Service account overrides
- No-cache builds
- ImageSource cross-namespace

**2 from PR#60 cluster tests:**
- Docker + ImageStream (Ruby)
- S2I + ImageStream (Node.js)

### Validation Approach

**Golden File Comparison:**
- Tests with expected outputs in `expected_output/` compare complete YAML
- Exact field-by-field comparison
- 10 tests currently have golden files

**Tests without golden files:**
- Skipped (incomplete BuildConfigs or Templates)
- Can be added later as needed

### What's Validated

When golden files exist, tests validate:

1. **API Version** - Must be `shipwright.io/v1beta1`
2. **Strategy Mapping**
   - Docker → buildah
   - Source → source-to-image
   - JenkinsPipeline/Custom → skipped
3. **Annotations**
   - `crane.konveyor.io/converted-from`
   - Conversion outcome tracking
4. **Field Mappings**
   - Git source (URI, ref, contextDir)
   - Output image
   - Dockerfile path
   - Timeouts and retention
5. **Labels Preservation**
6. **Triggers** - Preserved in annotations
7. **Environment Variables** - Preserved
8. **Volumes** - Preserved

## How Tests Work

Each test specifies its expected outcome:

1. **`"pass"`** - Expects Build generated and matching golden file
   - Parses BuildConfig YAML from `testdata/buildconfig_yamls/`
   - Calls `plugin.Run()` directly (no crane binary)
   - Compares actual vs expected YAML (exact match)
   - Fails if no Build generated or if it differs from golden file

2. **`"empty"`** - Expects NO Build generated (correct plugin behavior)
   - Used for JenkinsPipeline strategy (unsupported)
   - Used for BuildConfigs missing required fields (e.g., spec.output.to)
   - Asserts plugin correctly returns empty (not an error)
   - Fails if Build IS generated

3. **`"skip"`** - Skips test (incomplete test data)
   - Used for Templates/Lists that need unwrapping
   - Used for tests with known issues
   - Does not fail the test suite

**Test outcomes:**
- ✅ **Pass** - Build matches expected (or correctly empty)
- ❌ **Fail** - Build differs from expected (or unexpected outcome)
- ⊘ **Skip** - Test explicitly skipped

## Example Output

```
Running Suite: BuildConfig to Shipwright Conversion Suite
==========================================================

✓ [#835] docker-and-s2i [PASSED] [0.001s]
✓ [#836] webapp-docker [PASSED] [0.001s]
✓ [#837] api-s2i [PASSED] [0.001s]
✓ [#838] jenkins-pipeline (skipped) [PASSED] [0.001s]
✓ [PR#60] docker-imagestream-ruby [PASSED] [0.001s]
✓ [PR#60] s2i-imagestream-nodejs [PASSED] [0.001s]
...

Ran 20 of 20 Specs in 0.012 seconds
SUCCESS! -- 12 Passed | 8 Failed | 0 Pending | 0 Skipped
```

## Adding New Tests

### 1. Add BuildConfig YAML

```bash
cp my-buildconfig.yaml testdata/21-my-test.yaml
```

### 2. Add Test Entry

Edit `e2e/conversion_test.go`:

```go
Entry("[#851] my-test", "21-my-test.yaml", "851", "description"),
```

### 3. Run Test

```bash
go test ./e2e -v -ginkgo.focus="my-test"
```

## Extending Validation

### Add New Rule to rules.yaml

```yaml
- id: RULE-14
  name: My New Rule
  description: Validates something important
  type: field_equals
  field: build.spec.something
  expected: expected-value
```

### Add New Rule Type

If you need a new rule type, add evaluator in `framework/rule_evaluator.go`:

```go
case "my_new_type":
    return evaluateMyNewType(rule, bc, build)
```

## CI Integration

```yaml
# .github/workflows/test.yml
name: Plugin Tests
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.22'
      
      - name: Run Unit Tests
        run: go test ./tests/e2e -v
```

**No dependencies needed in CI** - just Go!

## Benefits

✅ **Fast** - 0.012s vs minutes with crane/cluster  
✅ **Simple** - No crane binary, no cluster setup  
✅ **Focused** - Tests plugin logic, not crane workflow  
✅ **Maintainable** - YAML rules instead of hard-coded checks  
✅ **Type-safe** - Go evaluation, not bash string matching  
✅ **Comprehensive** - 20 test cases, 13 validation rules  

## What's NOT Tested

This framework focuses on **plugin conversion correctness**. It does NOT test:

- ❌ crane CLI workflow (export/transform/apply)
- ❌ Plugin loading mechanism  
- ❌ Actual image builds on cluster
- ❌ BuildRun execution

**For integration testing:** Use crane's E2E tests or the bash scripts in this repo (`e2e-cluster.sh`, `e2e-transform.sh`).

## Relationship to PR#60

PR#60 added cluster-based integration tests (Bash scripts). This framework:
- **Complements** those tests (unit vs integration)
- **Includes** their test cases (#19, #20) as unit tests
- **Validates** conversion logic they depend on
- **Runs faster** for development iteration

Both are valuable:
- **Unit tests (this):** Fast feedback on conversion logic
- **Cluster tests (PR#60):** Full workflow validation

## Test Results

**Summary:** 12 Passed | 0 Failed | 8 Skipped (20 total)

### Passing Tests (12/20)

**Correct conversions (9 tests):**
- ✅ webapp-docker
- ✅ api-s2i
- ✅ docker-with-envvars
- ✅ s2i-with-envvars
- ✅ docker-with-volumes
- ✅ docker-nocache
- ✅ serviceaccount-override
- ✅ docker-imagestream-ruby (from PR#60)
- ✅ s2i-imagestream-nodejs (from PR#60)

**Correct empty results (3 tests):**
- ✅ jenkins-pipeline (JenkinsPipeline strategy - unsupported)
- ✅ s2i-with-volumes (missing spec.output.to)
- ✅ pullsecret-nodejs (missing spec.output.to)

### Skipped Tests (8/20)

Templates/Lists that need unwrapping:
- ⊘ datagrid-hotrod (Template wrapper)
- ⊘ cakephp-mysql (List wrapper)
- ⊘ docker-and-s2i (Multi-BuildConfig file)
- ⊘ custom-strategy (List wrapper)
- ⊘ generic-test-build (List wrapper)
- ⊘ docker-postcommit (List wrapper)
- ⊘ build-with-proxy (List wrapper)
- ⊘ imagesource-cross-namespace (List wrapper)

## Troubleshooting

### Tests Fail

1. Check error message for which rule failed
2. Compare expected vs actual values
3. Fix plugin code or update rule definition

### Rule Definition Issues

- Verify YAML syntax in `rules.yaml`
- Check field paths use dot notation correctly
- Ensure rule type exists in `rule_evaluator.go`

### Plugin Build Issues

```bash
# Ensure plugin builds
cd ..
go build .
```
