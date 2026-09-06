# Analysis of 11 Skipped Tests - For Dev Discussion

## Summary

**9 tests pass** with golden files  
**11 tests skip** - Cannot generate expected output

---

## Issues Breakdown

### Issue #1: OpenShift Templates (2 files)

**Files:**
- `01-datagrid-hotrod.yaml`
- `02-cakephp-mysql.yaml`

**Problem:**
```yaml
kind: Template  # ← Not a BuildConfig
apiVersion: template.openshift.io/v1
objects:
  - kind: BuildConfig  # ← BuildConfig is nested inside Template
    metadata:
      name: datagrid-hotrod
```

**Why it fails:**
- Generator expects `kind: BuildConfig` at top level
- These are OpenShift Templates that CONTAIN BuildConfigs
- Templates have parameters that need to be processed first

**Question for devs:**
- Should we test Templates at all?
- Or should we extract the BuildConfig objects first?
- Are Templates out of scope for this plugin?

---

### Issue #2: List Objects (6 files)

**Files:**
- `07-custom-strategy.yaml`
- `13-generic-test-build.yaml`
- `14-docker-postcommit.yaml`
- `15-build-with-proxy.yaml`
- `16-imagesource-cross-namespace.yaml`

**Problem:**
```yaml
kind: List  # ← Not a BuildConfig
apiVersion: v1
items:
  - kind: ImageStream
    ...
  - kind: BuildConfig  # ← BuildConfig is one item in a list
    metadata:
      name: my-app
  - kind: Service
    ...
```

**Why it fails:**
- Generator expects single-document YAML with `kind: BuildConfig`
- These are multi-object Kubernetes Lists
- BuildConfig is buried inside the items array

**Question for devs:**
- Should test data be just BuildConfigs? (not Lists)
- Or should we extract BuildConfigs from Lists automatically?
- Are these real-world scenarios we need to support?

---

### Issue #3: Missing Output Spec (2 files)

**Files:**
- `11-s2i-with-volumes.yaml`
- `12-pullsecret-nodejs.yaml`

**Problem:**
```yaml
kind: BuildConfig
metadata:
  name: pullsecret-nodejs
spec:
  source:
    git:
      uri: "https://github.com/..."
  strategy:
    type: Source
    sourceStrategy:
      from:
        kind: DockerImage
        name: registry.redhat.io/ubi8/nodejs-14:latest
  # ❌ Missing: spec.output.to
```

**Why it fails:**
- Plugin REQUIRES `spec.output.to` field (where to push the built image)
- Without output, plugin returns NO Build (by design)
- Generator: "No Build generated (likely skipped strategy)"

**Check in plugin code:**
```go
// buildconfig/converter.go:162
if bc.Spec.Output.To == nil {
    return nil, Outcome{State: OutcomeSkipped, Reason: "no output"}
}
```

**Question for devs:**
- Is this correct plugin behavior? (skip if no output)
- Should test data always have `spec.output.to`?
- Or should we add it to these test files?

---

### Issue #4: Unsupported Strategy (1 file)

**Files:**
- `06-jenkins-pipeline.yaml`

**Problem:**
```yaml
kind: BuildConfig
metadata:
  name: my-pipeline
spec:
  strategy:
    type: JenkinsPipeline  # ← Not supported by Shipwright
```

**Why it fails:**
- JenkinsPipeline strategy is intentionally NOT converted
- Plugin correctly returns no Build
- This is EXPECTED behavior

**Question for devs:**
- This is correct, right? (JenkinsPipeline should skip)
- Should we keep this test to verify "skipping works"?
- Or remove it from test suite?

---

## Detailed File Analysis

### 01-datagrid-hotrod.yaml
```
Type: Template
Issue: BuildConfig nested in Template.objects[...]
Real-world: Yes (OpenShift export)
Fixable: Extract BuildConfig object
Worth fixing: Maybe (if Templates are in scope)
```

### 02-cakephp-mysql.yaml
```
Type: Template
Issue: BuildConfig nested in Template.objects[...]
Real-world: Yes (OpenShift export)
Fixable: Extract BuildConfig object
Worth fixing: Maybe (if Templates are in scope)
```

### 06-jenkins-pipeline.yaml
```
Type: Unsupported Strategy
Issue: JenkinsPipeline not supported by Shipwright
Real-world: Yes (common in OpenShift)
Fixable: No (correct behavior)
Worth fixing: N/A (working as intended)
```

### 07-custom-strategy.yaml
```
Type: List
Issue: BuildConfig in List.items[1]
Real-world: Yes (kubectl export format)
Fixable: Extract BuildConfig from list
Worth fixing: Yes
```

