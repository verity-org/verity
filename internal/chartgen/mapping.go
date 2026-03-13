package chartgen

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ImageMapping struct {
	OriginalRepo string `json:"originalRepo"`
	OriginalTag  string `json:"originalTag"`
	PatchedRepo  string `json:"patchedRepo"`
	PatchedTag   string `json:"patchedTag"`
}

func BuildImageMappings(imageRefs []string, targetRegistry string, excludeNames map[string]struct{}) ([]ImageMapping, error) {
	ctx := context.Background()
	mappings := make([]ImageMapping, 0, len(imageRefs))

	for _, imageRef := range imageRefs {
		sourceRepo, sourceTag := splitRef(imageRef)
		name := nameFromRef(imageRef)

		if _, excluded := excludeNames[name]; excluded {
			fmt.Fprintf(os.Stderr, "warning: skipping excluded image %q (%s)\n", name, imageRef)
			continue
		}

		patchedRepo := targetRegistry + "/" + name
		lsOutput, err := runCrane(ctx, "ls", patchedRepo)
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

func FindLatestPatchedTag(craneLsOutput string, sourceTag string) string {
	base := sourceTag + "-patched"
	bestTag := ""
	bestVersion := -1

	for _, line := range strings.Split(craneLsOutput, "\n") {
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

		prefix := base + "-"
		if !strings.HasPrefix(tag, prefix) {
			continue
		}

		n, err := strconv.Atoi(strings.TrimPrefix(tag, prefix))
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

func splitRef(ref string) (name, tag string) {
	lastSlash := strings.LastIndex(ref, "/")
	if lastColon := strings.LastIndex(ref, ":"); lastColon > lastSlash {
		return ref[:lastColon], ref[lastColon+1:]
	}
	return ref, ""
}

func runCrane(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "crane", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("crane %s: %w\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}

	return stdout.String(), nil
}
