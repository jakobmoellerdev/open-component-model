// Package tools contains the MCP tool definitions and handlers for OCM operations.
package tools

import (
	"context"
	"encoding/json"

	genericv1 "ocm.software/open-component-model/bindings/go/configuration/generic/v1/spec"
	"ocm.software/open-component-model/bindings/go/credentials"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
)

// Deps holds the OCM dependencies shared by all tool handlers.
type Deps struct {
	PluginManager   *manager.PluginManager
	CredentialGraph credentials.Resolver
	Config          *genericv1.Config
}

// Handler is the function signature for an MCP tool handler.
// It receives the shared OCM deps and the raw JSON arguments from the MCP client.
// It returns a text string to send back to the AI, or an error.
type Handler func(ctx context.Context, deps Deps, input json.RawMessage) (string, error)

// Registry maps tool names to their handlers.
var Registry = map[string]Handler{
	"get_component_version":   GetComponentVersion,
	"list_component_versions": ListComponentVersions,
	"get_resource_info":       GetResourceInfo,
}

// ToolDef is the MCP tool definition returned in tools/list responses.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Definitions returns the MCP tool definitions for all registered tools.
func Definitions() []ToolDef {
	return []ToolDef{
		{
			Name:        "get_component_version",
			Description: "Fetch the complete component descriptor for a specific OCM component version. Returns all metadata including resources, sources, references, signatures, labels, and repository contexts. Use this to examine a specific component version.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"reference": map[string]any{
						"type":        "string",
						"description": "Full OCM component reference: [type::]{repository}/[prefix]/{component}:{version}. Examples: 'ghcr.io/open-component-model/ocm//ocm.software/ocmcli:0.23.0', 'ctf::./path/to/ctf//my.org/myapp:1.0.0'",
					},
				},
				"required": []string{"reference"},
			},
		},
		{
			Name:        "list_component_versions",
			Description: "List all available versions of an OCM component in a repository. Use this to discover what versions exist, find the latest release, or enumerate versions matching a semver constraint.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"reference": map[string]any{
						"type":        "string",
						"description": "Repository and component reference without a version specifier. Example: 'ghcr.io/open-component-model/ocm//ocm.software/ocmcli'",
					},
					"constraint": map[string]any{
						"type":        "string",
						"description": "Optional semver constraint to filter versions, e.g. '>=1.0.0', '<2.0.0', '~1.2'. Leave empty for all versions.",
					},
					"latest_only": map[string]any{
						"type":        "boolean",
						"description": "If true, return only the single latest version.",
					},
				},
				"required": []string{"reference"},
			},
		},
		{
			Name:        "get_resource_info",
			Description: "Return metadata about the resources in an OCM component version including types, access specifications, cryptographic digests, and labels. Does NOT download resource content. Use this for security assessments, provenance analysis, or understanding what artifacts are available.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"reference": map[string]any{
						"type":        "string",
						"description": "Full OCM component reference including version.",
					},
					"resource_name": map[string]any{
						"type":        "string",
						"description": "Optional: filter to a specific resource by name. If omitted, returns info for all resources.",
					},
				},
				"required": []string{"reference"},
			},
		},
	}
}
