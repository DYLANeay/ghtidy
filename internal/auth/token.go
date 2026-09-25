// Package auth finds the github token used by the app
package auth

import (
	"errors"
	"os"
)

// ErrNoToken is returned when no token can be found anywhere
var ErrNoToken = errors.New("no github token found, set GITHUB_TOKEN or run `gh auth login`")

// Token returns the github token used to call the api
func Token() (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return token, nil
	}
	return "", ErrNoToken
}
