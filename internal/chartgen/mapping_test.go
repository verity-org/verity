package chartgen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/verity-org/verity/internal/config"
)

func TestFindLatestPatchedTag(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		sourceTag string
		want      string
	}{
		{
			name:      "no matching tags",
			output:    "latest\nv3.3.0\nv3.3.0-patched",
			sourceTag: "v3.2.1",
			want:      "",
		},
		{
			name:      "single patched tag",
			output:    "v3.2.1-patched",
			sourceTag: "v3.2.1",
			want:      "v3.2.1-patched",
		},
		{
			name:      "multiple versions choose highest",
			output:    "v3.2.1-patched\nv3.2.1-patched-1\nv3.2.1-patched-2",
			sourceTag: "v3.2.1",
			want:      "v3.2.1-patched-2",
		},
		{
			name:      "filter unrelated tags",
			output:    "v3.2.1\nv3.3.0-patched\nlatest\nv3.2.1-patched-1",
			sourceTag: "v3.2.1",
			want:      "v3.2.1-patched-1",
		},
		{
			name:      "empty input",
			output:    "",
			sourceTag: "v3.2.1",
			want:      "",
		},
		{
			name:      "non numeric suffix skipped",
			output:    "v3.2.1-patched-abc\nv3.2.1-patched-1",
			sourceTag: "v3.2.1",
			want:      "v3.2.1-patched-1",
		},
		{
			name:      "only base patched tag",
			output:    "v3.2.1-patched",
			sourceTag: "v3.2.1",
			want:      "v3.2.1-patched",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FindLatestPatchedTag(tc.output, tc.sourceTag)
			if got != tc.want {
				t.Errorf("FindLatestPatchedTag(%q, %q) = %q, want %q", tc.output, tc.sourceTag, got, tc.want)
			}
		})
	}
}

func TestNameFromRef(t *testing.T) {
	tests := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "org equals name deduplicated",
			ref:  "quay.io/prometheus/prometheus:v3.2.1",
			want: "prometheus",
		},
		{
			name: "org name joined",
			ref:  "ghcr.io/kiwigrid/k8s-sidecar:1.28.0",
			want: "kiwigrid-k8s-sidecar",
		},
		{
			name: "simple image",
			ref:  "nginx:1.25",
			want: "nginx",
		},
		{
			name: "collision safety",
			ref:  "quay.io/some-org/nginx:1.29",
			want: "some-org-nginx",
		},
		{
			name: "repeated component",
			ref:  "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.0",
			want: "kube-state-metrics",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := nameFromRef(tc.ref)
			if got != tc.want {
				t.Errorf("nameFromRef(%q) = %q, want %q", tc.ref, got, tc.want)
			}
		})
	}
}

func TestSplitRef(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		wantName string
		wantTag  string
	}{
		{
			name:     "name and tag",
			ref:      "quay.io/prom/prometheus:v3.2.1",
			wantName: "quay.io/prom/prometheus",
			wantTag:  "v3.2.1",
		},
		{
			name:     "name only",
			ref:      "nginx",
			wantName: "nginx",
			wantTag:  "",
		},
		{
			name:     "digest stripped",
			ref:      "quay.io/foo/bar:v1.2@sha256:abc123",
			wantName: "quay.io/foo/bar",
			wantTag:  "v1.2",
		},
		{
			name:     "digest only no tag",
			ref:      "quay.io/foo/bar@sha256:abc123",
			wantName: "quay.io/foo/bar",
			wantTag:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotTag := splitRef(tc.ref)
			if gotName != tc.wantName || gotTag != tc.wantTag {
				t.Errorf("splitRef(%q) = (%q, %q), want (%q, %q)", tc.ref, gotName, gotTag, tc.wantName, tc.wantTag)
			}
		})
	}
}

