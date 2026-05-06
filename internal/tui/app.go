package tui

import (
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
	showConfig  bool
	infoMsg     string
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

func waitForLog(ch chan proxy.RequestLog) tea.Cmd {
	return func() tea.Msg { return logMsg(<-ch) }
}
