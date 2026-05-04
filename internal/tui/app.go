package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/babisque/goproxy-tui/internal/proxy"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	colorGray   = lipgloss.Color("#555555")
	colorWhite  = lipgloss.Color("#FFFFFF")
	colorAccent = lipgloss.Color("#7D56F4")

	inactiveBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), true).
				BorderForeground(colorGray).
				Padding(1, 2)

	activeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(colorAccent).
			Padding(1, 2)
)

type App struct {
	width       int
	height      int
	logChannel  chan proxy.RequestLog
	requests    []proxy.RequestLog
	cursor      int
	detailsView viewport.Model
	focusLeft   bool
}

type logMsg proxy.RequestLog

func NewApp(ch chan proxy.RequestLog) App {
	vp := viewport.New(0, 0)
	vp.SetContent("(Empty)")
	return App{
		logChannel:  ch,
		detailsView: vp,
		focusLeft:   true,
	}
}

func (a App) Init() tea.Cmd {
	return waitForLog(a.logChannel)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case logMsg:
		a.requests = append(a.requests, proxy.RequestLog(msg))
		if len(a.requests) == 1 {
			a.detailsView.SetContent(buildDetails(a.requests[a.cursor]))
		}
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
		case "tab":
			a.focusLeft = !a.focusLeft
		case "left", "h":
			a.focusLeft = true
		case "right", "l":
			a.focusLeft = false
		}

		if a.focusLeft {
			switch msg.String() {
			case "up", "k":
				if a.cursor > 0 {
					a.cursor--
					a.detailsView.SetContent(buildDetails(a.requests[a.cursor]))
					a.detailsView.GotoTop()
				}
			case "down", "j":
				if a.cursor < len(a.requests)-1 {
					a.cursor++
					a.detailsView.SetContent(buildDetails(a.requests[a.cursor]))
					a.detailsView.GotoTop()
				}
			}
		}
	}

	if !a.focusLeft {
		var vpCmd tea.Cmd
		a.detailsView, vpCmd = a.detailsView.Update(msg)
		cmds = append(cmds, vpCmd)
	} else {
		if _, isKey := msg.(tea.KeyMsg); !isKey {
			var vpCmd tea.Cmd
			a.detailsView, vpCmd = a.detailsView.Update(msg)
			cmds = append(cmds, vpCmd)
		}
	}

	return a, tea.Batch(cmds...)
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

	leftStyle := inactiveBoxStyle.Copy()
	if a.focusLeft {
		leftStyle = activeBoxStyle.Copy()
	}

	leftBox := leftStyle.
		Width(leftWidth).
		Height(boxHeight).
		Render(listBuilder.String())

	rightStyle := inactiveBoxStyle.Copy()
	if !a.focusLeft {
		rightStyle = activeBoxStyle.Copy()
	}

	rightBox := rightStyle.
		Width(rightWidth).
		Height(boxHeight).
		Render("Request details\n\n" + a.detailsView.View())

	ui := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)
	return lipgloss.NewStyle().Margin(1, 2).Render(ui)
}

func buildDetails(req proxy.RequestLog) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Method: %s\n", req.Method))
	b.WriteString(fmt.Sprintf("URL: %s\n", req.URL))
	b.WriteString(fmt.Sprintf("Status: %d\n\n", req.Status))
	b.WriteString("--- RESPONSE HEADERS ---\n")

	var keys []string
	for k := range req.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		values := req.Headers[key]
		joinedValues := strings.Join(values, ", ")
		coloredKey := lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Render(key + ":")
		b.WriteString(fmt.Sprintf("%s %s\n", coloredKey, joinedValues))
	}

	b.WriteString("\n--- RESPONSE BODY ---\n")

	var prettyJSON bytes.Buffer
	err := json.Indent(&prettyJSON, []byte(req.Body), "", "  ")
	if err == nil {
		b.WriteString(prettyJSON.String())
	} else {
		b.WriteString(req.Body)
	}

	return b.String()
}

func waitForLog(ch chan proxy.RequestLog) tea.Cmd {
	return func() tea.Msg {
		log := <-ch
		return logMsg(log)
	}
}
