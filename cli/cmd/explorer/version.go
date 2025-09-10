package explorer

import (
	"context"
	"errors"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"ocm.software/open-component-model/bindings/go/descriptor/runtime"
	"ocm.software/open-component-model/bindings/go/repository"
	runtime2 "ocm.software/open-component-model/bindings/go/runtime"
	"sigs.k8s.io/yaml"
)

type editorFinishedMsg error

// openEditor writes the version to a temp file and launches $EDITOR on it.
func openEditor(ctx context.Context, repo repository.ComponentVersionRepository, component, version string) tea.Cmd {
	cv, err := repo.GetComponentVersion(ctx, component, version)
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg(err) }
	}

	v2, err := runtime.ConvertToV2(runtime2.NewScheme(runtime2.WithAllowUnknown()), cv)
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg(err) }
	}

	data, err := yaml.Marshal(v2)
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg(err) }
	}

	tmp, err := os.CreateTemp("", "component-*.yaml")
	if err != nil {
		return func() tea.Msg { return editorFinishedMsg(err) }
	}
	if _, err := tmp.Write(data); err != nil {
		return func() tea.Msg { return editorFinishedMsg(err) }
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}
	c := exec.Command(editor, tmp.Name())
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg(errors.Join(err, tmp.Close()))
	})
}

func NewVersionModel(root tea.Model, ctx context.Context, repo repository.ComponentVersionRepository, component, version string) (*VersionModel, tea.Cmd) {
	m := &VersionModel{
		root:      root,
		ctx:       ctx,
		repo:      repo,
		component: component,
		version:   version,
	}
	return m, m.Init()
}

type VersionModel struct {
	root tea.Model

	ctx                context.Context
	component, version string
	repo               repository.ComponentVersionRepository

	altscreenActive bool
	err             error
}

func (m *VersionModel) Init() tea.Cmd {
	return openEditor(m.ctx, m.repo, m.component, m.version)
}

func (m *VersionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m.root, tea.ClearScreen
		}
		switch msg.Type {
		case tea.KeyEscape:
			return m.root, tea.ClearScreen
		}
	case tea.WindowSizeMsg:
	case editorFinishedMsg:
		if msg != nil {
			m.err = msg
			return m, tea.ClearScreen
		} else {
			return m.root, tea.ClearScreen
		}
	}
	return m, nil
}

func (m *VersionModel) View() string {
	if m.err != nil {
		return m.err.Error()
	} else {
		return "Loading editor ..."
	}
}
