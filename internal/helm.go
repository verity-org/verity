package internal

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

type Dependency struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	Repository string `yaml:"repository"`
	Condition  string `yaml:"condition,omitempty"`
}

type ChartFile struct {
	APIVersion   string       `yaml:"apiVersion"`
	Name         string       `yaml:"name"`
	Description  string       `yaml:"description,omitempty"`
	Type         string       `yaml:"type,omitempty"`
	Version      string       `yaml:"version"`
	Dependencies []Dependency `yaml:"dependencies"`
}

func ParseChartFile(path string) (*ChartFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var cf ChartFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(cf.Dependencies) == 0 {
		return nil, fmt.Errorf("no dependencies found in %s", path)
	}
	return &cf, nil
}

func DownloadChart(dep Dependency, destDir string) (string, error) {
	settings := cli.New()
	cfg := &action.Configuration{}

	pull := action.NewPullWithOpts(action.WithConfig(cfg))
	pull.Settings = settings
	pull.Untar = true
	pull.UntarDir = destDir
	pull.DestDir = destDir
	pull.Version = dep.Version
	pull.RepoURL = dep.Repository

	_, err := pull.Run(dep.Name)
	if err != nil {
		return "", fmt.Errorf("pulling %s@%s: %w", dep.Name, dep.Version, err)
	}

	return filepath.Join(destDir, dep.Name), nil
}
