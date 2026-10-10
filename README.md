# ghtidy

[![CI](https://github.com/DYLANeay/ghtidy/actions/workflows/ci.yml/badge.svg)](https://github.com/DYLANeay/ghtidy/actions/workflows/ci.yml)

A terminal UI to tidy up your GitHub repositories: archive, delete and change visibility in bulk.

Select as many repositories as you want, then apply one action to all of them in a single run. Every action shows a confirmation step and a final report of what succeeded, what was skipped and what failed.

## Features

- list all your repositories, public and private
- filter the list by name
- multi-select with select-all and clear
- detail view: description, visibility, dates, languages and latest commits
- bulk actions: archive, toggle visibility, delete
- typed confirmation for destructive actions
- per-repository report after each run

## Demo

<!-- TODO: add a GIF recorded with vhs -->

## Install

### go install

```sh
go install github.com/DYLANeay/ghtidy/cmd/ghtidy@latest
```

### Build from source

```sh
git clone https://github.com/DYLANeay/ghtidy.git
cd ghtidy
go build ./cmd/ghtidy
```

### Prebuilt binaries

Grab the latest release for your platform from the [releases page](https://github.com/DYLANeay/ghtidy/releases).

## Authentication

ghtidy reads the token from the `GITHUB_TOKEN` environment variable, and falls back to the token stored by the `gh` CLI:

```sh
export GITHUB_TOKEN=your_token_here
ghtidy
```

or log in with `gh auth login`.

### Token scopes

| Action | Required scope |
| --- | --- |
| archive | `repo` |
| toggle visibility | `repo` |
| delete | `delete_repo` |

Grant the scopes for the actions you plan to use. Deleting a repository is the only destructive action and needs its own scope.

## Keybindings

### Repository list

| Key | Action |
| --- | --- |
| `j` / `k` / arrows | move the cursor |
| `space` | toggle selection |
| `a` | select all visible |
| `A` | clear selection |
| `/` | filter by name |
| `esc` | clear the filter |
| `enter` | open detail view |
| `r` | archive selected repositories |
| `v` | toggle visibility of selected repositories |
| `d` | delete selected repositories |
| `q` / `ctrl+c` | quit |

### Detail view

| Key | Action |
| --- | --- |
| `esc` / `q` | back to the list |
| `ctrl+c` | quit |

### Confirmations

| Action | How to confirm |
| --- | --- |
| archive / toggle visibility | `y` to confirm, `n` or `esc` to cancel |
| delete | type `delete` exactly to confirm |

## Development

```sh
make test   # go test ./...
make vet    # go vet ./...
make lint   # golangci-lint run
make build  # go build ./cmd/ghtidy
```

## License

[MIT](LICENSE)