package tui

import (
	"github.com/babisque/goproxy-tui/internal/proxy"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	colorGray  = lipgloss.Color("#555555")
	colorWhite = lipgloss.Color("#FFFFFF")

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(colorGray).
			Padding(1, 2)
)

type App struct {
	width      int
	height     int
	logChannel chan proxy.RequestLog
}

func NewApp(ch chan proxy.RequestLog) App {
	return App{
		logChannel: ch,
	}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		}
	}
	return a, nil
}

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	boxHeight := a.height - 4
	leftWidth := (a.width / 100) * 30
	rightWidth := a.width - leftWidth - 6

	leftBox := boxStyle.Copy().
		Width(leftWidth).
		Height(boxHeight).
		Render("Requests list\n\n(Empty)")

	rightBox := boxStyle.Copy().
		Width(rightWidth).
		Height(boxHeight).
		Render("Request details\n\n(Empty)")

	ui := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	return lipgloss.NewStyle().Margin(1, 2).Render(ui)
}
