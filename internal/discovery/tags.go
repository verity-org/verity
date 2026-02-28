package discovery

import (
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"

	"github.com/verity-org/verity/internal/config"
)

// ErrUnknownStrategy is returned when an image spec has an unrecognized tag strategy.
var ErrUnknownStrategy = errors.New("unknown tag strategy")

// FindTagsToPatch discovers the set of tags to patch for a given image spec.
func FindTagsToPatch(spec *config.ImageSpec) ([]string, error) {
	repo, err := name.NewRepository(spec.Image)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository: %w", err)
	}

	switch spec.Tags.Strategy {
	case "list":
		result := make([]string, len(spec.Tags.List))
		copy(result, spec.Tags.List)
		return result, nil
	case "pattern":
		return findTagsByPattern(repo, spec)
	case "latest":
		return findTagsByLatest(repo, spec)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownStrategy, spec.Tags.Strategy)
	}
}

func findTagsByLatest(repo name.Repository, spec *config.ImageSpec) ([]string, error) {
	allTags, err := remote.List(repo, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, err
	}

	filteredTags := ExcludeTags(allTags, spec.Tags.Exclude)
	var versions []*semver.Version
	for _, t := range filteredTags {
		if v, err := semver.NewVersion(t); err == nil {
			versions = append(versions, v)
		}
	}

	if len(versions) == 0 {
		return []string{}, nil
	}

	sort.Sort(semver.Collection(versions))
	return []string{versions[len(versions)-1].Original()}, nil
}

func findTagsByPattern(repo name.Repository, spec *config.ImageSpec) ([]string, error) {
	allTags, err := remote.List(repo, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, err
	}

	pattern, err := regexp.Compile(spec.Tags.Pattern)
	if err != nil {
		return nil, err
	}

	var matchingTags []string
	for _, tag := range allTags {
		if pattern.MatchString(tag) {
			matchingTags = append(matchingTags, tag)
		}
	}

	matchingTags = ExcludeTags(matchingTags, spec.Tags.Exclude)
	var versions []*semver.Version
	for _, t := range matchingTags {
		if v, err := semver.NewVersion(t); err == nil {
			versions = append(versions, v)
		}
	}

	if len(versions) == 0 {
		return []string{}, nil
	}

	sort.Sort(semver.Collection(versions))

	if spec.Tags.MaxTags > 0 && len(versions) > spec.Tags.MaxTags {
		versions = versions[len(versions)-spec.Tags.MaxTags:]
	}

	result := make([]string, len(versions))
	for i, v := range versions {
		result[i] = v.Original()
	}
	return result, nil
}

// ExcludeTags returns a new slice with excluded entries removed.
func ExcludeTags(tags, exclusions []string) []string {
	if len(exclusions) == 0 {
		result := make([]string, len(tags))
		copy(result, tags)
		return result
	}
	exclusionSet := make(map[string]struct{})
	for _, ex := range exclusions {
		exclusionSet[ex] = struct{}{}
	}
	result := []string{}
	for _, tag := range tags {
		if _, found := exclusionSet[tag]; !found {
			result = append(result, tag)
		}
	}
	return result
}
