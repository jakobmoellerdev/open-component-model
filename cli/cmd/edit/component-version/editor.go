package componentversion

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// resolveEditor determines the editor to use based on the flag, environment
// variables, and a fallback default.
func resolveEditor(editorFlag string) string {
	if editorFlag != "" {
		return editorFlag
	}
	if v := os.Getenv("VISUAL"); v != "" {
		return v
	}
	if v := os.Getenv("EDITOR"); v != "" {
		return v
	}
	return "vi"
}

// launchEditor writes content to a temporary file, opens the editor, and
// returns the (possibly modified) file contents after the editor exits.
func launchEditor(cmd *cobra.Command, content []byte, format, editorOverride string) ([]byte, error) {
	ext := ".yaml"
	if format == "json" {
		ext = ".json"
	}

	tmpFile, err := os.CreateTemp("", "ocm-edit-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("closing temp file: %w", err)
	}

	editor := resolveEditor(editorOverride)
	parts := strings.Fields(editor)
	args := append(parts[1:], tmpFile.Name())

	editorCmd := exec.CommandContext(cmd.Context(), parts[0], args...)
	editorCmd.Stdin = cmd.InOrStdin()
	editorCmd.Stdout = cmd.OutOrStdout()
	editorCmd.Stderr = cmd.ErrOrStderr()

	if err := editorCmd.Run(); err != nil {
		return nil, fmt.Errorf("editor exited with error: %w", err)
	}

	edited, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("reading edited file: %w", err)
	}

	return edited, nil
}
