package explorer

import (
	"context"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// New represents any command that is related to adding ( "add"ing ) objects
func New() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explorer",
		Short: "The OCM Explorer is a tool for exploring OCM interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			logs := &logModel{}
			slog.SetDefault(slog.New(logs))
			_, err := tea.NewProgram(NewModel(cmd.Context(), logs),
				tea.WithContext(cmd.Context()),
				tea.WithAltScreen(),
				tea.WithMouseCellMotion(),
				tea.WithOutput(cmd.OutOrStdout()),
				tea.WithInput(cmd.InOrStdin()),
				tea.WithReportFocus(),
				tea.WithFPS(30),
			).Run()
			return err
		},
	}

	return cmd
}

type logModel struct {
	msg []string
}

func (l *logModel) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

func (l *logModel) Handle(ctx context.Context, record slog.Record) error {
	l.msg = append(l.msg, record.Message)
	return nil
}

func (l *logModel) WithAttrs(attrs []slog.Attr) slog.Handler {
	return l
}

func (l *logModel) WithGroup(name string) slog.Handler {
	return l
}

func (l *logModel) View() string {
	var builder strings.Builder
	for _, msg := range l.msg {
		builder.WriteString(msg)
		builder.WriteString("\n")
	}
	l.msg = nil
	return builder.String()
}
