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
	listOffset  int
	detailsView viewport.Model
	focusLeft   bool
	proxy       *proxy.ProxyHandler
	input       textinput.Model
	inputMode   bool
	inputTarget string
	filterQuery string
}

type logMsg proxy.RequestLog

func NewApp(ph *proxy.ProxyHandler) App {
	vp := viewport.New(0, 0)
	vp.SetContent("(Empty)")

	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 60

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

	boxHeight := a.height - 8
	if a.inputMode {
		boxHeight -= 3
	}

	visibleHeight := boxHeight - 6
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	leftWidth := (a.width / 100) * 30
	rightWidth := a.width - leftWidth - 6

	switch msg := msg.(type) {
	case logMsg:
		a.requests = append(a.requests, proxy.RequestLog(msg))
		filtered := a.FilteredRequests()

		if a.cursor == len(filtered)-2 || len(filtered) == 1 {
			a.cursor = len(filtered) - 1
			if a.cursor >= a.listOffset+visibleHeight {
				a.listOffset = a.cursor - visibleHeight + 1
			}
			a.detailsView.SetContent(buildDetails(filtered[a.cursor], rightWidth))
		}
		return a, waitForLog(a.logChannel)

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.detailsView.Width = rightWidth - 4
		a.detailsView.Height = boxHeight - 6

	case tea.KeyMsg:
		if a.inputMode {
			switch msg.String() {
			case "enter":
				val := a.input.Value()
				if val != "" {
					a.handleCommand(val)
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
		case "esc":
			a.filterQuery = ""
			a.cursor = 0
			a.listOffset = 0
			filtered := a.FilteredRequests()
			if len(filtered) > 0 {
				a.detailsView.SetContent(buildDetails(filtered[0], rightWidth))
			}
		case "tab":
			a.focusLeft = !a.focusLeft
		case "left", "h":
			a.focusLeft = true
		case "right", "l":
			a.focusLeft = false
		case "b":
			a.inputMode, a.inputTarget, a.input.Placeholder = true, "block", "Domain to block..."
		case "i":
			a.inputMode, a.inputTarget, a.input.Placeholder = true, "ignore", "Domain to ignore..."
		case "n":
			a.inputMode, a.inputTarget, a.input.Placeholder = true, "intercept", "host,header:value"
		case "m":
			a.inputMode, a.inputTarget, a.input.Placeholder = true, "modify", "host,oldText:newText"
		case "/":
			a.inputMode, a.inputTarget, a.input.Placeholder = true, "filter", "Search URL..."
		case "r":
			a.inputMode, a.inputTarget, a.input.Placeholder = true, "remove", "Domain to remove..."
		}

		if a.focusLeft {
			filtered := a.FilteredRequests()
			switch msg.String() {
			case "up", "k":
				if a.cursor > 0 {
					a.cursor--
					if a.cursor < a.listOffset {
						a.listOffset = a.cursor
					}
					a.detailsView.SetContent(buildDetails(filtered[a.cursor], rightWidth))
					a.detailsView.GotoTop()
				}
			case "down", "j":
				if a.cursor < len(filtered)-1 {
					a.cursor++
					if a.cursor >= a.listOffset+visibleHeight {
						a.listOffset = a.cursor - visibleHeight + 1
					}
					a.detailsView.SetContent(buildDetails(filtered[a.cursor], rightWidth))
					a.detailsView.GotoTop()
				}
			}
		}
	}

	if !a.inputMode && !a.focusLeft {
		var vpCmd tea.Cmd
		a.detailsView, vpCmd = a.detailsView.Update(msg)
		cmds = append(cmds, vpCmd)
	}

	return a, tea.Batch(cmds...)
}

func (a *App) handleCommand(val string) {
	switch a.inputTarget {
	case "block":
		a.proxy.AddBlocked(val)
	case "ignore":
		a.proxy.AddIgnored(val)
	case "filter":
		a.filterQuery = val
		a.cursor, a.listOffset = 0, 0
	case "remove":
		a.proxy.RemoveBlocked(val)
		a.proxy.RemoveIgnored(val)
	case "intercept":
		parts := strings.Split(val, ",")
		if len(parts) == 2 {
			kv := strings.Split(parts[1], ":")
			if len(kv) == 2 {
				a.proxy.AddIntercept(strings.TrimSpace(parts[0]), strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
			}
		}
	case "modify":
		parts := strings.Split(val, ",")
		if len(parts) == 2 {
			tp := strings.Split(parts[1], ":")
			if len(tp) == 2 {
				a.proxy.AddResponseRule(proxy.ResponseRule{
					Host:    strings.TrimSpace(parts[0]),
					OldText: strings.TrimSpace(tp[0]),
					NewText: strings.TrimSpace(tp[1]),
				})
			}
		}
	}
}

func (a App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	boxHeight := a.height - 8
	if a.inputMode {
		boxHeight -= 3
	}
	visibleHeight := boxHeight - 6
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	filtered := a.FilteredRequests()
	leftWidth := (a.width / 100) * 30
	rightWidth := a.width - leftWidth - 6

	var listBuilder strings.Builder
	title := lipgloss.NewStyle().Bold(true).Underline(true).Render("Requests list")
	if a.filterQuery != "" {
		title = lipgloss.NewStyle().Bold(true).Underline(true).Render(fmt.Sprintf("Search: %s", a.filterQuery))
	}
	listBuilder.WriteString(title + "\n\n")

	if len(filtered) == 0 {
		listBuilder.WriteString("(Empty)")
	} else {
		endIndex := a.listOffset + visibleHeight
		if endIndex > len(filtered) {
			endIndex = len(filtered)
		}

		for i := a.listOffset; i < endIndex; i++ {
			req := filtered[i]

			maxLineLen := leftWidth - 6
			methodPart := fmt.Sprintf("[%d] %s ", req.Status, req.Method)
			lineText := methodPart + req.URL

			if len(lineText) > maxLineLen {
				lineText = lineText[:maxLineLen-3] + "..."
			}

			if i == a.cursor {
				listBuilder.WriteString(lipgloss.NewStyle().
					Foreground(colorWhite).
					Background(colorAccent).
					Width(leftWidth-4).
					Render("> "+lineText) + "\n")
			} else {
				listBuilder.WriteString(lipgloss.NewStyle().
					Foreground(colorGray).
					Width(leftWidth-4).
					Render("  "+lineText) + "\n")
			}
		}
	}

	leftStyle := inactiveBoxStyle.Copy().Width(leftWidth).Height(boxHeight)
	if a.focusLeft && !a.inputMode {
		leftStyle = activeBoxStyle.Copy().Width(leftWidth).Height(boxHeight)
	}

	rightStyle := inactiveBoxStyle.Copy().Width(rightWidth).Height(boxHeight)
	if !a.focusLeft && !a.inputMode {
		rightStyle = activeBoxStyle.Copy().Width(rightWidth).Height(boxHeight)
	}

	ui := lipgloss.JoinHorizontal(lipgloss.Top,
		leftStyle.Render(listBuilder.String()),
		rightStyle.Render("Request details\n\n"+a.detailsView.View()),
	)

	help := lipgloss.NewStyle().Foreground(colorGray).Render("q: quit • esc: clear • j/k: nav • tab: swap • b/i/r: rules • n/m: intercept/modify • /: search")

	finalView := lipgloss.JoinVertical(lipgloss.Left, ui, "\n"+help)

	if a.inputMode {
		prompt := lipgloss.NewStyle().Background(colorAccent).Foreground(colorWhite).Padding(0, 1).Render(strings.ToUpper(a.inputTarget) + ":")
		inputView := lipgloss.JoinHorizontal(lipgloss.Left, prompt, " ", a.input.View())
		finalView = lipgloss.JoinVertical(lipgloss.Left, ui, "\n"+inputView, "\n"+help)
	}

	return finalView
}

func (a App) FilteredRequests() []proxy.RequestLog {
	if a.filterQuery == "" {
		return a.requests
	}
	var res []proxy.RequestLog
	q := strings.ToLower(a.filterQuery)
	for _, r := range a.requests {
		if strings.Contains(strings.ToLower(r.URL), q) || strings.Contains(strings.ToLower(r.Method), q) {
			res = append(res, r)
		}
	}
	return res
}

func buildDetails(req proxy.RequestLog, width int) string {
	var b strings.Builder
	labelStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)

	b.WriteString(labelStyle.Render("METHOD: ") + req.Method + "\n")
	b.WriteString(labelStyle.Render("URL:    ") + req.URL + "\n")
	b.WriteString(labelStyle.Render("STATUS: ") + fmt.Sprint(req.Status) + "\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(colorWhite).Underline(true).Render("HEADERS") + "\n")
	keys := make([]string, 0, len(req.Headers))
	for k := range req.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(labelStyle.Render(k+": ") + strings.Join(req.Headers[k], ", ") + "\n")
	}

	b.WriteString("\n" + lipgloss.NewStyle().Foreground(colorWhite).Underline(true).Render("BODY") + "\n")

	bodyText := req.Body
	if bodyText == "" {
		b.WriteString("(Empty)")
		return b.String()
	}

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(bodyText), "", "  "); err == nil {
		bodyText = pretty.String()
	}

	bodyText = strings.ReplaceAll(bodyText, `\n`, "\n")
	bodyText = strings.ReplaceAll(bodyText, `\"`, `"`)

	cleaned := make([]rune, 0, len(bodyText))
	for _, r := range bodyText {
		if r == '\n' || r == '\t' || (r >= 32 && r != 127) {
			cleaned = append(cleaned, r)
		} else {
			cleaned = append(cleaned, '·')
		}
	}
	bodyText = string(cleaned)

	wrappedBody := lipgloss.NewStyle().
		Width(width - 6).
		Render(bodyText)

	b.WriteString(wrappedBody)

	return b.String()
}

func waitForLog(ch chan proxy.RequestLog) tea.Cmd {
	return func() tea.Msg { return logMsg(<-ch) }
}
