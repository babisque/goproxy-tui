package tui

import (
	"fmt"
	"strings"

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
	requests   []proxy.RequestLog
	cursor     int
}

type logMsg proxy.RequestLog

func NewApp(ch chan proxy.RequestLog) App {
	return App{
		logChannel: ch,
	}
}

func (a App) Init() tea.Cmd {
	return waitForLog(a.logChannel)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case logMsg:
		a.requests = append(a.requests, proxy.RequestLog(msg))

		return a, waitForLog(a.logChannel)

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "up", "k":
			if a.cursor > 0 {
				a.cursor--
			}
		case "down", "j":
			if a.cursor < len(a.requests)-1 {
				a.cursor++
			}
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

	var listBuilder strings.Builder
	listBuilder.WriteString("Requests list\n\n")

	if len(a.requests) == 0 {
		listBuilder.WriteString("(Empty)")
	} else {
		for i, req := range a.requests {
			text := fmt.Sprintf("[%d] %s %s", req.Status, req.Method, req.URL)

			if i == a.cursor {
				row := lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Render("> " + text)
				listBuilder.WriteString(row + "\n")
			} else {
				row := lipgloss.NewStyle().Foreground(colorGray).Render("  " + text)
				listBuilder.WriteString(row + "\n")
			}
		}
	}

	leftBox := boxStyle.Copy().
		Width(leftWidth).
		Height(boxHeight).
		Render(listBuilder.String())

	var details string
	if len(a.requests) == 0 {
		details = "(Empty)"
	} else {
		req := a.requests[a.cursor]

		details = fmt.Sprintf("Method: %s\nURL: %s\nStatus: %d", req.Method, req.URL, req.Status)
	}

	rightBox := boxStyle.Copy().
		Width(rightWidth).
		Height(boxHeight).
		Render("Request details\n\n" + details)

	ui := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	return lipgloss.NewStyle().Margin(1, 2).Render(ui)
}

func waitForLog(ch chan proxy.RequestLog) tea.Cmd {
	return func() tea.Msg {
		log := <-ch
		return logMsg(log)
	}
}
