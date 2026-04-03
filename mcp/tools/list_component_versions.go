package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"ocm.software/open-component-model/bindings/go/oci/compref"
)

// ListComponentVersions lists available versions for a component.
func ListComponentVersions(ctx context.Context, deps Deps, input json.RawMessage) (string, error) {
	var args struct {
		Reference  string `json:"reference"`
		Constraint string `json:"constraint"`
		LatestOnly bool   `json:"latest_only"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Reference == "" {
		return "", fmt.Errorf("reference is required")
	}

	ref, err := compref.Parse(args.Reference, compref.IgnoreSemverCompatibility())
	if err != nil {
		return "", fmt.Errorf("parsing reference %q: %w", args.Reference, err)
	}

	resolver, err := newResolver(ctx, deps, ref)
	if err != nil {
		return "", fmt.Errorf("creating resolver: %w", err)
	}

	repo, err := resolver.GetComponentVersionRepositoryForComponent(ctx, ref.Component, ref.Version)
	if err != nil {
		return "", fmt.Errorf("accessing repository: %w", err)
	}

	versions, err := repo.ListComponentVersions(ctx, ref.Component)
	if err != nil {
		return "", fmt.Errorf("listing versions: %w", err)
	}

	if args.Constraint != "" {
		versions, err = filterBySemver(versions, args.Constraint)
		if err != nil {
			return "", fmt.Errorf("applying semver constraint %q: %w", args.Constraint, err)
		}
	}

	if args.LatestOnly && len(versions) > 1 {
		latest := findLatest(versions)
		versions = []string{latest}
	}

	type versionEntry struct {
		Component string `json:"component"`
		Version   string `json:"version"`
	}
	entries := make([]versionEntry, len(versions))
	for i, v := range versions {
		entries[i] = versionEntry{Component: ref.Component, Version: v}
	}

	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling result: %w", err)
	}
	return string(out), nil
}

func filterBySemver(versions []string, constraint string) ([]string, error) {
	c, err := semver.NewConstraint(constraint)
	if err != nil {
		return nil, err
	}
	var filtered []string
	for _, v := range versions {
		sv, err := semver.NewVersion(v)
		if err != nil {
			continue
		}
		if c.Check(sv) {
			filtered = append(filtered, v)
		}
	}
	return filtered, nil
}

func findLatest(versions []string) string {
	var latest *semver.Version
	latestStr := versions[0]
	for _, v := range versions {
		sv, err := semver.NewVersion(v)
		if err != nil {
			continue
		}
		if latest == nil || sv.GreaterThan(latest) {
			latest = sv
			latestStr = v
		}
	}
	return latestStr
}
