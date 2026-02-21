package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Catalog struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Hash string `json:"root_hash"`
}

type Bundle struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FileItem struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type Message struct {
	AuthorID string `json:"author_id"`
	Content  string `json:"content"`
	Time     int64  `json:"created_at"`
}

type model struct {
	width  int
	height int

	input textinput.Model
	err   error

	// Data
	messages    []Message
	catalogs    []Catalog
	bundles     []Bundle
	files       []FileItem
	bundleChain []Bundle
	activeCat   *Catalog

	// UI State
	cursor int // index of the active list item
}

// Styling tokens
var (
	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("202")).
			Padding(1)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")). // Cyan
			Bold(true).
			MarginBottom(1)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("202")). // Amber
				Bold(true)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))

	systemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	msgAuthorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	msgTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
)

func LaunchTUI() {
	ti := textinput.New()
	ti.Placeholder = "Transmit message..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	m := model{
		input: ti,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
	}
}

// -- Commands --

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchMessagesCmd() tea.Msg {
	resp, err := http.Get(defaultAPIUrl + "/api/chat")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var messages []Message
	json.Unmarshal(body, &messages)

	if len(messages) > 15 {
		messages = messages[len(messages)-15:]
	}
	return messages
}

type catalogsMsg []Catalog

func fetchCatalogsCmd() tea.Msg {
	resp, err := http.Get(defaultAPIUrl + "/api/catalogs")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var cats []Catalog
	json.Unmarshal(body, &cats)
	return catalogsMsg(cats)
}

type contentMsg struct {
	Bundles []Bundle
	Files   []FileItem
}

func fetchContentCmd(catalogID string, bundleID string) tea.Cmd {
	return func() tea.Msg {
		endpoint := fmt.Sprintf("%s/api/catalogs/%s", defaultAPIUrl, catalogID)
		if bundleID != "" {
			endpoint = fmt.Sprintf("%s/api/bundles/%s", defaultAPIUrl, bundleID)
		}

		resp, err := http.Get(endpoint)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var data struct {
			Bundles []Bundle   `json:"bundles"`
			Files   []FileItem `json:"files"`
		}
		json.Unmarshal(body, &data)
		return contentMsg{Bundles: data.Bundles, Files: data.Files}
	}
}

func importURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		payload := map[string]string{"url": url}
		body, _ := json.Marshal(payload)
		resp, err := http.Post(defaultAPIUrl+"/api/import/url", "application/json", bytes.NewBuffer(body))
		if err != nil {
			return fmt.Errorf("import error: %w", err)
		}
		defer resp.Body.Close()
		return fetchCatalogsCmd()
	}
}

// -- TEA Methods --

