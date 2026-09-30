package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/DYLANeay/ghtidy/internal/auth"
	"github.com/DYLANeay/ghtidy/internal/github"
	"github.com/DYLANeay/ghtidy/internal/tui"
)

// overridden at build time with -ldflags "-X main.version=..."
var version = "dev"

func main() {
	// err only lives inside this if, so the name stays free for later calls
	// := stands for "declare and assign", so it creates a new variable err and assigns the result of run() to it
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run holds the real startup logic so main stays tiny and errors are handled in one place
func run() error {
	fmt.Printf("ghtidy %s\n", version)

	token, err := auth.Token()
	if err != nil {
		return err
	}

	client := github.NewClient(token)
	model := tui.NewModel(client)

	_, err = tea.NewProgram(model).Run()
	return err
}
