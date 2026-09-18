//go:build !documentation

package buildconfig

import (
	"strings"
	"testing"
)

const (
	dockerEnvWarnMarker = "sets dockerStrategy.env"
	sourceEnvWarnMarker = "sets sourceStrategy.env"
)

// strategyEnvSpec is a Parallel-runPolicy BuildConfig with a DockerImage output
// and a push secret, so the only strategy-dependent warnings are the env ones.
func strategyEnvSpec(strategy, source string) string {
	return `{
		"runPolicy": "Parallel",
		"source": ` + source + `,
		"strategy": ` + strategy + `,
		"output": {"to": {"kind": "DockerImage", "name": "quay.io/example/app:latest"}, "pushSecret": {"name": "push"}}
	}`
}

const gitSource = `{"type": "Git", "git": {"uri": "https://github.com/example/app.git"}}`

// BUILD-2476: spec.env keeps every entry, and one warning per strategy names
// each entry without its value.
func TestConvertStrategyEnvWarns(t *testing.T) {
	env := `[
		{"name": "ARTIFACT_URL", "value": "https://example.com/a.jar"},
		{"name": "TOKEN", "valueFrom": {"secretKeyRef": {"name": "creds", "key": "token"}}}
	]`
	tests := []struct {
		name     string
		strategy string
		marker   string
		other    string
	}{
		{"docker", `{"type": "Docker", "dockerStrategy": {"env": ` + env + `}}`, dockerEnvWarnMarker, sourceEnvWarnMarker},
		{"source", `{"type": "Source", "sourceStrategy": {"from": {"kind": "DockerImage", "name": "registry.example.com/builder:1"}, "env": ` + env + `}}`, sourceEnvWarnMarker, dockerEnvWarnMarker},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, outcome, warns := convertOutputSpec(t, strategyEnvSpec(tt.strategy, gitSource), PluginOptionalFields{})

			if n := countContaining(warns, tt.marker); n != 1 {
				t.Fatalf("%q warnings = %d, want 1 (%v)", tt.marker, n, warns)
			}
			if n := countContaining(warns, tt.other); n != 0 {
				t.Errorf("%q warnings = %d, want 0 (%v)", tt.other, n, warns)
			}
			for _, w := range warns {
				if !strings.Contains(w, tt.marker) {
					continue
				}
				if !strings.Contains(w, "ARTIFACT_URL, TOKEN") {
					t.Errorf("warning does not name both entries: %s", w)
				}
				if strings.Contains(w, "example.com/a.jar") || strings.Contains(w, "creds") {
					t.Errorf("warning leaks an entry's value or source: %s", w)
				}
			}
			if outcome.State != OutcomeConvertedWithWarnings {
				t.Errorf("outcome = %s, want %s", outcome.State, OutcomeConvertedWithWarnings)
			}
			if len(b.Spec.Env) != 2 || b.Spec.Env[0].Name != "ARTIFACT_URL" || b.Spec.Env[1].Name != "TOKEN" {
				t.Errorf("spec.env = %+v, want both entries in order", b.Spec.Env)
			}
		})
	}
}

// Neither warning fires without strategy env, including when git proxy
// settings put HTTP_PROXY and friends into spec.env.
func TestConvertStrategyEnvNoWarning(t *testing.T) {
	proxySource := `{"type": "Git", "git": {"uri": "https://github.com/example/app.git", "httpProxy": "http://proxy:3128"}}`
	tests := []struct {
		name     string
		strategy string
		source   string
	}{
		{"docker without env", `{"type": "Docker", "dockerStrategy": {}}`, gitSource},
		{"docker with empty env", `{"type": "Docker", "dockerStrategy": {"env": []}}`, gitSource},
		{"source without env", `{"type": "Source", "sourceStrategy": {"from": {"kind": "DockerImage", "name": "registry.example.com/builder:1"}}}`, gitSource},
		{"docker with git proxy only", `{"type": "Docker", "dockerStrategy": {}}`, proxySource},
		{"source with git proxy only", `{"type": "Source", "sourceStrategy": {"from": {"kind": "DockerImage", "name": "registry.example.com/builder:1"}}}`, proxySource},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, warns := convertOutputSpec(t, strategyEnvSpec(tt.strategy, tt.source), PluginOptionalFields{})
			for _, marker := range []string{dockerEnvWarnMarker, sourceEnvWarnMarker} {
				if n := countContaining(warns, marker); n != 0 {
					t.Errorf("%q warnings = %d, want 0 (%v)", marker, n, warns)
				}
			}
		})
	}
}
