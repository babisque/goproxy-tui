package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/babisque/goproxy-tui/internal/proxy"
	"github.com/charmbracelet/bubbles/textinput"
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
	proxy       *proxy.ProxyHandler
	input       textinput.Model
	inputMode   bool
	inputTarget string
}

type logMsg proxy.RequestLog

func NewApp(ph *proxy.ProxyHandler) App {
	vp := viewport.New(0, 0)
	vp.SetContent("(Empty)")

	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	return App{
		logChannel:  ph.LogChannel,
		detailsView: vp,
		focusLeft:   true,
		proxy:       ph,
		input:       ti,
		inputMode:   false,
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
		a.detailsView.Height = a.height - 12

	case tea.KeyMsg:
		if a.inputMode {
			switch msg.String() {
			case "enter":
				domain := a.input.Value()
				if domain != "" {
					if a.inputTarget == "block" {
						a.proxy.AddBlocked(domain)
					} else {
						a.proxy.AddIgnored(domain)
					}
				}
				a.input.SetValue("")
				a.inputMode = false
				return a, nil
			case "esc":
				a.input.SetValue("")
				a.inputMode = false
				return a, nil
			}

			var cmd tea.Cmd
			a.input, cmd = a.input.Update(msg)
			return a, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "b":
			a.inputMode = true
			a.inputTarget = "block"
			a.input.Placeholder = "Enter domain to block..."
			return a, nil
		case "i":
			a.inputMode = true
			a.inputTarget = "ignore"
			a.input.Placeholder = "Enter domain to ignore..."
			return a, nil
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

	if !a.inputMode {
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
	}

	return a, tea.Batch(cmds...)
}

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	helpHeight := 2
	boxHeight := a.height - 4 - helpHeight
	if a.inputMode {
		boxHeight -= 2
	}

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
	if a.focusLeft && !a.inputMode {
		leftStyle = activeBoxStyle.Copy()
	}

	leftBox := leftStyle.
		Width(leftWidth).
		Height(boxHeight).
		Render(listBuilder.String())

	rightStyle := inactiveBoxStyle.Copy()
	if !a.focusLeft && !a.inputMode {
		rightStyle = activeBoxStyle.Copy()
	}

	rightBox := rightStyle.
		Width(rightWidth).
		Height(boxHeight).
		Render("Request details\n\n" + a.detailsView.View())

	ui := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	if a.inputMode {
		label := "BLOCK DOMAIN:"
		if a.inputTarget == "ignore" {
			label = "IGNORE DOMAIN:"
		}

		inputBox := lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(colorAccent).
			Padding(0, 1).
			Render(label)

		inputUI := lipgloss.JoinHorizontal(lipgloss.Left, inputBox, " ", a.input.View())
		ui = lipgloss.JoinVertical(lipgloss.Left, ui, "\n"+inputUI)
	}

	keyStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorGray)
	sep := descStyle.Render(" • ")

	var helpMenu string
	if a.inputMode {
		helpMenu = keyStyle.Render("enter") + descStyle.Render(" confirm") + sep +
			keyStyle.Render("esc") + descStyle.Render(" cancel")
	} else {
		helpMenu = keyStyle.Render("q") + descStyle.Render(" quit") + sep +
			keyStyle.Render("tab/h/l") + descStyle.Render(" switch panel") + sep +
			keyStyle.Render("j/k") + descStyle.Render(" navigate") + sep +
			keyStyle.Render("b") + descStyle.Render(" block") + sep +
			keyStyle.Render("i") + descStyle.Render(" ignore")
	}

	ui = lipgloss.JoinVertical(lipgloss.Left, ui, "\n"+helpMenu)

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
