package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/descope/verity/internal"
	"gopkg.in/yaml.v3"
)

func main() {
	chartFile := flag.String("chart", "Chart.yaml", "path to Chart.yaml")
	outputDir := flag.String("output", "charts", "output directory for per-chart image values")
	flag.Parse()

	chart, err := internal.ParseChartFile(*chartFile)
	if err != nil {
		log.Fatalf("Failed to parse %s: %v", *chartFile, err)
	}

	tmpDir, err := os.MkdirTemp("", "verity-")
	if err != nil {
		log.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, dep := range chart.Dependencies {
		fmt.Printf("Processing %s@%s\n", dep.Name, dep.Version)

		chartPath, err := internal.DownloadChart(dep, tmpDir)
		if err != nil {
			log.Fatalf("Failed to download %s: %v", dep.Name, err)
		}

		images, err := internal.ScanForImages(chartPath)
		if err != nil {
			log.Fatalf("Failed to scan %s: %v", dep.Name, err)
		}

		outDir := filepath.Join(*outputDir, dep.Name)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			log.Fatalf("Failed to create %s: %v", outDir, err)
		}

		outFile := filepath.Join(outDir, "values.yaml")
		if err := writeImagesFile(outFile, images); err != nil {
			log.Fatalf("Failed to write %s: %v", outFile, err)
		}

		fmt.Printf("  Found %d images → %s\n", len(images), outFile)
		for _, img := range images {
			fmt.Printf("    %s  (%s)\n", img.Reference(), img.Path)
		}
	}
}

type imagesFile struct {
	Images []internal.Image `yaml:"images"`
}

func writeImagesFile(path string, images []internal.Image) error {
	data, err := yaml.Marshal(&imagesFile{Images: images})
	if err != nil {
		return fmt.Errorf("marshaling images: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}
