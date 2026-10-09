package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DYLANeay/ghtidy/internal/github"
)

// shortSHALength is how many characters of a commit hash are displayed
const shortSHALength = 7

// openDetail switches to the detail screen of the repo under the cursor
func (m Model) openDetail() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		return m, nil
	}
	m.detailRepo = m.filtered[m.cursor]
	m.detail = github.RepoDetail{}
	m.detailLoading = true
	m.detailErr = nil
	m.mode = modeDetail
	return m, fetchDetail(m.service, m.detailRepo)
}

// handleDetailLoaded stores the fetched detail unless the user already left the screen
func (m Model) handleDetailLoaded(msg detailLoadedMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeDetail || msg.detail.Repo.FullName != m.detailRepo.FullName {
		return m, nil
	}
	m.detail = msg.detail
	m.detailLoading = false
	return m, nil
}

// handleDetailErr shows the failure unless the user already left the screen
func (m Model) handleDetailErr(msg detailErrMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeDetail || msg.fullName != m.detailRepo.FullName {
		return m, nil
	}
	m.detailErr = msg.err
	m.detailLoading = false
	return m, nil
}

// updateDetail handles keys on the detail screen
func (m Model) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return m.closeDetail(), nil
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

// closeDetail goes back to the list, cursor, filter and selection stay as they were
func (m Model) closeDetail() Model {
	m.detailRepo = github.Repo{}
	m.detail = github.RepoDetail{}
	m.detailLoading = false
	m.detailErr = nil
	m.mode = modeList
	return m
}

// viewDetail renders the detail screen
func (m Model) viewDetail() string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s\n\n", m.detailRepo.FullName)

	switch {
	case m.detailLoading:
		b.WriteString("loading details...\n")
	case m.detailErr != nil:
		fmt.Fprintf(&b, "error: %v\n", m.detailErr)
	default:
		b.WriteString(detailBody(m.detail))
	}

	b.WriteString("\nesc or q: back | ctrl+c: quit\n")
	return b.String()
}

// detailBody renders metadata, urls, dates, languages and commits
func detailBody(detail github.RepoDetail) string {
	var b strings.Builder
	repo := detail.Repo

	fmt.Fprintf(&b, "%s\n\n", descriptionOrDefault(repo.Description))
	fmt.Fprintf(&b, "visibility: %s | archived: %s | fork: %s\n",
		repo.Visibility, yesNo(repo.Archived), yesNo(repo.Fork))
	fmt.Fprintf(&b, "web: %s\n", repo.HTMLURL)
	fmt.Fprintf(&b, "https: %s\n", detail.CloneURL)
	fmt.Fprintf(&b, "ssh: %s\n\n", detail.SSHURL)
	fmt.Fprintf(&b, "created: %s\n", formatDate(repo.CreatedAt))
	fmt.Fprintf(&b, "last push: %s\n\n", formatDate(repo.PushedAt))

	b.WriteString("languages:\n")
	b.WriteString(languageLines(detail))
	b.WriteString("\nlatest commits:\n")
	b.WriteString(commitLines(detail.Commits))
	return b.String()
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

// formatDate prints a day, or a dash when the api gave no date
func formatDate(date time.Time) string {
	if date.IsZero() {
		return "-"
	}
	return date.Format("2006-01-02")
}

func descriptionOrDefault(description string) string {
	if description == "" {
		return "no description"
	}
	return description
}

func shortSHA(sha string) string {
	if len(sha) <= shortSHALength {
		return sha
	}
	return sha[:shortSHALength]
}

// languageLines lists each language with its share of the code
func languageLines(detail github.RepoDetail) string {
	if len(detail.Languages) == 0 {
		return "  none detected\n"
	}

	total := 0
	for _, language := range detail.Languages {
		total += language.Bytes
	}

	var b strings.Builder
	for _, language := range detail.Languages {
		percent := float64(language.Bytes) * 100 / float64(total)
		fmt.Fprintf(&b, "  %s %.1f%%\n", language.Name, percent)
	}
	return b.String()
}

// commitLines lists one commit per line: short hash, date, author, message
func commitLines(commits []github.Commit) string {
	if len(commits) == 0 {
		return "  no commits yet\n"
	}

	var b strings.Builder
	for _, commit := range commits {
		fmt.Fprintf(&b, "  %s %s %s: %s\n",
			shortSHA(commit.SHA), formatDate(commit.Date), commit.Author, commit.Message)
	}
	return b.String()
}