### 11-s2i-with-volumes.yaml
```
Type: Incomplete BuildConfig
Issue: Missing spec.output.to
Real-world: No (invalid BuildConfig)
Fixable: Add spec.output.to
Worth fixing: Yes
```

### 12-pullsecret-nodejs.yaml
```
Type: Incomplete BuildConfig
Issue: Missing spec.output.to
Real-world: No (invalid BuildConfig)
Fixable: Add spec.output.to
Worth fixing: Yes
```

### 13-generic-test-build.yaml
```
Type: List
Issue: BuildConfig in List.items[...]
Real-world: Yes
Fixable: Extract BuildConfig
Worth fixing: Yes
```

### 14-docker-postcommit.yaml
```
Type: List
Issue: BuildConfig in List.items[...]
Real-world: Yes
Fixable: Extract BuildConfig
Worth fixing: Yes
```

### 15-build-with-proxy.yaml
```
Type: List
Issue: BuildConfig in List.items[...]
Real-world: Yes
Fixable: Extract BuildConfig
Worth fixing: Yes
```

### 16-imagesource-cross-namespace.yaml
```
Type: List
Issue: BuildConfig in List.items[...]
Real-world: Yes
Fixable: Extract BuildConfig
Worth fixing: Yes
```

---

## Questions for Dev Discussion

### 1. Test Data Format

**Current:** Mix of single BuildConfigs, Lists, and Templates

**Question:**
- Should test data be ONLY BuildConfig objects?
- Or should we support Lists/Templates (more realistic)?
- Should we preprocess complex files to extract BuildConfigs?

### 2. Missing Output Behavior

**Current:** Plugin skips BuildConfigs without `spec.output.to`

**Question:**
- Is this correct behavior?
- Should we reject these test files as invalid?
- Or add output to make them complete?

### 3. Test Coverage Philosophy

**Current:** 9/20 tests pass, 11 skip

**Options:**
- **A:** Fix all test data (18-19 tests passing)
  - Extract BuildConfigs from Lists/Templates
  - Add missing output fields
  - Skip only JenkinsPipeline
  
- **B:** Keep as-is (9 tests passing)
  - Accept that some real-world files don't work
  - Focus on quality over quantity
  
- **C:** Remove broken tests (9 tests total)
  - Delete files that can't generate golden files
  - Cleaner test suite

**Question:** Which approach makes sense?

### 4. Real-World Scenarios

**Lists/Templates are common in:**
- OpenShift exports (`oc get -o yaml`)
- Multi-object YAML files
- Migration scenarios (crane export)

**Question:**
- Should the plugin handle these formats?
- Or is preprocessing (extracting BuildConfigs) expected?
- Is this the user's responsibility or plugin's?

---

## Current Plugin Behavior

From generator output, the plugin:

✅ **Handles:**
- Single BuildConfig documents
- Docker strategy → buildah
- Source strategy → source-to-image
- ImageStream references

❌ **Doesn't Handle:**
- Templates (needs preprocessing)
- Lists (needs extraction)
- BuildConfigs without output (skips)
- JenkinsPipeline strategy (skips - expected)

---

## Recommendation for Discussion

**Propose:**

1. **Fix easy ones** (11, 12): Add missing output
   - Result: 11/20 tests

2. **Extract BuildConfigs from Lists** (07, 13, 14, 15, 16)
   - Result: 17/20 tests

3. **Skip Templates** (01, 02): Too complex, out of scope?
   - Result: Templates stay skipped

4. **Keep JenkinsPipeline** (06): Validates skip behavior works

**Final state:** 17 pass, 3 skip (2 Templates + 1 JenkinsPipeline)

---

## Technical Details

### How Generator Works

```go
1. Read YAML file
2. Parse multi-document YAML
3. Find first BuildConfig object
   ├─ If Template/List → SKIP (no BuildConfig at top level)
   └─ If BuildConfig → Continue
4. Run plugin.Run(BuildConfig)
   ├─ If no output → SKIP (plugin returns empty)
   └─ If Build generated → Save as golden file
```

### Why Not Auto-Extract?

**Could we update generator to extract from Lists/Templates?**

Yes, but:
- Adds complexity
- Templates need parameter resolution
- Not testing what users actually send to plugin

**Question:** Should generator be smarter, or test data be simpler?

---

## Summary for Devs

**Issue:** 11 tests skip because:
- 2 are Templates (complex structure)
- 6 are Lists (BuildConfig nested in items)
- 2 missing required output field
- 1 intentionally unsupported (JenkinsPipeline)

**Options:**
1. Fix test data → 17-19 tests passing
2. Keep as-is → 9 tests passing
3. Remove broken tests → 9 tests total
