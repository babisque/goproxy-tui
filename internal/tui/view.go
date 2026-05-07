package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/babisque/goproxy-tui/internal/proxy"
	"github.com/charmbracelet/lipgloss"
)

func (a App) View() string {
	if a.showConfig {
		var b strings.Builder

		configTitleStyle := lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Underline(true)
		subStyle := lipgloss.NewStyle().Foreground(colorWhite).Bold(true)

		b.WriteString(configTitleStyle.Render("GOPROXY RULES & CONFIGURATION") + "\n\n")

		b.WriteString(subStyle.Render("Blocked Domains (403 Forbidden):") + "\n")
		blocked := a.proxy.BlockedDomains.ToSlice()
		if len(blocked) == 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render("  (None)\n"))
		}
		for _, d := range blocked {
			b.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render("  - " + d + "\n"))
		}
		b.WriteString("\n")

		b.WriteString(subStyle.Render("Ignored Domains (Bypass MITM):") + "\n")
		ignored := a.proxy.IgnoredDomains.ToSlice()
		if len(ignored) == 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render("  (None)\n"))
		}
		for _, d := range ignored {
			b.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render("  - " + d + "\n"))
		}
		b.WriteString("\n")

		b.WriteString(subStyle.Render("Intercepted Headers:") + "\n")
		if len(a.proxy.InterceptRules) == 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render("  (None)\n"))
		}
		for _, r := range a.proxy.InterceptRules {
			for k, v := range r.Headers {
				b.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render(fmt.Sprintf("  - %s -> Injects [%s: %s]\n", r.Host, k, v)))
			}
		}
		b.WriteString("\n")

		b.WriteString(subStyle.Render("Request Modifiers (POST/PUT/PATCH):") + "\n")
		if len(a.proxy.RequestRules) == 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render("  (None)\n"))
		}
		for _, r := range a.proxy.RequestRules {
			b.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render(fmt.Sprintf("  - %s: Replaces '%s' with '%s'\n", r.Host, r.OldText, r.NewText)))
		}
		b.WriteString("\n")

		b.WriteString(subStyle.Render("Response Modifiers:") + "\n")
		if len(a.proxy.ResponseRules) == 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render("  (None)\n"))
		}
		for _, r := range a.proxy.ResponseRules {
			b.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render(fmt.Sprintf("  - %s: Replaces '%s' with '%s'\n", r.Host, r.OldText, r.NewText)))
		}

		help := lipgloss.NewStyle().Foreground(colorDarkGray).Render("\n\nv, esc: close config and return to proxy")

		configBox := activeBoxStyle.Copy().
			Width(a.width - 4).
			Height(a.height - 4).
			Render(b.String() + help)

		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, configBox)
	}

	if a.width == 0 {
		return "Loading..."
	}

	boxHeight := a.height - 4
	if a.inputMode {
		boxHeight -= 3
	}
	visibleHeight := boxHeight - 5
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	filtered := a.FilteredRequests()
	leftWidth := (a.width / 100) * 30
	rightWidth := a.width - leftWidth - 6

	var listBuilder strings.Builder

	headerTitle := " REQUESTS LIST "
	if a.filterQuery != "" {
		headerTitle = fmt.Sprintf(" SEARCH: %s ", a.filterQuery)
	}
	listBuilder.WriteString(titleStyle.Render(headerTitle) + "\n\n")

	if len(filtered) == 0 {
		listBuilder.WriteString(lipgloss.NewStyle().Foreground(colorLightGray).Render("(Empty)"))
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
				paddedLine := fmt.Sprintf(" %-*s ", leftWidth-4, lineText)
				listBuilder.WriteString(selectedItemStyle.Render(paddedLine) + "\n")
			} else {
				listBuilder.WriteString(lipgloss.NewStyle().
					Foreground(colorLightGray).
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

	leftContent := strings.TrimRight(listBuilder.String(), "\n")

	ui := lipgloss.JoinHorizontal(lipgloss.Top,
		leftStyle.Render(leftContent),
		rightStyle.Render(titleStyle.Render(" INSPECTOR ")+"\n\n"+a.detailsView.View()),
	)

	helpText := "q: quit • I: intercept • j/k: nav • tab: swap • a: accept • d: drop • /: search"
	var footer string

	alertStyle := lipgloss.NewStyle().Background(lipgloss.Color("#FF0000")).Foreground(colorWhite).Bold(true).Padding(0, 1)
	onStyle := lipgloss.NewStyle().Background(lipgloss.Color("#550000")).Foreground(colorWhite).Bold(true).Padding(0, 1)

	if a.pendingReq != nil {
		footer = "\n " + alertStyle.Render(" INTERCEPTED REQUEST ") + " " + lipgloss.NewStyle().Foreground(colorDarkGray).Render(helpText)
	} else if a.isIntercepting {
		footer = "\n " + onStyle.Render(" INTECEPTOR ON ") + " " + lipgloss.NewStyle().Foreground(colorDarkGray).Render(helpText)
	} else if a.infoMsg != "" {
		footer = "\n " + selectedItemStyle.Render(fmt.Sprintf(" %s ", a.infoMsg))
	} else {
		footer = lipgloss.NewStyle().Foreground(colorDarkGray).Render("\n " + helpText)
	}

	finalView := lipgloss.JoinVertical(lipgloss.Left, ui, footer)

	if a.inputMode {
		prompt := selectedItemStyle.Render(" " + strings.ToUpper(a.inputTarget) + " ")
		inputView := lipgloss.JoinHorizontal(lipgloss.Left, prompt, " ", a.input.View())
		finalView = lipgloss.JoinVertical(lipgloss.Left, ui, "\n"+inputView, footer)
	}

	return finalView
}

func buildDetails(req proxy.RequestLog, width int) string {
	var b strings.Builder
	labelStyle := lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(colorLightGray)

	b.WriteString(labelStyle.Render("METHOD: ") + valueStyle.Render(req.Method) + "\n")
	b.WriteString(labelStyle.Render("URL:    ") + valueStyle.Render(req.URL) + "\n")
	b.WriteString(labelStyle.Render("STATUS: ") + valueStyle.Render(fmt.Sprint(req.Status)) + "\n\n")

	b.WriteString(titleStyle.Render("HEADERS") + "\n")
	keys := make([]string, 0, len(req.Headers))
	for k := range req.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(labelStyle.Render(k+": ") + valueStyle.Render(strings.Join(req.Headers[k], ", ")) + "\n")
	}

	b.WriteString("\n" + titleStyle.Render("PAYLOAD") + "\n")

	bodyText := req.Body
	if bodyText == "" {
		b.WriteString(valueStyle.Render("(Empty)"))
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
		Foreground(colorLightGray).
		Render(bodyText)

	b.WriteString(wrappedBody)

	return b.String()
}
