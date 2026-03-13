package chartgen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/verity-org/verity/internal/config"
)

type WrapperChart struct {
	Name       string
	Version    string
	ChartYAML  []byte
	ValuesYAML []byte
}

func BuildWrapperChart(original config.ChartSpec, overrides []ValueOverride) (*WrapperChart, error) {
	if original.Name == "" {
		return nil, fmt.Errorf("build wrapper chart: original chart name is required")
	}

	wrapperName := original.Name + "-patched"
	wrapperVersion := original.Version + "-patched.1"

	chartDoc := map[string]any{
		"apiVersion":  "v2",
		"name":        wrapperName,
		"description": fmt.Sprintf("Verity-patched wrapper for %s %s", original.Name, original.Version),
		"type":        "application",
		"version":     wrapperVersion,
		"dependencies": []map[string]string{{
			"name":       original.Name,
			"version":    original.Version,
			"repository": original.Repository,
		}},
		"annotations": map[string]string{
			"verity.supply/source-chart":      original.Name,
			"verity.supply/source-version":    original.Version,
			"verity.supply/source-repository": original.Repository,
		},
	}

	chartYAML, err := yaml.Marshal(chartDoc)
	if err != nil {
		return nil, fmt.Errorf("marshal Chart.yaml: %w", err)
	}

	valuesTree := buildValuesTree(original.Name, overrides)
	valuesYAML, err := yaml.Marshal(valuesTree)
	if err != nil {
		return nil, fmt.Errorf("marshal values.yaml: %w", err)
	}

	return &WrapperChart{
		Name:       wrapperName,
		Version:    wrapperVersion,
		ChartYAML:  chartYAML,
		ValuesYAML: valuesYAML,
	}, nil
}

func PackageChart(chart *WrapperChart) (string, error) {
	if chart == nil {
		return "", fmt.Errorf("package chart: chart is nil")
	}

	tmpDir, err := os.MkdirTemp("", "verity-wrapper-chart-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	chartPath := filepath.Join(tmpDir, "Chart.yaml")
	valuesPath := filepath.Join(tmpDir, "values.yaml")

	if err := os.WriteFile(chartPath, chart.ChartYAML, 0o644); err != nil {
		return "", fmt.Errorf("write Chart.yaml: %w", err)
	}
	if err := os.WriteFile(valuesPath, chart.ValuesYAML, 0o644); err != nil {
		return "", fmt.Errorf("write values.yaml: %w", err)
	}

	out, err := runHelm(context.Background(), "package", tmpDir)
	if err != nil {
		return "", fmt.Errorf("helm package: %w", err)
	}

	const prefix = "Successfully packaged chart and saved it to:"
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			path := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if strings.HasSuffix(path, ".tgz") {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("helm package output did not contain chart archive path")
}

func PushChart(tgzPath string, registry string) error {
	if _, err := runHelm(context.Background(), "push", tgzPath, registry); err != nil {
		return fmt.Errorf("helm push %s to %s: %w", tgzPath, registry, err)
	}
	return nil
}

func buildValuesTree(chartName string, overrides []ValueOverride) map[string]any {
	if len(overrides) == 0 {
		return map[string]any{}
	}

	root := make(map[string]any)
	chartRoot := make(map[string]any)
	root[chartName] = chartRoot

	for _, override := range overrides {
		parts := strings.Split(override.Path, ".")
		current := chartRoot

		for i, part := range parts {
			if part == "" {
				continue
			}

			if i == len(parts)-1 {
				current[part] = map[string]any{
					"repository": override.Repository,
					"tag":        override.Tag,
				}
				break
			}

			next, ok := current[part]
			if !ok {
				nextMap := make(map[string]any)
				current[part] = nextMap
				current = nextMap
				continue
			}

			nextMap, ok := next.(map[string]any)
			if !ok {
				nextMap = make(map[string]any)
				current[part] = nextMap
			}
			current = nextMap
		}
	}

	return root
}
