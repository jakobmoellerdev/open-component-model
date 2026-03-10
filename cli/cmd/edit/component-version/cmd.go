package componentversion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"

	normalisation "ocm.software/open-component-model/bindings/go/descriptor/normalisation"
	"ocm.software/open-component-model/bindings/go/descriptor/normalisation/json/v4alpha1"
	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/oci/compref"
	ctfv1 "ocm.software/open-component-model/bindings/go/oci/spec/repository/v1/ctf"
	ociv1 "ocm.software/open-component-model/bindings/go/oci/spec/repository/v1/oci"
	"ocm.software/open-component-model/bindings/go/runtime"
	ocmctx "ocm.software/open-component-model/cli/internal/context"
	"ocm.software/open-component-model/cli/internal/flags/enum"
	"ocm.software/open-component-model/cli/internal/repository/ocm"
)

const (
	FlagForce                  = "force"
	FlagDryRun                 = "dry-run"
	FlagEditor                 = "editor"
	FlagFormat                 = "format"
	FlagNormalisationAlgorithm = "normalisation"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "component-version {reference}",
		Aliases:    []string{"cv", "component-versions", "cvs", "componentversion", "componentversions", "component", "components", "comp", "comps", "c"},
		SuggestFor: []string{"version", "versions"},
		Short:      "Edit a component version descriptor interactively",
		Args:       cobra.MatchAll(cobra.ExactArgs(1), componentReferenceAsFirstPositional),
		Long: fmt.Sprintf(`Open a component version descriptor in a text editor for interactive editing.

The descriptor is serialized to YAML (default) or JSON before opening, and
validated after editing. Changes that would invalidate existing signatures
(signature-relevant fields) or modify access specifications are rejected
unless --force is used.

Signature relevance is determined by normalizing the descriptor using the
configured normalisation algorithm (default: %[4]s) — the same algorithm
used during signing. Use --normalisation to match a different signing setup.

## Reference Format

	[type::]{repository}/[valid-prefix]/{component}[:version]

- Prefixes: {%[1]s|none} (default: %[1]q)
- Repo types: {%[2]s} (short: {%[3]s})`,
			compref.DefaultPrefix,
			strings.Join([]string{ociv1.Type, ctfv1.Type}, "|"),
			strings.Join([]string{ociv1.ShortType, ociv1.ShortType2, ctfv1.ShortType, ctfv1.ShortType2}, "|"),
			v4alpha1.Algorithm,
		),
		Example: strings.TrimSpace(`
# Edit a component version from a CTF archive
edit component-version ctf::./repo//my-component:1.0.0

# Edit using JSON format
edit cv ctf::./repo//my-component:1.0.0 --format json

# Dry-run: validate edits without persisting
edit cv ctf::./repo//my-component:1.0.0 --dry-run

# Force edit of signature-relevant fields
edit cv ctf::./repo//my-component:1.0.0 --force

# Use a specific editor
edit cv ctf::./repo//my-component:1.0.0 --editor "code --wait"
`),
		RunE:              EditComponentVersion,
		DisableAutoGenTag: true,
	}

	cmd.Flags().Bool(FlagForce, false, "allow edits to signature-relevant fields and access specifications")
	cmd.Flags().Bool(FlagDryRun, false, "validate and print result without persisting")
	cmd.Flags().String(FlagEditor, "", "editor command (overrides $VISUAL/$EDITOR, default: vi)")
	cmd.Flags().String(FlagNormalisationAlgorithm, v4alpha1.Algorithm, "normalisation algorithm used to determine signature relevance")
	enum.VarP(cmd.Flags(), FlagFormat, "f", []string{"yaml", "json"}, "serialization format for the editor")

	return cmd
}

func componentReferenceAsFirstPositional(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing component reference as first positional argument")
	}
	if _, err := compref.Parse(args[0]); err != nil {
		return fmt.Errorf("parsing component reference from first positional argument %q failed: %w", args[0], err)
	}
	return nil
}

