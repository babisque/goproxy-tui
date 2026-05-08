package tui

import (
	"strings"

	"github.com/babisque/goproxy-tui/internal/proxy"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	colorDarkGray  = lipgloss.Color("#333333")
	colorLightGray = lipgloss.Color("#A0A0A0")
	colorWhite     = lipgloss.Color("#FFFFFF")
	colorBlack     = lipgloss.Color("#000000")

	inactiveBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), true).
				BorderForeground(colorDarkGray).
				Padding(0, 1)

	activeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true).
			BorderForeground(colorWhite).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Bold(true).
			Underline(true)

	selectedItemStyle = lipgloss.NewStyle().
				Background(colorWhite).
				Foreground(colorBlack).
				Bold(true)
)

type App struct {
	width          int
	height         int
	logChannel     chan proxy.RequestLog
	requests       []proxy.RequestLog
	cursor         int
	listOffset     int
	detailsView    viewport.Model
	focusLeft      bool
	proxy          *proxy.ProxyHandler
	input          textinput.Model
	inputMode      bool
	inputTarget    string
	filterQuery    string
	showConfig     bool
	infoMsg        string
	interceptChan  chan proxy.InterceptRequest
	isIntercepting bool
	pendingReq     *proxy.InterceptRequest
	editor         textarea.Model
	editing        bool
}

type logMsg proxy.RequestLog
type interceptMsg proxy.InterceptRequest

func waitForIntercept(ch chan proxy.InterceptRequest) tea.Cmd {
	return func() tea.Msg { return interceptMsg(<-ch) }
}

func NewApp(ph *proxy.ProxyHandler, intCh chan proxy.InterceptRequest) App {
	vp := viewport.New(0, 0)
	vp.SetContent("(Empty)")

	ti := textinput.New()
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 60

	ta := textarea.New()
	ta.Placeholder = ""
	ta.Focus()
	ta.ShowLineNumbers = true
	ta.CharLimit = 0

	ta.Prompt = "  "
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(colorDarkGray)
	ta.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(colorLightGray)
	ta.FocusedStyle.Base = lipgloss.NewStyle()

	return App{
		logChannel:    ph.LogChannel,
		interceptChan: intCh,
		detailsView:   vp,
		focusLeft:     true,
		proxy:         ph,
		input:         ti,
		inputMode:     false,
		editor:        ta,
		editing:       false,
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		waitForLog(a.logChannel),
		waitForIntercept(a.interceptChan),
	)
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
