# MCP Server for OCM CLI Natural Language Interface

* Status: proposed
* Deciders: Jakob Möller
* Date: 2026-04-03

Technical Story: Provide an MCP (Model Context Protocol) server that exposes OCM component version operations as tools, enabling AI agents (Claude Desktop, Claude Code, and other MCP clients) to introspect and work with component versions using natural language.

## Context and Problem Statement

Users and operators working with OCM component versions need to inspect, analyze, and understand component metadata — resources, sources, references, signatures, provenance, and dependencies. This is currently done through the CLI (`ocm get cv`, `ocm download resource`, etc.), which requires knowing the exact command syntax and reference format.

AI agents like Claude have become powerful tools for working with structured data, but they need programmatic access to OCM operations. The question is: **what is the best integration point for AI tooling?**

## Decision Drivers

* Any MCP-compatible AI client should be able to use OCM, not just Claude
* The OCM CLI (`ocm`) should remain clean and focused — AI tooling should not pollute it
* The integration must reuse existing OCM infrastructure (plugin system, credential graph, config) without duplication
* Minimal new dependencies; no external AI SDK needed
* Must work with existing `.ocmconfig` credentials transparently

## Considered Options

1. **[Option 1]** Embed an `ocm ai` command directly in the CLI (with bundled Claude API client)
2. **[Option 2]** Standalone MCP server binary that wraps OCM operations
3. **[Option 3]** External tool that pipes `ocm get cv -o json` output to an AI

## Decision Outcome

**Chosen: Option 2 — Standalone MCP server.**

Justification:

* **Any agent can use it**: MCP is an open protocol. Claude Desktop, Claude Code, Cursor, and any other MCP-compatible client can use the server without modification
* **Clean separation**: AI tooling is a separate concern from the OCM CLI workflow. Users who don't use AI agents don't need the extra binary or dependencies
* **OCM context reuse**: The server initializes the same plugin manager and credential graph as the CLI, so existing `.ocmconfig` credentials work transparently
* **No AI SDK dependency**: The MCP protocol is simple JSON-RPC 2.0 over stdio — no external SDK needed. The server contains zero AI-specific code; it just exposes OCM operations as tools
* **Testable in isolation**: Each tool handler can be tested independently against real or mock OCM repositories

### Trust Model

The MCP server never receives AI model output or sends data to any AI provider. It only:
1. Receives tool call requests from the local MCP client (which mediates between the AI and the server)
2. Fetches component metadata from OCM repositories using the user's existing credentials
3. Returns structured JSON to the local MCP client

Component descriptors fetched from private registries are returned to the local MCP client and may be forwarded to an AI provider by the client. Users should be aware of this when connecting to private registries.

## Implementation

### Module Structure

A new Go module at `mcp/` (registered in `go.work`):

```
mcp/
  main.go          # stdin/stdout JSON-RPC loop + OCM context initialization
  server.go        # MCP protocol (initialize, tools/list, tools/call)
  ocmconfig.go     # OCM config file loading (mirrors CLI config lookup)
  plugins.go       # Builtin plugin registration (OCI + Docker credential)
  tools/
    tools.go                    # Tool registry and MCP tool definitions
    get_component_version.go    # Fetches complete component descriptor
    list_component_versions.go  # Lists versions with optional semver filter
    get_resource_info.go        # Resource metadata (no binary download)
```

No dependency on `cli/` — only on `bindings/go/` modules.

### Exposed MCP Tools

| Tool | Description |
|------|-------------|
| `get_component_version` | Fetch full component descriptor as JSON (resources, sources, references, signatures, labels) |
| `list_component_versions` | List versions with optional semver constraint and latest-only filter |
| `get_resource_info` | Resource metadata summary (type, access spec type, digest, labels) — no binary download |

### Protocol

MCP uses JSON-RPC 2.0 over stdio. The server reads newline-delimited JSON from stdin, responds on stdout. All logging goes to stderr.

### Configuration

The server uses the standard OCM config file lookup (`OCM_CONFIG` env → `~/.config/ocm/config` → `$PWD/.ocmconfig`). No MCP-specific configuration is needed. Credentials for OCM repositories are resolved through the standard credential graph.

Plugin directory defaults to `$HOME/.config/ocm/plugins` (overridable via `OCM_PLUGIN_DIRECTORY` env var).

### Claude Desktop / Claude Code Integration

Add to MCP client configuration:

```json
{
  "mcpServers": {
    "ocm": {
      "command": "ocm-mcp-server",
      "args": []
    }
  }
}
```

## Pros and Cons of the Options

### [Option 1] Embed `ocm ai` in the CLI

Pros:
* Single binary; no extra installation step
* Direct access to all CLI internals

Cons:
* Ties AI provider choice (Claude) to the CLI — other agents can't use it
* Adds AI SDK dependency to the CLI binary for all users, including those who don't use AI
* Requires API key management inside the CLI

### [Option 2] Standalone MCP Server (chosen)

Pros:
* Protocol-agnostic: any MCP client works
* Clean separation of concerns
* No extra dependencies in the main CLI
* Zero AI-specific code in the server

Cons:
* Requires installing a second binary (`ocm-mcp-server`)
* Users must configure their MCP client

### [Option 3] External pipe-based tool

Pros:
* No code changes needed — just pipe `ocm get cv -o json` to an AI CLI tool

Cons:
* No agentic tool-calling (AI can't autonomously call multiple OCM operations)
* Subprocess spawning overhead for each operation
* No streaming results; full output must be buffered before sending to AI
