package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/verity-org/verity/internal/discovery"
	"github.com/verity-org/verity/internal/preflight"
)

var errMissingGithubRepo = errors.New("--github-repo is required when --preflight is enabled")

// DiscoverCommand enumerates all image+tag combos from three sources:
//   - copa-config.yaml  — standalone images (Copa's domain)
//   - Chart.yaml        — Helm chart dependencies (standard Helm format)
//   - verity.yaml       — tag variant overrides (verity-specific)
var DiscoverCommand = &cli.Command{
	Name:  "discover",
	Usage: "Enumerate all image+tag combos from copa-config.yaml, Chart.yaml, and verity.yaml overrides",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "config",
			Aliases:  []string{"c"},
			Usage:    "Path to copa-config.yaml (standalone images)",
			Required: true,
		},
		&cli.StringFlag{
			Name:  "target-registry",
			Usage: "Override the target registry from config (e.g., ghcr.io/verity-org)",
		},
		&cli.StringFlag{
			Name:  "charts-file",
			Usage: "Helm Chart.yaml whose dependencies: provides chart images",
			Value: "Chart.yaml",
		},
		&cli.StringFlag{
			Name:  "verity-config",
			Usage: "Path to verity.yaml (tag variant overrides)",
			Value: "verity.yaml",
		},
		&cli.StringFlag{
			Name:  "only",
			Usage: "Comma-separated list of image names to include (empty = all)",
		},
		&cli.BoolFlag{
			Name:  "preflight",
			Usage: "Enable preflight digest-based skip logic (compares upstream digests with manifest)",
		},
		&cli.StringFlag{
			Name:  "github-repo",
			Usage: "GitHub repository (owner/repo) for preflight manifest lookup",
		},
		&cli.StringFlag{
			Name:  "reports-branch",
			Usage: "Branch where preflight-manifest.json is stored",
			Value: "reports",
		},
	},
	Action: func(c *cli.Context) error {
		cfg, err := discovery.LoadConfig(c.String("config"))
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		charts, err := discovery.LoadChartsFile(c.String("charts-file"))
		if err != nil {
			return fmt.Errorf("failed to load charts file: %w", err)
		}
		cfg.Charts = append(cfg.Charts, charts...)

		vc, err := discovery.LoadVerityConfig(c.String("verity-config"))
		if err != nil {
			return fmt.Errorf("failed to load verity config: %w", err)
		}

		// Merge overrides: verity.yaml takes precedence over copa-config.yaml.
		overrides := maps.Clone(cfg.Overrides)
		if overrides == nil {
			overrides = maps.Clone(vc.Overrides)
		} else {
			maps.Copy(overrides, vc.Overrides)
		}
		images, err := discovery.Discover(cfg, c.String("target-registry"), overrides)
		if err != nil {
			return fmt.Errorf("failed to discover images: %w", err)
		}

		// --only: filter to specific image names
		if only := c.String("only"); only != "" {
			images = filterCopaImagesByName(images, only)
			fmt.Fprintf(os.Stderr, "Filtered to %d images matching --only=%s\n", len(images), only)
		}

		// --preflight: skip images that don't need work
		if c.Bool("preflight") {
			repo := c.String("github-repo")
			if repo == "" {
				return errMissingGithubRepo
			}
			branch := c.String("reports-branch")
			token := os.Getenv("GH_TOKEN")
			if token == "" {
				token = os.Getenv("GITHUB_TOKEN")
			}
			images, err = preflight.FilterCopaImages(images, repo, branch, token)
			if err != nil {
				return fmt.Errorf("preflight filtering failed: %w", err)
			}
			fmt.Fprintf(os.Stderr, "Preflight: %d images need work\n", len(images))
		}

		out, err := json.MarshalIndent(images, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}

		fmt.Fprintln(os.Stdout, string(out))
		return nil
	},
}

// filterCopaImagesByName filters images to only those whose Name matches one
// of the comma-separated names.
func filterCopaImagesByName(images []discovery.DiscoveredImage, names string) []discovery.DiscoveredImage {
	allowed := make(map[string]struct{})
	for n := range strings.SplitSeq(names, ",") {
		n = strings.TrimSpace(n)
		if n != "" {
			allowed[n] = struct{}{}
		}
	}

	var filtered []discovery.DiscoveredImage
	for _, img := range images {
		if _, ok := allowed[img.Name]; ok {
			filtered = append(filtered, img)
		}
	}
	return filtered
}
