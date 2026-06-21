# kubectl-notify

[![CI](https://github.com/ebuildy/kubectl-notify/actions/workflows/ci.yml/badge.svg)](https://github.com/ebuildy/kubectl-notify/actions/workflows/ci.yml)
[![Release](https://github.com/ebuildy/kubectl-notify/actions/workflows/release.yml/badge.svg)](https://github.com/ebuildy/kubectl-notify/actions/workflows/release.yml)

A `kubectl` plugin that watches events from **Kubernetes** or **FluxCD** and turns
them into desktop notifications (toasts), a local web UI, and more.

> Status: early bootstrap — no business features yet.

## Installation

Once built, the binary is installed on your `PATH` as `kubectl-notify`, which lets
`kubectl` discover it as the `notify` subcommand:

```bash
make install        # copies ./bin/kubectl-notify to ~/bin
kubectl notify --help
```

Make sure `~/bin` is on your `PATH`.

## Usage

```bash
kubectl notify --help
```

Standard kubectl connection flags are supported: `--kubeconfig`, `--context`,
`--namespace`, `--token`, etc.

### Send a test notification

Exercise the desktop notifier without a cluster connection:

```bash
kubectl notify test --title "Hello" --body "It works" --level normal
```

`--level` accepts `low`, `normal`, or `critical` (critical shows an alert).

## Specs

Long-lived behavioral specs live in [`openspec/specs/`](openspec/specs/). This
table is generated from those files.

| Capability | Requirements |
|---|---|
| [datasource-events](openspec/specs/datasource-events/spec.md) | EventSource port, Event value object, Filter value object, Kubernetes events adapter |
| [desktop-notification](openspec/specs/desktop-notification/spec.md) | Notifier port, Desktop adapter, CLI test command |

## Contribute

### Prerequisites

- **Go 1.26+** (the toolchain version is pinned in [`go.mod`](go.mod); `go` will
  fetch it automatically if needed)
- **`golangci-lint`** — install with:
  ```bash
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  ```
- **`git`** (used to stamp the build version)

### Bootstrap the dev environment

```bash
# 1. Clone
git clone https://github.com/ebuildy/kubectl-notify.git
cd kubectl-notify

# 2. Download dependencies
go mod download

# 3. Build the plugin into ./bin
make build

# 4. Smoke-test
./bin/kubectl-notify --help

# 5. (optional) Install it as a kubectl plugin
make install
kubectl notify --help
```

### Common make targets

| Target          | What it does                                      |
|-----------------|---------------------------------------------------|
| `make build`    | Build the plugin binary into `./bin`              |
| `make install`  | Install the binary to `~/bin` (must be on `PATH`) |
| `make lint`     | Run `golangci-lint`                               |
| `make test`     | Run unit tests (`-race`)                          |
| `make e2e`      | Run integration / e2e tests                       |
| `make coverage` | Generate and open an HTML coverage report         |
| `make tidy`     | Tidy `go.mod` / `go.sum`                          |
| `make clean`    | Remove build artifacts                            |
| `make help`     | List all available targets                        |

### Before opening a PR

```bash
make lint build test e2e
```

This project uses [OpenSpec](https://openspec.pro/) for spec-driven development.
Non-trivial features start as a change under `openspec/changes/` — see
[`AGENTS.md`](AGENTS.md) for the full workflow and coding conventions.