func TestBuildImageMappings(t *testing.T) {
	tmpDir := t.TempDir()
	fakeCrane := filepath.Join(tmpDir, "crane")
	script := `#!/bin/sh
if [ "$1" != "ls" ]; then
  exit 1
fi

case "$2" in
  "ghcr.io/verity-org/prometheus")
    printf "v3.2.1\nv3.2.1-patched\nv3.2.1-patched-2\n"
    ;;
  "ghcr.io/verity-org/kiwigrid-k8s-sidecar")
    printf "1.28.0\n1.28.0-patched\n"
    ;;
  "ghcr.io/verity-org/no-patch")
    printf "latest\n1.0.0\n"
    ;;
  *)
    printf "\n"
    ;;
esac
`
	if err := os.WriteFile(fakeCrane, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile(fake crane) error = %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+origPath)

	imageRefs := []string{
		"quay.io/prometheus/prometheus:v3.2.1",
		"ghcr.io/kiwigrid/k8s-sidecar:1.28.0",
		"docker.io/library/no-patch:1.0.0",
	}

	excludeNames := map[string]struct{}{
		"kiwigrid-k8s-sidecar": {},
	}

	got, err := BuildImageMappings(imageRefs, "ghcr.io/verity-org", excludeNames, nil)
	if err != nil {
		t.Fatalf("BuildImageMappings() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("BuildImageMappings() returned %d mappings, want 1", len(got))
	}

	want := ImageMapping{
		OriginalRepo: "quay.io/prometheus/prometheus",
		OriginalTag:  "v3.2.1",
		PatchedRepo:  "ghcr.io/verity-org/prometheus",
		PatchedTag:   "v3.2.1-patched-2",
	}

	if got[0] != want {
		t.Errorf("BuildImageMappings()[0] = %+v, want %+v", got[0], want)
	}
}

func TestBuildCopaNameMap(t *testing.T) {
	images := []config.ImageSpec{
		{Name: "k8s-sidecar", Image: "ghcr.io/kiwigrid/k8s-sidecar"},
		{Name: "redis", Image: "mirror.gcr.io/library/redis"},
		{Name: "prometheus-node-exporter", Image: "quay.io/prometheus/node-exporter"},
		{Name: "kube-state-metrics", Image: "ghcr.io/kubernetes/kube-state-metrics/kube-state-metrics"},
	}
	m := BuildCopaNameMap(images)

	tests := []struct {
		repo string
		want string
	}{
		{repo: "kiwigrid/k8s-sidecar", want: "k8s-sidecar"},
		{repo: "library/redis", want: "redis"},
		{repo: "prometheus/node-exporter", want: "prometheus-node-exporter"},
		{repo: "kubernetes/kube-state-metrics/kube-state-metrics", want: "kube-state-metrics"},
	}

	for _, tt := range tests {
		if got := m[tt.repo]; got != tt.want {
			t.Errorf("copaNames[%q] = %q, want %q", tt.repo, got, tt.want)
		}
	}
}

func TestResolveImageName(t *testing.T) {
	copaNames := map[string]string{
		"kiwigrid/k8s-sidecar": "k8s-sidecar",
		"library/redis":        "redis",
		"kubernetes/kube-state-metrics/kube-state-metrics": "kube-state-metrics",
	}

	tests := []struct {
		ref  string
		want string
	}{
		{ref: "ghcr.io/kiwigrid/k8s-sidecar:1.28.0", want: "k8s-sidecar"},
		{ref: "mirror.gcr.io/library/redis:7.0.5", want: "redis"},
		{ref: "ghcr.io/kubernetes/kube-state-metrics/kube-state-metrics:v2.10.0", want: "kube-state-metrics"},
		{ref: "quay.io/some-unknown/thing:1.0", want: "some-unknown-thing"},
		{ref: "nginx:1.25", want: "nginx"},
	}

	for _, tt := range tests {
		got := resolveImageName(tt.ref, copaNames)
		if got != tt.want {
			t.Errorf("resolveImageName(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestIsExcluded(t *testing.T) {
	exclude := map[string]struct{}{"rabbitmq": {}, "kiwigrid-k8s-sidecar": {}}

	tests := []struct {
		name     string
		imgName  string
		imageRef string
		want     bool
	}{
		{
			name:     "exact nameFromRef match",
			imgName:  "kiwigrid-k8s-sidecar",
			imageRef: "ghcr.io/kiwigrid/k8s-sidecar:1.28.0",
			want:     true,
		},
		{
			name:     "basename fallback match",
			imgName:  "library-rabbitmq",
			imageRef: "docker.io/library/rabbitmq:4.2.3",
			want:     true,
		},
		{
			name:     "no match",
			imgName:  "prometheus",
			imageRef: "quay.io/prometheus/prometheus:v3",
			want:     false,
		},
		{
			name:     "nil exclude set",
			imgName:  "rabbitmq",
			imageRef: "docker.io/library/rabbitmq:4.2.3",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exc := exclude
			if tc.name == "nil exclude set" {
				exc = nil
			}
			got := isExcluded(tc.imgName, tc.imageRef, exc)
			if got != tc.want {
				t.Errorf("isExcluded(%q, %q) = %v, want %v", tc.imgName, tc.imageRef, got, tc.want)
			}
		})
	}
}

func TestFindLatestPatchedTagEmptySource(t *testing.T) {
	got := FindLatestPatchedTag("v1-patched\nv2-patched", "")
	if got != "" {
		t.Errorf("FindLatestPatchedTag with empty sourceTag = %q, want empty", got)
	}
}
