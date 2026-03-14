package chartgen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/verity-org/verity/internal/config"
)

// ImageMapping represents the mapping from an original image to its patched replacement.
type ImageMapping struct {
	OriginalRepo string `json:"originalRepo"`
	OriginalTag  string `json:"originalTag"`
	PatchedRepo  string `json:"patchedRepo"`
	PatchedTag   string `json:"patchedTag"`
}

// BuildImageMappings queries the target registry for patched versions of each image
// and returns mappings for images that have been successfully patched.
func BuildImageMappings(imageRefs []string, targetRegistry string, excludeNames map[string]struct{}, copaNames map[string]string) ([]ImageMapping, error) {
	ctx := context.Background()
	mappings := make([]ImageMapping, 0, len(imageRefs))

	for _, imageRef := range imageRefs {
		sourceRepo, sourceTag := splitRef(imageRef)
		name := resolveImageName(imageRef, copaNames)

		if isExcluded(name, imageRef, excludeNames) {
			fmt.Fprintf(os.Stderr, "warning: skipping excluded image %q (%s)\n", name, imageRef)
			continue
		}

		patchedRepo := targetRegistry + "/" + name
		lsOutput, err := runCommand(ctx, 30*time.Second, "crane", "ls", patchedRepo)
		if err != nil {
			// crane ls fails if repo doesn't exist — treat as no patched images.
			fmt.Fprintf(os.Stderr, "warning: cannot list tags for %s: %v\n", patchedRepo, err)
			continue
		}

		patchedTag := FindLatestPatchedTag(lsOutput, sourceTag)
		if patchedTag == "" {
			fmt.Fprintf(os.Stderr, "warning: no patched tag found for %s (%s)\n", imageRef, patchedRepo)
			continue
		}

		mappings = append(mappings, ImageMapping{
			OriginalRepo: sourceRepo,
			OriginalTag:  sourceTag,
			PatchedRepo:  patchedRepo,
			PatchedTag:   patchedTag,
		})
	}

	return mappings, nil
}

// BuildCopaNameMap builds a lookup map from normalized image repository path
// (without host/tag/digest) to the configured Copa image name.
func BuildCopaNameMap(images []config.ImageSpec) map[string]string {
	if len(images) == 0 {
		return nil
	}

	names := make(map[string]string, len(images))
	for i := range images {
		repo := normalizeRepo(images[i].Image)
		if repo == "" || images[i].Name == "" {
			continue
		}
		names[repo] = images[i].Name
	}

	return names
}

func resolveImageName(imageRef string, copaNames map[string]string) string {
	if len(copaNames) > 0 {
		if name, ok := copaNames[normalizeRepo(imageRef)]; ok {
			return name
		}
	}

	return nameFromRef(imageRef)
}

// FindLatestPatchedTag finds the latest patched tag from crane ls output.
// Returns "" if no patched tag matches the source tag.
func FindLatestPatchedTag(craneLsOutput, sourceTag string) string {
	if sourceTag == "" {
		return ""
	}

	base := sourceTag + "-patched"
	bestTag := ""
	bestVersion := -1

	for line := range strings.SplitSeq(craneLsOutput, "\n") {
		tag := strings.TrimSpace(line)
		if tag == "" {
			continue
		}

		if tag == base {
			if bestVersion < 0 {
				bestVersion = 0
				bestTag = tag
			}
			continue
		}

		rest, found := strings.CutPrefix(tag, base+"-")
		if !found {
			continue
		}

		n, err := strconv.Atoi(rest)
		if err != nil || n <= 0 {
			continue
		}
		if n > bestVersion {
			bestVersion = n
			bestTag = tag
		}
	}

	return bestTag
}

// isExcluded checks whether a chart-discovered image should be skipped.
// It matches the derived name (from nameFromRef) AND the raw basename of
// the source ref against the exclude set, consistent with discovery.isExcluded.
func isExcluded(name, imageRef string, excludeNames map[string]struct{}) bool {
	if len(excludeNames) == 0 {
		return false
	}
	if _, ok := excludeNames[name]; ok {
		return true
	}
	baseName := nameBasename(imageRef)
	if baseName != name {
		if _, ok := excludeNames[baseName]; ok {
			return true
		}
	}
	return false
}

// nameFromRef derives a short image name from an image reference.
// Duplicated from internal/discovery for package isolation.
func nameFromRef(ref string) string {
	if idx := strings.Index(ref, "@"); idx != -1 {
		ref = ref[:idx]
	}

	lastSlash := strings.LastIndex(ref, "/")
	if lastColon := strings.LastIndex(ref, ":"); lastColon > lastSlash {
		ref = ref[:lastColon]
	}

	parts := strings.Split(ref, "/")

	if len(parts) >= 3 {
		org := parts[len(parts)-2]
		name := parts[len(parts)-1]
		if org == name {
			return name
		}
		return org + "-" + name
	}

	return parts[len(parts)-1]
}

// nameBasename returns the last path component of an image ref with tag/digest stripped.
// e.g. "docker.io/library/rabbitmq:4.2.3" → "rabbitmq".
// Duplicated from internal/discovery for package isolation.
func nameBasename(ref string) string {
	if idx := strings.Index(ref, "@"); idx != -1 {
		ref = ref[:idx]
	}
	lastSlash := strings.LastIndex(ref, "/")
	if lastColon := strings.LastIndex(ref, ":"); lastColon > lastSlash {
		ref = ref[:lastColon]
	}
	if lastSlash >= 0 {
		return ref[lastSlash+1:]
	}
	return ref
}

func normalizeRepo(ref string) string {
	ref, _ = splitRef(ref)
	if ref == "" {
		return ""
	}

	parts := strings.Split(ref, "/")
	if len(parts) > 1 {
		first := parts[0]
		if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
			parts = parts[1:]
		}
	}

	return strings.Join(parts, "/")
}

// splitRef splits an image reference into its name and tag components.
// Digest suffixes (@sha256:...) are stripped before extracting the tag.
func splitRef(ref string) (name, tag string) {
	// Strip digest — we want the tag, not the digest hash.
	if idx := strings.Index(ref, "@"); idx != -1 {
		ref = ref[:idx]
	}
	lastSlash := strings.LastIndex(ref, "/")
	if lastColon := strings.LastIndex(ref, ":"); lastColon > lastSlash {
		return ref[:lastColon], ref[lastColon+1:]
	}
	return ref, ""
}

// runCommand executes a CLI command with a timeout and returns stdout.
func runCommand(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %s: %w\nstderr: %s", name, strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}

// ErrNilChart is returned when a nil chart is passed to PackageChart.
var ErrNilChart = errors.New("package chart: chart is nil")

// ErrEmptyChartName is returned when an empty chart name is provided.
var ErrEmptyChartName = errors.New("build wrapper chart: original chart name is required")

// ErrNoArchivePath is returned when helm package output doesn't contain the archive path.
var ErrNoArchivePath = errors.New("helm package output did not contain chart archive path")
