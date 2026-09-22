//go:build !documentation

package buildconfig

import (
	"strings"
	"testing"
)

const (
	localTemplateWarnMarker = "has a Local source, which starts only through"
	localResourcesSentence  = "no flag for step resources"
	localAccountSentence    = "--sa-name"
	templateWarnMarker      = "Resource requirements are not supported on Shipwright Build"
	customStepsWarnMarker   = "is a custom mapping with unknown step names"
)

// resourcesSpec is a Parallel-runPolicy Docker BuildConfig with a DockerImage
// output and a push secret, so the only resource-dependent warnings are the
// ones under test.
func resourcesSpec(source string) string {
	return `{
		"runPolicy": "Parallel",
		"source": ` + source + `,
		"strategy": {"type": "Docker", "dockerStrategy": {}},
		"resources": {"requests": {"cpu": "500m"}, "limits": {"memory": "2Gi", "cpu": "2"}},
		"output": {"to": {"kind": "DockerImage", "name": "quay.io/example/app:latest"}, "pushSecret": {"name": "push"}}
	}`
}

// BUILD-2477: a binary source with resources gets one warning saying the build
// runs with the strategy's defaults, instead of W48 pointing at a template that
// cannot start a Local-source Build. The template is still written.
func TestConvertBinarySourceWithResources(t *testing.T) {
	tests := []struct {
		name string
		// wantStepResources is false for a custom strategy mapping, whose step
		// names are unknown, so the template carries none and the warning is
		// the only record of the values.
		wantStepResources bool
		source            string
		opts              PluginOptionalFields
	}{
		{"binary", true, `{"type": "Binary", "binary": {}}`, PluginOptionalFields{}},
		{"binary with asFile", true, `{"type": "Binary", "binary": {"asFile": "app.jar"}}`, PluginOptionalFields{}},
		{"binary with custom strategy", false, `{"type": "Binary", "binary": {}}`, PluginOptionalFields{StrategyMapping: map[string]string{"docker": "buildah-with-volumes"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _, warns := convertOutputSpec(t, resourcesSpec(tt.source), tt.opts)

			if n := countContaining(warns, localTemplateWarnMarker); n != 1 {
				t.Fatalf("Local-source template warnings = %d, want 1 (%v)", n, warns)
			}
			for _, marker := range []string{templateWarnMarker, customStepsWarnMarker} {
				if n := countContaining(warns, marker); n != 0 {
					t.Errorf("%q warnings = %d, want 0 (%v)", marker, n, warns)
				}
			}
			for _, w := range warns {
				if !strings.Contains(w, localTemplateWarnMarker) {
					continue
				}
				if !strings.Contains(w, localResourcesSentence) {
					t.Errorf("warning drops the step-resources sentence: %s", w)
				}
				if !strings.Contains(w, "requests cpu=500m, limits cpu=2 memory=2Gi") {
					t.Errorf("warning does not list the requested values: %s", w)
				}
			}
			tmpl := b.Annotations[BuildRunTemplateAnnotation]
			if tmpl == "" {
				t.Fatalf("BuildRun template annotation missing")
			}
			if got := strings.Contains(tmpl, "stepResources"); got != tt.wantStepResources {
				t.Errorf("template carries stepResources = %v, want %v:\n%s", got, tt.wantStepResources, tmpl)
			}
			if tt.wantStepResources && !strings.Contains(tmpl, "memory: 2Gi") {
				t.Errorf("template does not carry the requested values:\n%s", tmpl)
			}
		})
	}
}

// A git source with resources keeps W48 and gets no Local-source warning.
func TestConvertGitSourceWithResourcesKeepsTemplateWarning(t *testing.T) {
	_, _, warns := convertOutputSpec(t, resourcesSpec(`{"type": "Git", "git": {"uri": "https://github.com/example/app.git"}}`), PluginOptionalFields{})

	if n := countContaining(warns, templateWarnMarker); n != 1 {
		t.Errorf("template warnings = %d, want 1 (%v)", n, warns)
	}
	if n := countContaining(warns, localTemplateWarnMarker); n != 0 {
		t.Errorf("Local-source template warnings = %d, want 0 (%v)", n, warns)
	}
}

// accountSpec is resourcesSpec's binary sibling without spec.resources: a
// Parallel-runPolicy Docker BuildConfig with a binary source, so the only
// thing that puts a BuildRun template on the Build is the account.
func accountSpec(strategy, extra string) string {
	return `{
		"runPolicy": "Parallel",
		"source": {"type": "Binary", "binary": {}},
		"strategy": ` + strategy + `,` + extra + `
		"output": {"to": {"kind": "DockerImage", "name": "quay.io/example/app:latest"}, "pushSecret": {"name": "push"}}
	}`
}

// BUILD-2402 review: widening the template gate to an account alone put a
// template on binary Builds that had none before, and the Local-source guard
// sat below the no-resources return, so nothing said the template cannot start
// them. The warning now fires for an account on its own, names --sa-name, and
// says nothing about step resources, because none were set.
func TestConvertBinarySourceWithAccountAndNoResources(t *testing.T) {
	tests := []struct {
		name string
		// wantAccount is the account the upload has to name: the one the
		// BuildConfig named, or the one the pull secret made the plugin
		// generate.
		wantAccount string
		strategy    string
		extra       string
	}{
		{
			name:        "named account",
			wantAccount: "custom-builder-sa",
			strategy:    `{"type": "Docker", "dockerStrategy": {}}`,
			extra:       `"serviceAccount": "custom-builder-sa",`,
		},
		{
			name:        "generated account",
			wantAccount: "app",
			strategy:    `{"type": "Docker", "dockerStrategy": {"from": {"kind": "DockerImage", "name": "registry.example.com/builder:latest"}, "pullSecret": {"name": "my-pull-secret"}}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _, warns := convertOutputSpec(t, accountSpec(tt.strategy, tt.extra), PluginOptionalFields{})

			tmpl := b.Annotations[BuildRunTemplateAnnotation]
			if tmpl == "" {
				t.Fatalf("BuildRun template annotation missing")
			}
			if !strings.Contains(tmpl, "serviceAccount: "+tt.wantAccount) {
				t.Errorf("template does not name %s:\n%s", tt.wantAccount, tmpl)
			}

			if n := countContaining(warns, localTemplateWarnMarker); n != 1 {
				t.Fatalf("Local-source template warnings = %d, want 1 (%v)", n, warns)
			}
			for _, w := range warns {
				if !strings.Contains(w, localTemplateWarnMarker) {
					continue
				}
				if !strings.Contains(w, localAccountSentence) {
					t.Errorf("warning does not say how to pass the account on the upload: %s", w)
				}
				if !strings.Contains(w, "shp build upload app <directory> --sa-name "+tt.wantAccount) {
					t.Errorf("warning does not name %s on the upload command: %s", tt.wantAccount, w)
				}
				if strings.Contains(w, localResourcesSentence) {
					t.Errorf("warning talks about step resources for a BuildConfig that set none: %s", w)
				}
			}
			for _, marker := range []string{templateWarnMarker, customStepsWarnMarker} {
				if n := countContaining(warns, marker); n != 0 {
					t.Errorf("%q warnings = %d, want 0 (%v)", marker, n, warns)
				}
			}
		})
	}
}
