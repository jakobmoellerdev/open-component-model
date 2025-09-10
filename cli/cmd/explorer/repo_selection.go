package explorer

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"ocm.software/open-component-model/bindings/go/oci/repository/provider"
	"ocm.software/open-component-model/bindings/go/repository"
	ocmctx "ocm.software/open-component-model/cli/internal/context"
	"ocm.software/open-component-model/cli/internal/reference/compref"
)

func NewModel(ctx context.Context, logs *logModel) tea.Model {
	ti := textinput.New()
	ti.SetValue("ghcr.io/open-component-model/ocm")
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 20

	return &model{
		textInput: ti,
		logs:      logs,
		ctx:       ctx,
	}
}

// A model can be more or less any type of data. It holds all the data for a
// program, so often it's a struct. For this simple example, however, all
// we'll need is a simple integer.
type model struct {
	ctx  context.Context
	logs *logModel

	textInput textinput.Model
	err       error

	width, height int
}

func (m *model) Init() tea.Cmd {
	return textinput.Blink
}

// Update is called when messages are received. The idea is that you inspect the
// message and send back an updated model accordingly. You can also return
// a command, which is a function that performs I/O and returns a message.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "ctrl+z":
			return m, tea.Suspend
		case "enter":
			return m, m.resolve()
		}

	case repository.ComponentVersionRepository:
		m := NewRepoListModel(m, m.ctx, m.textInput.Value(), msg, m.logs, m.width, m.height)
		return m, tea.Batch(tea.ClearScreen, m.Init())
	}
	return m, nil
}

func (m *model) Err(err error) tea.Cmd {
	return func() tea.Msg {
		m.err = err
		return err
	}
}

// View returns a string based on data in the model. That string which will be
// rendered to the terminal.
func (m *model) View() string {
	var builder strings.Builder
	builder.WriteString(m.textInput.View())
	if m.err != nil {
		builder.WriteString(fmt.Sprintf("Error: %s\n", m.err.Error()))
	}

	builder.WriteString("\n")
	builder.WriteString(m.logs.View())

	return builder.String()
}

func (m *model) resolve() tea.Cmd {
	repo, err := compref.ParseRepository(m.textInput.Value())
	if err != nil {
		return m.Err(err)
	}

	pluginManager := ocmctx.FromContext(m.ctx).PluginManager()
	if pluginManager == nil {
		return m.Err(fmt.Errorf("could not retrieve plugin manager from context"))
	}

	credentialGraph := ocmctx.FromContext(m.ctx).CredentialGraph()
	if credentialGraph == nil {
		return m.Err(fmt.Errorf("could not retrieve credential graph from context"))
	}

	return func() tea.Msg {
		cvRepoProvider := provider.NewComponentVersionRepositoryProvider()
		var creds map[string]string
		id, err := cvRepoProvider.GetComponentVersionRepositoryCredentialConsumerIdentity(m.ctx, repo)
		if err == nil {
			creds, _ = credentialGraph.Resolve(m.ctx, id)
		}

		cvRepo, err := cvRepoProvider.GetComponentVersionRepository(m.ctx, repo, creds)
		if err != nil {
			return m.Err(err)
		}

		return cvRepo
	}
}
