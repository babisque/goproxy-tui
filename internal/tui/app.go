package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/babisque/goproxy-tui/internal/proxy"
	"github.com/charmbracelet/bubbles/viewport"
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
	width       int
	height      int
	logChannel  chan proxy.RequestLog
	requests    []proxy.RequestLog
	cursor      int
	detailsView viewport.Model
}

type logMsg proxy.RequestLog

func NewApp(ch chan proxy.RequestLog) App {
	return App{
		logChannel:  ch,
		detailsView: viewport.New(0, 0),
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

		leftWidth := (a.width / 100) * 30
		rightWidth := a.width - leftWidth - 6

		a.detailsView.Width = rightWidth - 4

		a.detailsView.Height = a.height - 10

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

	var cmd tea.Cmd
	a.detailsView, cmd = a.detailsView.Update(msg)

	return a, cmd
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

	var detailsBuilder strings.Builder

	if len(a.requests) == 0 {
		detailsBuilder.WriteString("(Empty)")
	} else {
		req := a.requests[a.cursor]

		detailsBuilder.WriteString(fmt.Sprintf("Method: %s\n", req.Method))
		detailsBuilder.WriteString(fmt.Sprintf("URL: %s\n", req.URL))
		detailsBuilder.WriteString(fmt.Sprintf("Status: %d\n\n", req.Status))

		detailsBuilder.WriteString("--- RESPONSE HEADERS ---\n")

		var keys []string
		for k := range req.Headers {
			keys = append(keys, k)
		}

		sort.Strings(keys)

		for _, key := range keys {
			values := req.Headers[key]
			joinedValues := strings.Join(values, ", ")

			coloredKey := lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Render(key + ":")
			detailsBuilder.WriteString(fmt.Sprintf("%s %s\n", coloredKey, joinedValues))
		}

		detailsBuilder.WriteString("\n--- RESPONSE BODY ---\n")
		detailsBuilder.WriteString(req.Body)
	}

	a.detailsView.SetContent(detailsBuilder.String())

	rightBox := boxStyle.Copy().
		Width(rightWidth).
		Height(boxHeight).
		Render("Request details\n\n" + a.detailsView.View())

	ui := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	return lipgloss.NewStyle().Margin(1, 2).Render(ui)
}

func waitForLog(ch chan proxy.RequestLog) tea.Cmd {
	return func() tea.Msg {
		log := <-ch
		return logMsg(log)
	}
}
