// Package auth finds the github token used by the app
package auth

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrNoToken is returned when no token can be found anywhere
var ErrNoToken = errors.New("no github token found, set GITHUB_TOKEN or run `gh auth login`")

// Token returns the github token used to call the api,
// looking at GITHUB_TOKEN first and falling back to the gh cli
func Token() (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return token, nil
	}
	// if we do not find it in the env
	token, err := tokenFromGhCli()
	if err != nil {
		return "", ErrNoToken
	}
	return token, nil
}

// tokenFromGhCli asks the gh cli for the token it's logged in with
func tokenFromGhCli() (string, error) {
	output, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("run gh auth token: %w", err)
	}

	token := strings.TrimSpace(string(output))
	if token == "" {
		return "", ErrNoToken
	}
	return token, nil
}
