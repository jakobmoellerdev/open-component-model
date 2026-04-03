package tools

import (
	"context"
	"encoding/json"
	"fmt"

	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	descriptorv2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
)

// ResourceInfo is the metadata-only view of a resource returned by get_resource_info.
type ResourceInfo struct {
	Name       string      `json:"name"`
	Version    string      `json:"version,omitempty"`
	Type       string      `json:"type"`
	Relation   string      `json:"relation"`
	AccessType string      `json:"access_type,omitempty"`
	Digest     *DigestInfo `json:"digest,omitempty"`
	Labels     []LabelInfo `json:"labels,omitempty"`
}

// DigestInfo is a summary of a resource's digest.
type DigestInfo struct {
	HashAlgorithm string `json:"hash_algorithm"`
	Value         string `json:"value"`
}

// LabelInfo summarizes a label.
type LabelInfo struct {
	Name    string `json:"name"`
	Signing bool   `json:"signing,omitempty"`
}

// GetResourceInfo returns metadata about resources in a component version without downloading content.
func GetResourceInfo(ctx context.Context, deps Deps, input json.RawMessage) (string, error) {
	var args struct {
		Reference    string `json:"reference"`
		ResourceName string `json:"resource_name"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Reference == "" {
		return "", fmt.Errorf("reference is required")
	}

	desc, err := fetchDescriptor(ctx, deps, args.Reference)
	if err != nil {
		return "", err
	}

	v2desc, err := descruntime.ConvertToV2(descriptorv2.Scheme, desc)
	if err != nil {
		return "", fmt.Errorf("converting descriptor: %w", err)
	}

	var infos []ResourceInfo
	for _, res := range v2desc.Component.Resources {
		if args.ResourceName != "" && res.Name != args.ResourceName {
			continue
		}
		info := ResourceInfo{
			Name:     res.Name,
			Version:  res.Version,
			Type:     res.Type,
			Relation: string(res.Relation),
		}
		if res.Access != nil {
			info.AccessType = res.Access.GetType().String()
		}
		if res.Digest != nil {
			info.Digest = &DigestInfo{
				HashAlgorithm: res.Digest.HashAlgorithm,
				Value:         res.Digest.Value,
			}
		}
		for _, l := range res.Labels {
			info.Labels = append(info.Labels, LabelInfo{Name: l.Name, Signing: l.Signing})
		}
		infos = append(infos, info)
	}

	if len(infos) == 0 {
		if args.ResourceName != "" {
			return fmt.Sprintf("no resource named %q found in %s", args.ResourceName, args.Reference), nil
		}
		return "no resources found in component version", nil
	}

	out, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling result: %w", err)
	}
	return string(out), nil
}
