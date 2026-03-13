package chartgen

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestDryRunResultJSON(t *testing.T) {
	res := DryRunResult{
		Charts: []ChartResult{
			{
				Name:           "prometheus",
				Version:        "28.9.1",
				WrapperName:    "prometheus-patched",
				WrapperVersion: "28.9.1-patched.1",
				Registry:       "oci://ghcr.io/verity-org/charts",
				ImageMappings: []ImageMapping{
					{
						OriginalRepo: "quay.io/prometheus/prometheus",
						OriginalTag:  "v3.2.1",
						PatchedRepo:  "ghcr.io/verity-org/prometheus/prometheus",
						PatchedTag:   "v3.2.1-patched",
					},
				},
				ValueOverrides: []ValueOverride{
					{
						Path:       "server.image",
						Repository: "ghcr.io/verity-org/prometheus/prometheus",
						Tag:        "v3.2.1-patched",
					},
				},
			},
		},
	}

	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	charts, ok := doc["charts"].([]any)
	if !ok {
		t.Fatalf("charts type = %T, want []any", doc["charts"])
	}
	if len(charts) != 1 {
		t.Fatalf("charts length = %d, want 1", len(charts))
	}

	chart, ok := charts[0].(map[string]any)
	if !ok {
		t.Fatalf("charts[0] type = %T, want map[string]any", charts[0])
	}

	keys := []string{"name", "version", "wrapperName", "wrapperVersion", "registry", "imageMappings", "valueOverrides"}
	for _, key := range keys {
		if _, exists := chart[key]; !exists {
			t.Fatalf("chart missing key %q in JSON: %#v", key, chart)
		}
	}
}

func TestRunDryRunNoCharts(t *testing.T) {
	tmpDir := t.TempDir()

	chartsPath := filepath.Join(tmpDir, "does-not-exist-chart.yaml")
	verityPath := filepath.Join(tmpDir, "does-not-exist-verity.yaml")

	res, err := Run(&Config{
		ChartsFile:     chartsPath,
		VerityConfig:   verityPath,
		TargetRegistry: "ghcr.io/verity-org",
		ChartRegistry:  "oci://ghcr.io/verity-org/charts",
		ExcludeNames:   map[string]struct{}{},
		DryRun:         true,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res == nil {
		t.Fatal("Run() result = nil, want non-nil")
	}
	if len(res.Charts) != 0 {
		t.Fatalf("Run() charts length = %d, want 0", len(res.Charts))
	}
}
