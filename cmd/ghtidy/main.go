package main

import (
	"fmt"
	"os"

	"github.com/DYLANeay/ghtidy/internal/auth"
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

	// we dont print the token for obvious sec reason
	fmt.Println("token found, length:", len(token))
	return nil
}
