---
title: "Migrating to the New OCM Go Bindings"
date: 2026-07-22T10:00:00+02:00
draft: false
description: "A practical guide to migrating from the legacy github.com/open-component-model/ocm library to the new modular Go bindings in OCM v2."
summary: "The OCM v2 reboot ships a new Go library built around modularity, testability, and native OCI compliance. This post walks through the motivations, the key API differences, and concrete migration patterns for each layer of the stack."
categories: ["library"]
tags: ["go", "migration", "v2", "bindings", "supply-chain"]
contributors: []
---

OCM v2 ships with a ground-up rewrite of the Go library. If you are currently using `github.com/open-component-model/ocm`, this guide walks through what changed, why it changed, and how to migrate your code.

{{< callout context="note" title="Module path" >}}
The new library lives at `ocm.software/open-component-model/bindings/go/...` inside the [OCM monorepo](https://github.com/open-component-model/open-component-model/tree/main/bindings/go). The legacy library at `github.com/open-component-model/ocm` remains supported until at least the end of 2026.
{{< /callout >}}

---

## Why a new library?

The original library accumulated four problems that made it increasingly painful to work with:

1. **Tight coupling and global state.** Type registries and plugin state were registered through `init()` functions, making it impossible to isolate behaviour in tests.
2. **Complex interface abstractions.** The `ComponentVersionAccess` hierarchy was difficult to extend without risk of regressions in unrelated areas.
3. **Hard-coded dependencies.** Pulling in the library pulled in its entire dependency tree — every access method, every storage backend — even if your use case only touched a small slice.
4. **Session and context management.** The `ocm.Context` and `session.Close()` lifecycle spread implicit state across call sites and made safe, idiomatic Go hard to write.

The new library solves all four. It is split into small independent modules, uses standard `context.Context`, passes credentials and configuration explicitly, and is designed around plain Go interfaces that any backend can implement.

---

## What the new library looks like

Rather than one large `go.mod`, the new bindings are a collection of small modules you opt into:

| Module | Purpose |
|---|---|
| `bindings/go/blob` | Core blob abstraction (`ReadOnlyBlob`) |
| `bindings/go/descriptor/runtime` | Descriptor types (`Descriptor`, `Resource`, `Source`, `Reference`) |
| `bindings/go/descriptor/v2` | OCM v2 wire format and scheme |
| `bindings/go/repository` | Abstract `ComponentVersionRepository` interface |
| `bindings/go/oci` | OCI registry repository implementation |
| `bindings/go/ctf` | Common Transport Format (filesystem-based) |
| `bindings/go/credentials` | Identity-based credential resolution |
| `bindings/go/signing` | Digest generation and signature verification |
| `bindings/go/rsa` | RSA sign/verify handler |
| `bindings/go/transfer` | Transfer graph API |
| `bindings/go/constructor` | YAML-driven component construction |
| `bindings/go/configuration` | OCM config management |
| `bindings/go/http` | HTTP client configuration |

Import only what your code actually uses. A program that only reads descriptors from a CTF archive has no reason to pull in the OCI or signing packages.

---

## Concept mapping: old vs new

| Old API | New equivalent |
|---|---|
| `ocm.New()` / `ocm.Context` | No equivalent — use explicit config and credential objects |
| `ocm.Session`, `session.Close()` | No equivalent — use standard `context.Context` |
| `ctx.OCMContext().RepositoryForSpec(spec)` | `oci.NewRepository(...)` with functional options |
| `CompVersionAccess.GetResource(...)` | `repo.GetLocalResource(ctx, component, version, identity)` |
| `CompVersionAccess.AddBlob(...)` | `repo.AddLocalResource(ctx, component, version, res, blob)` |
| `resource.AsBlobAccess()` | `ReadOnlyBlob` returned directly |
| `signing.SignComponentVersion(ctx, ...)` | `handler.Sign(ctx, digest, cfg, creds)` |
| Global `init()` type registration | Explicit `runtime.NewScheme()` + `v2.MustAddToScheme(scheme)` |

---

## Repositories

The abstract interface your code should target is `repository.ComponentVersionRepository`:

```go
type ComponentVersionRepository interface {
    AddComponentVersion(ctx context.Context, desc *descriptor.Descriptor) error
    GetComponentVersion(ctx context.Context, component, version string) (*descriptor.Descriptor, error)
    ListComponentVersions(ctx context.Context, component string) ([]string, error)
    // plus LocalResourceRepository and LocalSourceRepository
}
```

Both CTF archives and live OCI registries implement this interface. You switch backends by changing how you construct the repository, not by changing any business logic.

### CTF-backed repository

```go
import (
    "os"
    "ocm.software/open-component-model/bindings/go/blob/filesystem"
    "ocm.software/open-component-model/bindings/go/ctf"
    "ocm.software/open-component-model/bindings/go/oci"
    ocictf "ocm.software/open-component-model/bindings/go/oci/ctf"
)

fs, err := filesystem.NewFS(dir, os.O_RDWR)
store := ocictf.NewFromCTF(ctf.NewFileSystemCTF(fs))
repo, err := oci.NewRepository(ocictf.WithCTF(store), oci.WithTempDir(tmpDir))
```

### OCI registry

```go
import (
    "ocm.software/open-component-model/bindings/go/oci"
    "ocm.software/open-component-model/bindings/go/oci/repository/urlresolver"
)

resolver, err := urlresolver.New(
    urlresolver.WithBaseURL("ghcr.io/my-org"),
    urlresolver.WithBaseClient(&auth.Client{...}),
)
repo, err := oci.NewRepository(oci.WithResolver(resolver), oci.WithTempDir(tmpDir))
```

The repository type is identical in both cases. Everything below this construction point is portable.

### Storing and retrieving component versions

```go
desc := &descriptor.Descriptor{
    Meta: descriptor.Meta{Version: "v2"},
    Component: descriptor.Component{
        Provider: descriptor.Provider{Name: "acme.org"},
        ComponentMeta: descriptor.ComponentMeta{
            ObjectMeta: descriptor.ObjectMeta{
                Name:    "acme.org/my-app",
                Version: "1.0.0",
            },
        },
    },
}

if err := repo.AddComponentVersion(ctx, desc); err != nil {
    return err
}

got, err := repo.GetComponentVersion(ctx, "acme.org/my-app", "1.0.0")
```

`repository.ErrNotFound` is returned when the component version does not exist — check for it with `errors.Is`.

### Adding resources

```go
import (
    "ocm.software/open-component-model/bindings/go/blob/inmemory"
    v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
    "github.com/opencontainers/go-digest"
)

content := []byte("resource payload")
res := &descriptor.Resource{
    Relation: descriptor.LocalRelation,
    ElementMeta: descriptor.ElementMeta{
        ObjectMeta: descriptor.ObjectMeta{Name: "my-resource", Version: "1.0.0"},
    },
    Type: "plainText",
    Access: &v2.LocalBlob{
        LocalReference: digest.FromBytes(content).String(),
        MediaType:      "text/plain",
    },
}

b := inmemory.New(bytes.NewReader(content))
newRes, err := repo.AddLocalResource(ctx, component, version, res, b)
```

Retrieval uses a map-based identity:

```go
readBlob, _, err := repo.GetLocalResource(ctx, component, version, map[string]string{
    "name":    "my-resource",
    "version": "1.0.0",
})

var buf bytes.Buffer
if err := blob.Copy(&buf, readBlob); err != nil {
    return err
}
```

---

## Signing

Signing is now a two-step operation: generate a digest from the descriptor, then sign the digest.

```go
import (
    "crypto"
    "log/slog"
    "ocm.software/open-component-model/bindings/go/descriptor/normalisation/json/v4alpha1"
    "ocm.software/open-component-model/bindings/go/signing"
    rsahandler "ocm.software/open-component-model/bindings/go/rsa/signing/handler"
    "ocm.software/open-component-model/bindings/go/rsa/signing/v1alpha1"
    v1 "ocm.software/open-component-model/bindings/go/rsa/spec/credentials/v1"
)

logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

// 1. Generate the digest.
dig, err := signing.GenerateDigest(ctx, desc, logger, v4alpha1.Algorithm, crypto.SHA256.String())

// 2. Sign with the RSA handler.
handler, err := rsahandler.New(v1alpha1.Scheme, false)
cfg := &v1alpha1.Config{
    SignatureAlgorithm:      v1alpha1.AlgorithmRSASSAPSS,
    SignatureEncodingPolicy: v1alpha1.SignatureEncodingPolicyPlain,
}
sigInfo, err := handler.Sign(ctx, *dig, cfg, &v1.RSACredentials{
    Type:              v1.VersionedType,
    PrivateKeyPEMFile: privKeyPath,
})

// 3. Assemble and verify.
sig := descriptor.Signature{Name: "my-sig", Digest: *dig, Signature: sigInfo}
err = handler.Verify(ctx, sig, nil, &v1.RSACredentials{
    Type:             v1.VersionedType,
    PublicKeyPEMFile: pubCertPath,
})
```

Before signing, check that all component references carry digests:

```go
if err := signing.IsSafelyDigestible(&desc.Component); err != nil {
    return fmt.Errorf("component has undigested references: %w", err)
}
```

The `logger` parameter accepts any `*slog.Logger` — pass `slog.Default()` in production or a discard logger in tests.

---

## Transfer

The transfer API models a move as a graph: build the graph definition, then execute it.

```go
import (
    "ocm.software/open-component-model/bindings/go/transfer"
    "ocm.software/open-component-model/bindings/go/oci/repository/provider"
    "ocm.software/open-component-model/bindings/go/oci/repository/resource"
)

// Describe the transfer.
tgd, err := transfer.BuildGraphDefinition(ctx,
    transfer.WithTransfer(
        transfer.Component(component, version),
        transfer.ToRepositorySpec(targetSpec),
        transfer.FromRepository(sourceRepo, sourceSpec),
    ),
)

// Execute it.
repoProvider := provider.NewComponentVersionRepositoryProvider(
    provider.WithTempDir(tmpDir),
)
resourceRepo := resource.NewResourceRepository(nil)

builder := transfer.NewDefaultBuilder(repoProvider, resourceRepo, nil)
graph, err := builder.BuildAndCheck(tgd)
if err := graph.Process(ctx); err != nil {
    return err
}
```

The same graph API handles CTF-to-CTF, CTF-to-OCI, and OCI-to-OCI transfers. Recursive transfers and resource copy modes are controlled through options on `WithTransfer`.

---

## Credentials

Credentials are resolved by identity — a typed key-value map — rather than threaded through a global context. The static resolver is suitable for tests and simple programs:

```go
import (
    "ocm.software/open-component-model/bindings/go/credentials"
    "ocm.software/open-component-model/bindings/go/runtime"
)

resolver := credentials.NewStaticTypedCredentialsResolver(map[string]runtime.Typed{
    "hostname=registry.example.com,type=OCIRegistry": &ocicredsv1.OCICredentials{...},
})

creds, err := resolver.Resolve(ctx, runtime.Identity{
    "type":     "OCIRegistry",
    "hostname": "registry.example.com",
})
```

`credentials.ErrNotFound` is returned when no identity matches. Production programs wiring OCM config files use the configuration package to build a resolver from `.ocmconfig` entries.

---

## Running the examples

The `bindings/go/examples/` directory contains eight runnable examples covering every topic above — blobs, descriptors, credentials, repositories, signing, OCI registry round-trips, HTTP config, and transfer. All of them run as standard Go tests:

```bash
# From the repo root:
task bindings/go/examples:test

# Or directly with go test (skips the live-registry test):
cd bindings/go/examples
go test -short -v ./...
```

Each example file is self-contained. Step 4 (repositories) and Step 8 (transfer) are a good starting point if you are migrating existing code that reads or writes component versions.

---

## Checklist

When migrating an existing program, work through these layers in order:

{{< steps >}}
{{< step >}}
**Replace the module dependency**

Remove `github.com/open-component-model/ocm` from `go.mod`. Add the specific `ocm.software/open-component-model/bindings/go/...` modules your code needs.
{{< /step >}}
{{< step >}}
**Remove `ocm.Context` and sessions**

Replace `ocm.New()`, `ctx.OCMContext()`, and `session.Close()` with explicit credential and configuration objects passed as function arguments.
{{< /step >}}
{{< step >}}
**Replace repository construction**

Replace `ctx.OCMContext().RepositoryForSpec(spec)` with `oci.NewRepository(...)` using functional options. The same interface covers CTF and OCI backends.
{{< /step >}}
{{< step >}}
**Replace resource access**

Replace `CompVersionAccess.GetResource(...)` and `AsBlobAccess()` with `repo.GetLocalResource(ctx, component, version, identity)` returning a `ReadOnlyBlob`.
{{< /step >}}
{{< step >}}
**Replace signing**

Replace `signing.SignComponentVersion(...)` with the two-step `signing.GenerateDigest` + `handler.Sign` pattern. Use `signing.IsSafelyDigestible` as a pre-flight check.
{{< /step >}}
{{< step >}}
**Replace type registration**

Remove any `init()`-driven type registration. Construct schemes explicitly with `runtime.NewScheme()` and register types as needed.
{{< /step >}}
{{< /steps >}}

---

## Get help

- Browse the examples: [`bindings/go/examples/`](https://github.com/open-component-model/open-component-model/tree/main/bindings/go/examples)
- Read the library overview: [`bindings/go/README.md`](https://github.com/open-component-model/open-component-model/blob/main/bindings/go/README.md)
- Ask a question on Zulip: [neonephos-ocm-support](https://linuxfoundation.zulipchat.com/#narrow/channel/532975-neonephos-ocm-support)
- File an issue: [github.com/open-component-model/open-component-model](https://github.com/open-component-model/open-component-model/issues)