func EditComponentVersion(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	ocmContext := ocmctx.FromContext(ctx)
	if ocmContext == nil {
		return fmt.Errorf("no OCM context found")
	}

	pluginManager := ocmContext.PluginManager()
	if pluginManager == nil {
		return fmt.Errorf("plugin manager not available in context")
	}

	credentialGraph := ocmContext.CredentialGraph()
	if credentialGraph == nil {
		return fmt.Errorf("credential graph not available in context")
	}

	// Flags
	force, _ := cmd.Flags().GetBool(FlagForce)
	dryRun, _ := cmd.Flags().GetBool(FlagDryRun)
	editorFlag, _ := cmd.Flags().GetString(FlagEditor)
	format, err := enum.Get(cmd.Flags(), FlagFormat)
	if err != nil {
		return fmt.Errorf("getting format flag failed: %w", err)
	}

	// Parse reference with ReadWrite access
	reference := args[0]
	ref, err := compref.Parse(reference, compref.WithCTFAccessMode(ctfv1.AccessModeReadWrite))
	if err != nil {
		return fmt.Errorf("parsing component reference %q failed: %w", reference, err)
	}

	config := ocmContext.Configuration()
	repoProvider, err := ocm.NewComponentVersionRepositoryForComponentProvider(
		ctx, pluginManager.ComponentVersionRepositoryRegistry, credentialGraph, config, ref,
	)
	if err != nil {
		return fmt.Errorf("could not initialize ocm repository: %w", err)
	}

	repo, err := repoProvider.GetComponentVersionRepositoryForComponent(ctx, ref.Component, ref.Version)
	if err != nil {
		return fmt.Errorf("could not access ocm repository: %w", err)
	}

	// Fetch descriptor
	desc, err := repo.GetComponentVersion(ctx, ref.Component, ref.Version)
	if err != nil {
		return fmt.Errorf("getting component version failed: %w", err)
	}

	// Convert to v2 for serialization
	scheme := runtime.NewScheme(runtime.WithAllowUnknown())
	originalV2, err := descruntime.ConvertToV2(scheme, desc)
	if err != nil {
		return fmt.Errorf("converting descriptor to v2 failed: %w", err)
	}

	// Serialize
	var originalBytes []byte
	switch format {
	case "json":
		originalBytes, err = json.MarshalIndent(originalV2, "", "  ")
	default:
		originalBytes, err = yaml.Marshal(originalV2)
	}
	if err != nil {
		return fmt.Errorf("serializing descriptor failed: %w", err)
	}

	// Launch editor
	editedBytes, err := launchEditor(cmd, originalBytes, format, editorFlag)
	if err != nil {
		return err
	}

	// Early exit on no changes
	if bytes.Equal(originalBytes, editedBytes) {
		fmt.Fprintln(cmd.ErrOrStderr(), "Edit cancelled: no changes detected.")
		return nil
	}

	// Deserialize edited content
	editedV2 := &v2.Descriptor{}
	if err := yaml.Unmarshal(editedBytes, editedV2); err != nil {
		return fmt.Errorf("parsing edited descriptor failed: %w", err)
	}

	// Check signature relevance via normalization
	normAlgo := cmd.Flag(FlagNormalisationAlgorithm).Value.String()
	sigRelevant, err := isSignatureRelevant(originalV2, editedV2, normAlgo)
	if err != nil {
		return fmt.Errorf("checking signature relevance failed: %w", err)
	}

	// Check access changes
	accessChanges := findAccessChanges(originalV2, editedV2)

	// Policy enforcement
	if sigRelevant && !force {
		sigCount := len(originalV2.Signatures)
		msg := "Error: edit changes the normalized descriptor (signature-relevant).\n"
		if sigCount > 0 {
			msg += fmt.Sprintf("\nThis component version has %d signature(s) that would be invalidated.\n", sigCount)
		}
		msg += "Use --force to apply these changes anyway."
		return fmt.Errorf("%s", msg)
	}

	if len(accessChanges) > 0 && !force {
		msg := "Error: edit modifies access specifications.\n\nModified accesses:\n"
		for _, path := range accessChanges {
			msg += fmt.Sprintf("  - %s\n", path)
		}
		msg += "\nAccess fields control artifact resolution. Use --force to apply."
		return fmt.Errorf("%s", msg)
	}

	// Warn when --force is used with blocked changes
	if force {
		if sigRelevant {
			fmt.Fprintln(cmd.ErrOrStderr(), "Warning: applying signature-relevant changes (--force)")
		}
		if len(accessChanges) > 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "Warning: applying access specification changes (--force)")
		}
	}

	// Convert back to runtime descriptor
	editedDesc, err := descruntime.ConvertFromV2(editedV2)
	if err != nil {
		return fmt.Errorf("converting edited descriptor from v2 failed: %w", err)
	}

	if dryRun {
		// Print the result
		var outBytes []byte
		switch format {
		case "json":
			outBytes, err = json.MarshalIndent(editedV2, "", "  ")
		default:
			outBytes, err = yaml.Marshal(editedV2)
		}
		if err != nil {
			return fmt.Errorf("serializing result failed: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(outBytes))
		fmt.Fprintln(cmd.ErrOrStderr(), "Dry run: changes not persisted.")
		return nil
	}

	// Persist
	if err := repo.AddComponentVersion(ctx, editedDesc); err != nil {
		return fmt.Errorf("updating component version failed: %w", err)
	}

	fmt.Fprintln(cmd.ErrOrStderr(), "Component version updated successfully.")
	return nil
}

// isSignatureRelevant normalizes both descriptors using the given algorithm
// (v2 → runtime → normalize) and compares the canonical output.
// This is exactly the path the signing pipeline uses.
func isSignatureRelevant(original, edited *v2.Descriptor, normAlgo string) (bool, error) {
	origNorm, err := normaliseV2(original, normAlgo)
	if err != nil {
		return false, fmt.Errorf("normalizing original: %w", err)
	}
	editNorm, err := normaliseV2(edited, normAlgo)
	if err != nil {
		return false, fmt.Errorf("normalizing edited: %w", err)
	}
	return !bytes.Equal(origNorm, editNorm), nil
}

// normaliseV2 converts a v2 descriptor to runtime and normalizes it using the
// specified normalisation algorithm.
func normaliseV2(desc *v2.Descriptor, normAlgo string) ([]byte, error) {
	rtDesc, err := descruntime.ConvertFromV2(desc)
	if err != nil {
		return nil, err
	}
	return normalisation.Normalise(rtDesc, normAlgo)
}

// findAccessChanges compares resource and source access fields between the
// original and edited descriptors. Access is excluded from normalization but
// controls artifact resolution, so it gets its own check.
func findAccessChanges(original, edited *v2.Descriptor) []string {
	var changes []string

	oldRes := indexByKey(original.Component.Resources, func(r v2.Resource) string {
		return elementKey(r.Name, r.ExtraIdentity)
	})
	for _, nw := range edited.Component.Resources {
		key := elementKey(nw.Name, nw.ExtraIdentity)
		if old, ok := oldRes[key]; ok && !rawEqual(old.Access, nw.Access) {
			changes = append(changes, fmt.Sprintf("component.resources[%s].access", key))
		}
	}

	oldSrc := indexByKey(original.Component.Sources, func(s v2.Source) string {
		return elementKey(s.Name, s.ExtraIdentity)
	})
	for _, nw := range edited.Component.Sources {
		key := elementKey(nw.Name, nw.ExtraIdentity)
		if old, ok := oldSrc[key]; ok && !rawEqual(old.Access, nw.Access) {
			changes = append(changes, fmt.Sprintf("component.sources[%s].access", key))
		}
	}

	return changes
}

// --- Helpers ---

func elementKey(name string, extraIdentity runtime.Identity) string {
	if len(extraIdentity) == 0 {
		return name
	}
	b, _ := json.Marshal(extraIdentity)
	return name + "+" + string(b)
}

func indexByKey[T any](items []T, keyFn func(T) string) map[string]T {
	m := make(map[string]T, len(items))
	for _, item := range items {
		m[keyFn(item)] = item
	}
	return m
}

func rawEqual(a, b *runtime.Raw) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return bytes.Equal(a.Data, b.Data)
}