func (m model) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		tickCmd(),
		func() tea.Msg { return fetchMessagesCmd() },
		func() tea.Msg { return fetchCatalogsCmd() },
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			max := len(m.catalogs) - 1
			if m.activeCat != nil {
				max = len(m.bundles) + len(m.files)
				if m.activeCat != nil {
					max++ // For the ".." back button
				}
			}
			if m.cursor < max {
				m.cursor++
			}
		case "enter":
			// Handle Input Chat override first if focused, wait no, input always reacts to enter if typing.
			// Actually we have a single pane. Let's make "enter" send chat IF input has text.
			if m.input.Value() != "" {
				msgText := m.input.Value()
				m.input.SetValue("")
				
				if len(msgText) > 8 && msgText[:8] == "/import " {
					cmds = append(cmds, importURLCmd(msgText[8:]))
				} else {
					cmds = append(cmds, func() tea.Msg {
						payload := map[string]string{"content": msgText, "ref_target_id": ""}
						data, _ := json.Marshal(payload)
						http.Post(defaultAPIUrl+"/api/chat", "application/json", bytes.NewBuffer(data))
						return fetchMessagesCmd()
					})
				}
			} else {
				// Handle Navigation
				if m.activeCat == nil {
					if len(m.catalogs) > 0 && m.cursor < len(m.catalogs) {
						cat := m.catalogs[m.cursor]
						m.activeCat = &cat
						m.bundleChain = nil
						m.cursor = 0
						cmds = append(cmds, fetchContentCmd(cat.ID, ""))
					}
				} else {
					// We are inside a catalog
					idx := m.cursor
					if idx == 0 {
						// Go back
						if len(m.bundleChain) > 0 {
							m.bundleChain = m.bundleChain[:len(m.bundleChain)-1]
							bID := ""
							if len(m.bundleChain) > 0 {
								bID = m.bundleChain[len(m.bundleChain)-1].ID
							}
							cmds = append(cmds, fetchContentCmd(m.activeCat.ID, bID))
						} else {
							m.activeCat = nil // Return to root catalogs
						}
						m.cursor = 0
					} else {
						idx-- // offset the ".."
						if idx < len(m.bundles) {
							// Dive into bundle
							b := m.bundles[idx]
							m.bundleChain = append(m.bundleChain, b)
							m.cursor = 0
							cmds = append(cmds, fetchContentCmd(m.activeCat.ID, b.ID))
						} else {
							// File selected, wait
							// We could trigger a download or show details. TUI file action.
							// For now, no-op or flash a message.
							fIdx := idx - len(m.bundles)
							if fIdx >= 0 && fIdx < len(m.files) {
								m.input.SetValue("/fetch " + m.files[fIdx].ID)
							}
						}
					}
				}
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		cmds = append(cmds, tickCmd(), func() tea.Msg { return fetchMessagesCmd() })
	case []Message:
		m.messages = msg
	case catalogsMsg:
		m.catalogs = msg
	case contentMsg:
		m.bundles = msg.Bundles
		m.files = msg.Files
	case error:
		m.err = msg
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if m.width == 0 {
		return "Initializing Vault TUI..."
	}

	paneWidth := (m.width / 2) - 4
	paneHeight := m.height - 8

	// --- Left Pane: Explorer ---
	expTitle := "Root Catalogs"
	if m.activeCat != nil {
		expTitle = m.activeCat.Name
		for _, b := range m.bundleChain {
			expTitle += " / " + b.Name
		}
	}

	expContent := headerStyle.Render(expTitle) + "\n\n"

	if m.activeCat == nil {
		for i, c := range m.catalogs {
			cursor := "  "
			style := itemStyle
			if m.cursor == i {
				cursor = "> "
				style = selectedItemStyle
			}
			expContent += style.Render(fmt.Sprintf("%s[DB] %s", cursor, c.Name)) + "\n"
		}
		if len(m.catalogs) == 0 {
			expContent += systemStyle.Render("  No catalogs found.")
		}
	} else {
		// Navigation inside Catalog Context
		cursor := "  "
		style := itemStyle
		if m.cursor == 0 {
			cursor = "> "
			style = selectedItemStyle
		}
		expContent += style.Render(fmt.Sprintf("%s.. (Back)", cursor)) + "\n"

		for i, b := range m.bundles {
			idx := i + 1
			cursor = "  "
			style = itemStyle
			if m.cursor == idx {
				cursor = "> "
				style = selectedItemStyle
			}
			expContent += style.Render(fmt.Sprintf("%s[DIR] %s", cursor, b.Name)) + "\n"
		}

		for i, f := range m.files {
			idx := i + 1 + len(m.bundles)
			cursor = "  "
			style = systemStyle
			if m.cursor == idx {
				cursor = "> "
				style = selectedItemStyle
			}
			expContent += style.Render(fmt.Sprintf("%s- %s", cursor, f.Path)) + "\n"
		}
	}

	leftPane := paneStyle.Width(paneWidth).Height(paneHeight).Render(expContent)

	// --- Right Pane: Chat / Details ---
	chatContent := headerStyle.Render("Global Comms Frequency") + "\n\n"
	if m.err != nil {
		chatContent += lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(fmt.Sprintf("Mesh Error: %v", m.err)) + "\n\n"
	}
	
	// Print messages from bottom up
	visibleMsgs := m.messages
	maxRows := paneHeight - 6
	if len(visibleMsgs) > maxRows {
		visibleMsgs = visibleMsgs[len(visibleMsgs)-maxRows:]
	}

	for _, msg := range visibleMsgs {
		t := time.Unix(msg.Time, 0).Format("15:04")
		author := msg.AuthorID
		if len(author) > 8 {
			author = author[:8] + "..."
		}
		chatContent += fmt.Sprintf("[%s] %s %s\n",
			systemStyle.Render(t),
			msgAuthorStyle.Render(author+":"),
			msgTextStyle.Render(msg.Content),
		)
	}

	rightPane := paneStyle.Width(paneWidth).Height(paneHeight).Render(chatContent)

	// Join panes
	layout := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// Bottom Output
	footer := "\n" + m.input.View() + "\n\n" + systemStyle.Render("Type to chat. Up/Down to Navigate. Enter to Open. Ctrl+C to Quit.")

	return lipgloss.NewStyle().Padding(1, 2).Render(layout + footer)
}
