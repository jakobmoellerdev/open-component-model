package explorer

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"ocm.software/open-component-model/bindings/go/repository"
)

type repoListModel struct {
	root tea.Model

	ctx  context.Context
	logs *logModel

	name      string
	repo      repository.ComponentVersionRepository
	listModel list.Model
	err       error
	loading   bool
	component string
}

type item struct {
	version string
}

func (i item) Title() string       { return i.version } // list.Item interface
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return i.version }

type itemDelegate struct{}

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	str := listItem.(item).version
	if index == m.Index() {
		str = "> " + str
	}
	_, _ = io.WriteString(w, str)
}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                              { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func NewRepoListModel(root tea.Model, ctx context.Context, ref string, repo repository.ComponentVersionRepository, logs *logModel, width, height int) tea.Model {
	listModel := list.New(nil, itemDelegate{}, width, height)
	listModel.Title = "Available Versions"

	return &repoListModel{
		component: "ocm.software/ocmcli",
		root:      root,
		name:      ref,
		repo:      repo,
		listModel: listModel,
		logs:      logs,
		ctx:       ctx,
	}
}

func (r *repoListModel) Init() tea.Cmd {
	return tea.Batch(
		r.listModel.StartSpinner(),
		r.list,
	)
}

func (r *repoListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.listModel.SetSize(msg.Width, msg.Height-5)
		return r, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return r.root, tea.ClearScreen
		case "enter":
			if sel, ok := r.listModel.SelectedItem().(item); ok {
				return NewVersionModel(r, r.ctx, r.repo, r.component, sel.version)
			}
		}

	case listFinished:
		if msg.err != nil {
			r.err = msg.err
		} else {
			r.listModel.SetItems(msg.versions)
		}
	}

	var cmd tea.Cmd
	r.listModel, cmd = r.listModel.Update(msg)
	return r, cmd
}

func (r *repoListModel) View() string {
	var b strings.Builder
	b.WriteString(r.name + "\n")
	if r.loading {
		b.WriteString("Loading...\n")
	}
	b.WriteString(r.logs.View())
	if r.err != nil {
		b.WriteString(r.err.Error() + "\n")
	}
	b.WriteString(r.listModel.View())
	return b.String()
}

type listFinished struct {
	time.Duration
	versions []list.Item
	err      error
}

func (r *repoListModel) list() tea.Msg {
	start := time.Now()
	var items []list.Item
	versions, err := r.repo.ListComponentVersions(r.ctx, r.component)
	if err == nil {
		for _, version := range versions {
			items = append(items, item{version: version})
		}
	}
	return listFinished{time.Since(start), items, err}
}
