package chartgen

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/verity-org/verity/internal/config"
)

type ValueOverride struct {
	Path       string `json:"path"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

type repoTagPair struct {
	Path   string
	Repo   string
	HasTag bool
}

func ResolveValuePaths(valuesYAML []byte, mappings []ImageMapping, overrides map[string]config.Override) ([]ValueOverride, error) {
	values := make(map[string]any)
	if err := yaml.Unmarshal(valuesYAML, &values); err != nil {
		return nil, fmt.Errorf("unmarshal values YAML: %w", err)
	}

	result := make([]ValueOverride, 0, len(mappings))
	matched := make([]bool, len(mappings))

	overrideKeys := make([]string, 0, len(overrides))
	for key := range overrides {
		overrideKeys = append(overrideKeys, key)
	}
	sort.Strings(overrideKeys)

	for i, m := range mappings {
		for _, key := range overrideKeys {
			override := overrides[key]
			if override.ValuePath == "" {
				continue
			}
			if matchesRepo(m.OriginalRepo, key) {
				result = append(result, ValueOverride{
					Path:       override.ValuePath,
					Repository: m.PatchedRepo,
					Tag:        m.PatchedTag,
				})
				matched[i] = true
				break
			}
		}
	}

	var pairs []repoTagPair
	walkValues("", values, &pairs)
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].Path < pairs[j].Path
	})

	for _, pair := range pairs {
		if !pair.HasTag {
			continue
		}
		for i, m := range mappings {
			if matched[i] {
				continue
			}
			if matchesRepo(pair.Repo, m.OriginalRepo) {
				result = append(result, ValueOverride{
					Path:       pair.Path,
					Repository: m.PatchedRepo,
					Tag:        m.PatchedTag,
				})
				matched[i] = true
				break
			}
		}
	}

	return result, nil
}

func GetChartValues(chart config.ChartSpec) ([]byte, error) {
	args := []string{"show", "values"}
	if strings.HasPrefix(chart.Repository, "oci://") {
		args = append(args, chart.Repository+"/"+chart.Name)
	} else {
		args = append(args, chart.Name, "--repo", chart.Repository)
	}
	args = append(args, "--version", chart.Version)

	out, err := runHelm(context.Background(), args...)
	if err != nil {
		return nil, fmt.Errorf("get chart values for %s: %w", chart.Name, err)
	}

	return []byte(out), nil
}

func walkValues(prefix string, node map[string]any, pairs *[]repoTagPair) {
	for key, val := range node {
		child, ok := val.(map[string]any)
		if !ok {
			continue
		}

		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		repo, hasRepo := child["repository"].(string)
		if hasRepo && repo != "" {
			_, hasTag := child["tag"].(string)
			*pairs = append(*pairs, repoTagPair{Path: path, Repo: repo, HasTag: hasTag})
		}

		walkValues(path, child, pairs)
	}
}

func matchesRepo(imageRepo, candidate string) bool {
	if imageRepo == candidate {
		return true
	}
	if strings.HasSuffix(imageRepo, "/"+candidate) {
		return true
	}
	if strings.HasSuffix(candidate, "/"+imageRepo) {
		return true
	}
	return false
}

func runHelm(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "helm", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("helm %s: %w\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}
