package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
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
	filterQuery string
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
		filtered := a.FilteredRequests()
		if len(filtered) == 1 {
			a.detailsView.SetContent(buildDetails(filtered[a.cursor]))
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
				val := a.input.Value()
				if val != "" {
					switch a.inputTarget {
					case "block":
						a.proxy.AddBlocked(val)
					case "ignore":
						a.proxy.AddIgnored(val)
					case "remove":
						a.proxy.RemoveBlocked(val)
						a.proxy.RemoveIgnored(val)
					case "filter":
						a.filterQuery = val
						a.cursor = 0
						filtered := a.FilteredRequests()
						if len(filtered) > 0 {
							a.detailsView.SetContent(buildDetails(filtered[0]))
						}
					case "intercept":
						parts := strings.Split(val, ",")
						if len(parts) == 2 {
							host := strings.TrimSpace(parts[0])
							kv := strings.Split(parts[1], ":")
							if len(kv) == 2 {
								a.proxy.AddIntercept(host, strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
							}
						}
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
		case "esc":
			a.filterQuery = ""
			a.cursor = 0
			filtered := a.FilteredRequests()
			if len(filtered) > 0 {
				a.detailsView.SetContent(buildDetails(filtered[0]))
			}
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
		case "n":
			a.inputMode = true
			a.inputTarget = "intercept"
			a.input.Placeholder = "Format: host,header:value"
			return a, nil
		case "/":
			a.inputMode = true
			a.inputTarget = "filter"
			a.input.Placeholder = "Filter by URL..."
			return a, nil
		case "r":
			a.inputMode = true
			a.inputTarget = "remove"
			a.input.Placeholder = "Domain to remove..."
			return a, nil
		case "tab":
			a.focusLeft = !a.focusLeft
		case "left", "h":
			a.focusLeft = true
		case "right", "l":
			a.focusLeft = false
		}

		if a.focusLeft {
			filtered := a.FilteredRequests()
			switch msg.String() {
			case "up", "k":
				if a.cursor > 0 {
					a.cursor--
					a.detailsView.SetContent(buildDetails(filtered[a.cursor]))
					a.detailsView.GotoTop()
				}
			case "down", "j":
				if a.cursor < len(filtered)-1 {
					a.cursor++
					a.detailsView.SetContent(buildDetails(filtered[a.cursor]))
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

	filtered := a.FilteredRequests()
	leftWidth := (a.width / 100) * 30
	rightWidth := a.width - leftWidth - 6

	var listBuilder strings.Builder
	title := "Requests list"
	if a.filterQuery != "" {
		title = fmt.Sprintf("Requests list (Filter: %s)", a.filterQuery)
	}
	listBuilder.WriteString(title + "\n\n")

	if len(filtered) == 0 {
		listBuilder.WriteString("(Empty)")
	} else {
		for i, req := range filtered {
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

	leftBox := leftStyle.Width(leftWidth).Height(boxHeight).Render(listBuilder.String())

	rightStyle := inactiveBoxStyle.Copy()
	if !a.focusLeft && !a.inputMode {
		rightStyle = activeBoxStyle.Copy()
	}

	rightBox := rightStyle.Width(rightWidth).Height(boxHeight).Render("Request details\n\n" + a.detailsView.View())

	ui := lipgloss.JoinHorizontal(lipgloss.Top, leftBox, rightBox)

	if a.inputMode {
		label := "COMMAND:"
		switch a.inputTarget {
		case "block":
			label = "BLOCK DOMAIN:"
		case "ignore":
			label = "IGNORE DOMAIN:"
		case "filter":
			label = "SEARCH URL:"
		case "remove":
			label = "REMOVE DOMAIN:"
		case "intercept":
			label = "INTERCEPT (host,h:v):"
		}

		inputBox := lipgloss.NewStyle().Foreground(colorWhite).Background(colorAccent).Padding(0, 1).Render(label)
		ui = lipgloss.JoinVertical(lipgloss.Left, ui, "\n"+lipgloss.JoinHorizontal(lipgloss.Left, inputBox, " ", a.input.View()))
	}

	keyStyle := lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(colorGray)
	sep := descStyle.Render(" • ")

	helpMenu := ""
	if a.inputMode {
		helpMenu = keyStyle.Render("enter") + descStyle.Render(" confirm") + sep + keyStyle.Render("esc") + descStyle.Render(" cancel")
	} else {
		helpMenu = keyStyle.Render("q") + descStyle.Render(" quit") + sep +
			keyStyle.Render("esc") + descStyle.Render(" clear filter") + sep +
			keyStyle.Render("j/k") + descStyle.Render(" navigate") + sep +
			keyStyle.Render("b/i/r") + descStyle.Render(" block/ignore/rem") + sep +
			keyStyle.Render("n") + descStyle.Render(" intercept") + sep +
			keyStyle.Render("/") + descStyle.Render(" filter")
	}

	return lipgloss.NewStyle().Margin(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left, ui, "\n"+helpMenu))
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

func buildDetails(req proxy.RequestLog) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Method: %s\nURL: %s\nStatus: %d\n\n--- HEADERS ---\n", req.Method, req.URL, req.Status))
	keys := make([]string, 0, len(req.Headers))
	for k := range req.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Render(k+":") + " " + strings.Join(req.Headers[k], ", ") + "\n")
	}
	b.WriteString("\n--- BODY ---\n")

	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(req.Body), "", "  "); err == nil {
		lexer := lexers.Get("json")
		style := styles.Get("monokai")
		formatter := formatters.Get("terminal256")
		iterator, _ := lexer.Tokenise(nil, pretty.String())
		formatter.Format(&b, style, iterator)
	} else {
		b.WriteString(req.Body)
	}
	return b.String()
}

func waitForLog(ch chan proxy.RequestLog) tea.Cmd {
	return func() tea.Msg { return logMsg(<-ch) }
}
