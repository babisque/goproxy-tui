package tui

import (
	"strings"

	"github.com/babisque/goproxy-tui/internal/proxy"
	tea "github.com/charmbracelet/bubbletea"
)

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	boxHeight := a.height - 4
	if a.inputMode {
		boxHeight -= 3
	}

	visibleHeight := boxHeight - 5
	if visibleHeight < 1 {
		visibleHeight = 1
	}

	leftWidth := (a.width / 100) * 30
	rightWidth := a.width - leftWidth - 6

	switch msg := msg.(type) {
	case interceptMsg:
		req := proxy.InterceptRequest(msg)
		a.pendingReq = &req

		a.detailsView.SetContent(buildDetails(req.Log, rightWidth))
		a.detailsView.GotoTop()

		return a, waitForIntercept(a.interceptChan)
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

		newBoxHeight := msg.Height - 4
		if a.inputMode {
			newBoxHeight -= 3
		}

		a.detailsView.Width = rightWidth - 4
		a.detailsView.Height = newBoxHeight - 5

	case tea.KeyMsg:
		if msg.String() != "d" {
			a.infoMsg = ""
		}
		if a.pendingReq != nil && !a.inputMode {
			switch msg.String() {
			case "a":
				a.pendingReq.ActionCh <- proxy.InterceptAction{Allow: true}
				a.pendingReq = nil
				a.infoMsg = "Request allowed"
				return a, nil
			case "d":
				a.pendingReq.ActionCh <- proxy.InterceptAction{Allow: false}
				a.pendingReq = nil
				a.infoMsg = "Request dropped"
				return a, nil
			}

			return a, nil
		}
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
		case "I":
			a.isIntercepting = a.proxy.ToggleIntercept()
			if a.isIntercepting {
				a.infoMsg = "Intercept mode enabled: new requests will be paused for review."
			} else {
				a.infoMsg = "Intercept mode disabled: new requests will flow without interruption."
				if a.pendingReq != nil {
					a.pendingReq.ActionCh <- proxy.InterceptAction{Allow: true}
					a.pendingReq = nil
				}
			}
			return a, nil
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
		case "c":
			a.requests = nil
			a.cursor, a.listOffset = 0, 0
			a.detailsView.SetContent("(Empty)")
			return a, nil
		case "v":
			a.showConfig = !a.showConfig
		case "M":
			a.inputMode = true
			a.inputTarget = "modify_req"
			a.input.Focus()
		case "d":
			a.infoMsg = a.exportCurrentRequest()
		case "R":
			filtered := a.FilteredRequests()
			if len(filtered) > 0 && a.cursor < len(filtered) {
				reqToReplay := filtered[a.cursor]
				a.infoMsg = "Replaying request..."
				go a.proxy.ReplayRequest(reqToReplay)
			}
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
	case "modify_req":
		parts := strings.Split(val, ",")
		if len(parts) == 2 {
			tp := strings.Split(parts[1], ":")
			if len(tp) == 2 {
				a.proxy.AddRequestRule(proxy.RequestRule{
					Host:    strings.TrimSpace(parts[0]),
					OldText: strings.TrimSpace(tp[0]),
					NewText: strings.TrimSpace(tp[1]),
				})
			}
		}

	}
}
