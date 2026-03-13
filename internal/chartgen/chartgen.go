package chartgen

import (
	"fmt"
	"os"

	"github.com/verity-org/verity/internal/discovery"
)

type Config struct {
	ChartsFile     string
	VerityConfig   string
	TargetRegistry string
	ChartRegistry  string
	ExcludeNames   map[string]struct{}
	DryRun         bool
}

type ChartResult struct {
	Name           string          `json:"name"`
	Version        string          `json:"version"`
	WrapperName    string          `json:"wrapperName"`
	WrapperVersion string          `json:"wrapperVersion"`
	Registry       string          `json:"registry"`
	ImageMappings  []ImageMapping  `json:"imageMappings"`
	ValueOverrides []ValueOverride `json:"valueOverrides"`
}

type DryRunResult struct {
	Charts []ChartResult `json:"charts"`
}

func Run(cfg Config) (*DryRunResult, error) {
	charts, err := discovery.LoadChartsFile(cfg.ChartsFile)
	if err != nil {
		return nil, fmt.Errorf("load charts file: %w", err)
	}

	vc, err := discovery.LoadVerityConfig(cfg.VerityConfig)
	if err != nil {
		return nil, fmt.Errorf("load verity config: %w", err)
	}

	result := &DryRunResult{Charts: make([]ChartResult, 0, len(charts))}

	for _, chart := range charts {
		fmt.Fprintf(os.Stderr, "info: processing chart %s@%s\n", chart.Name, chart.Version)

		imageRefs, err := discovery.ExtractChartImages(chart, vc.Overrides)
		if err != nil {
			return nil, fmt.Errorf("extract images for chart %s: %w", chart.Name, err)
		}

		mappings, err := BuildImageMappings(imageRefs, cfg.TargetRegistry, cfg.ExcludeNames)
		if err != nil {
			return nil, fmt.Errorf("build image mappings for chart %s: %w", chart.Name, err)
		}

		if len(mappings) == 0 {
			fmt.Fprintf(os.Stderr, "warning: no patched image mappings for chart %s@%s; skipping\n", chart.Name, chart.Version)
			continue
		}

		valuesYAML, err := GetChartValues(chart)
		if err != nil {
			return nil, fmt.Errorf("get chart values for %s: %w", chart.Name, err)
		}

		valueOverrides, err := ResolveValuePaths(valuesYAML, mappings, vc.Overrides)
		if err != nil {
			return nil, fmt.Errorf("resolve value paths for %s: %w", chart.Name, err)
		}

		wrapper, err := BuildWrapperChart(chart, valueOverrides)
		if err != nil {
			return nil, fmt.Errorf("build wrapper chart for %s: %w", chart.Name, err)
		}

		chartResult := ChartResult{
			Name:           chart.Name,
			Version:        chart.Version,
			WrapperName:    wrapper.Name,
			WrapperVersion: wrapper.Version,
			Registry:       cfg.ChartRegistry,
			ImageMappings:  mappings,
			ValueOverrides: valueOverrides,
		}
		result.Charts = append(result.Charts, chartResult)

		if cfg.DryRun {
			fmt.Fprintf(os.Stderr, "info: dry-run enabled; skipping package/push for %s\n", wrapper.Name)
			continue
		}

		tgzPath, err := PackageChart(wrapper)
		if err != nil {
			return nil, fmt.Errorf("package wrapper chart %s: %w", wrapper.Name, err)
		}

		if err := PushChart(tgzPath, cfg.ChartRegistry); err != nil {
			_ = os.Remove(tgzPath)
			return nil, fmt.Errorf("push wrapper chart %s: %w", wrapper.Name, err)
		}

		if err := os.Remove(tgzPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove packaged chart %s: %v\n", tgzPath, err)
		}
	}

	return result, nil
}
