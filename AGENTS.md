# AGENTS.md

Guidance for AI coding agents working on the `gobots` repository.

## Project Overview

`gobots` (module `github.com/evias/gobots`) is a Go package and CLI for
communicating with robots and edge devices over TCP/WiFi, Bluetooth (BLE),
and Serial connections. It introduces:

- **botfiles**: YAML driver files describing a device's connection and commands.
- **Wire protocol**: message format built from Go `text/template` strings
  (e.g. `{"N":102,"D1":{{.Speed}},"D2":{{.Direction}}}`), transmitted as
  size-prefixed payloads (`[uint32 big-endian length][payload]`).
- **Flows**: executable sequences of commands defined in YAML (under active development).

## Repository Layout

- `botfile/` — Core domain: `Driver`, `Flow`, `WireConfig`/`Message` (`Wireable`), config parsing.
- `robot/` — `Robot` connection wrapper implementing `IRobot`; `robot.Simulator` for driver testing.
  - `robot/conn/` — Transport implementations (`tcp`, `serial`, `ble`/`bluetooth`) behind `apiconn.Transport` and `NewTransport` factory.
  - `robot/errors/` — Robot error types.
- `cmd/` — CLI entrypoint (`main.go`), Cobra commands in `cmd/internal/`:
  `run`, `console`, `exec`, `connect`, and `driver` (`info`, `list`, `test`).
  Terminal UI helpers live in `cmd/internal/tui/`.
- `drivers/` — Sample botfiles (e.g. `drivers/ELEGOO/smartcar-v4.yaml`, `drivers/robot.yaml`).
- `doc.go` — Package documentation; keep it in sync with architectural changes.

## Build & Test Commands

```bash
go build ./...   # compile
go vet ./...     # static checks
go test ./...    # run tests (currently only robot/conn and robot/errors)
gofmt -l .       # formatting check (std gofmt, no gofumpt/golangci config present)
```

There is no Makefile or CI configuration; run the commands above directly.
Go 1.26 is required (see `go.mod`). The `tinygo.org/x/bluetooth` dependency
may need CGO/toolchain prerequisites on some platforms.

## Conventions

- Language: Go, stdlib `log/slog` for logging; Cobra + pflag for the CLI.
- Interfaces are named `I...` (e.g. `IRobot`, `botfile.Driver`); concrete types
  declare interface compliance via `var _ IRobot = (*Robot)(nil)` assertions.
- Aliased imports for internal packages: `apiconn "github.com/evias/gobots/robot/conn"`,
  `apierr "github.com/evias/gobots/robot/errors"`.
- Errors follow the package-local pattern in `robot/errors` and are wrapped
  with `%w` and lowercase contextual prefixes (e.g. `fmt.Errorf("msg: render: %w", err)`).
- YAML botfiles use kebab/lowercase keys; see `botfile/config.go` for the
  expected schema (`name`, `brand`, `connection`, `commands`, `setup`, `shutdown`, `fields`).
- Wire messages: payloads are capped at `botfile.MaxPayload` (1 MB) and must
  marshal via `MarshalBinary`/`UnmarshalBinary` (size-prefixed uint32 BE).
- Commit messages follow Conventional Commits (`feat:`, `fix:`, `refactor:`, `docs:`).

## Testing Notes

- Tests use `stretchr/testify`; place `_test.go` files next to the code.
- Prefer testing through `robot.Simulator` and sample drivers in `drivers/`
  for driver-related changes; `gobots driver test` offers a dry-run mode
  (`--count 0` / no transmit) for sanity-checking command parsing.
- Network/serial-dependent tests should not require real hardware.

## Agent Rules

- Do not commit directly to `main`; use feature branches (e.g. `feat/...`).
- Keep `doc.go` and `README.md` updated when adding commands, config keys, or packages.
- Do not add new heavy dependencies without need; the project favors stdlib.
