# Manual: Building and Distributing Your Go CLI

A practical guide to turning your Go app into a **single binary** and installing it on multiple Linux machines.

---

## Table of Contents

1. [Prerequisites](#1-prerequisites)
2. [Recommended project structure](#2-recommended-project-structure)
3. [Basic build](#3-basic-build)
4. [Static and optimized binary](#4-static-and-optimized-binary)
5. [Injecting the version into the binary](#5-injecting-the-version-into-the-binary)
6. [Cross-compilation (other architectures)](#6-cross-compilation-other-architectures)
7. [Automating with a Makefile](#7-automating-with-a-makefile)
8. [Checksums and verification](#8-checksums-and-verification)
9. [Installing on machines](#9-installing-on-machines)
10. [Automating releases with GoReleaser](#10-automating-releases-with-goreleaser)
11. [CI/CD with GitHub Actions](#11-cicd-with-github-actions)
12. [Troubleshooting](#12-troubleshooting)
13. [Final checklist](#13-final-checklist)

---

## 1. Prerequisites

Install Go on the **build** machine (the target machines **do not** need Go).

```bash
# Check whether it is already installed
go version

# Manual installation (example; check the current version at https://go.dev/dl/)
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz

# Add to PATH (put this in ~/.bashrc or ~/.zshrc)
export PATH=$PATH:/usr/local/go/bin
```

Confirm:

```bash
go version
```

---

## 2. Recommended project structure

```
mycli/
├── go.mod
├── go.sum
├── main.go              # or cmd/mycli/main.go
├── internal/            # app internal code
├── Makefile
├── .goreleaser.yaml     # optional
└── README.md
```

If the module is not initialized yet:

```bash
go mod init github.com/your-username/mycli
go mod tidy
```

> `go mod tidy` downloads dependencies and removes unused ones. Always run it before building.

---

## 3. Basic build

At the project root:

```bash
go build -o mycli .
```

If `main.go` is in `cmd/mycli/`:

```bash
go build -o mycli ./cmd/mycli
```

Test:

```bash
./mycli --help
```

This binary works, but it is still not ideal for distribution (it may depend on glibc and is larger than necessary).

---

## 4. Static and optimized binary

Use this command to generate a **self-contained** and **smaller** binary:

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o mycli .
```

| Option             | What it does                                                                                             |
| ------------------ | -------------------------------------------------------------------------------------------------------- |
| `CGO_ENABLED=0`    | Disables CGO. The binary becomes **100% static**, with no dependency on glibc. It runs on any Linux distro. |
| `-trimpath`        | Removes local paths from the binary (reproducible and cleaner build).                                     |
| `-ldflags="-s -w"` | Removes the symbol table and debug information. Greatly reduces the size.                                 |

### Checking whether it is static

```bash
file mycli
# expected: ... statically linked ...

ldd mycli
# expected: "not a dynamic executable"
```

### Optional compression with UPX

Reduces the size even further (30-60%), with a small startup-time cost:

```bash
sudo apt install upx
upx --best --lzma mycli
```

> Warning: some antivirus and security tools flag UPX-compressed binaries as suspicious. In a corporate environment, consider whether it is worth it.

---

## 5. Injecting the version into the binary

Useful for the `mycli --version` command. In the code, declare variables:

```go
package main

import "fmt"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	fmt.Printf("mycli %s (commit %s, build %s)\n", version, commit, date)
}
```

Build by injecting the values:

```bash
VERSION=1.0.0
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

CGO_ENABLED=0 go build -trimpath \
  -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
  -o mycli .
```

If the variables are in another package, use the full path:
`-X github.com/your-username/mycli/internal/version.Version=${VERSION}`

---

## 6. Cross-compilation (other architectures)

Go compiles for other platforms **without installing anything extra**. Just set `GOOS` and `GOARCH`.

```bash
# Linux x86_64 (servers and regular PCs)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/mycli-linux-amd64 .

# Linux ARM64 (Raspberry Pi 4/5, ARM servers, AWS Graviton)
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/mycli-linux-arm64 .

# Linux ARM 32-bit (older Raspberry Pis)
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags="-s -w" -o dist/mycli-linux-armv7 .

# Linux 32-bit (rare)
CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -trimpath -ldflags="-s -w" -o dist/mycli-linux-386 .
```

### How to find the target machine's architecture

```bash
uname -m
```

| Output of `uname -m` | Use `GOARCH`        |
| -------------------- | ------------------- |
| `x86_64`             | `amd64`             |
| `aarch64` / `arm64`  | `arm64`             |
| `armv7l`             | `arm` with `GOARM=7` |
| `i686` / `i386`      | `386`               |

### See all supported combinations

```bash
go tool dist list
```

---

## 7. Automating with a Makefile

Create a `Makefile` at the root:

```makefile
APP      := mycli
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
BUILD    := CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)"

.PHONY: build build-all clean test checksums

build:
	$(BUILD) -o dist/$(APP) .

build-all: clean
	GOOS=linux GOARCH=amd64         $(BUILD) -o dist/$(APP)-linux-amd64 .
	GOOS=linux GOARCH=arm64         $(BUILD) -o dist/$(APP)-linux-arm64 .
	GOOS=linux GOARCH=arm GOARM=7   $(BUILD) -o dist/$(APP)-linux-armv7 .

checksums:
	cd dist && sha256sum $(APP)-* > checksums.txt

test:
	go test ./...

clean:
	rm -rf dist
```

Usage:

```bash
make build          # builds only for the current machine
make build-all      # builds for all architectures
make checksums      # generates the SHA256 hashes
make test           # runs the tests
```

> Important: the Makefile requires **TAB** (not spaces) for command indentation.

---

## 8. Checksums and verification

Generate hashes to ensure the integrity of the distributed binaries:

```bash
cd dist
sha256sum mycli-* > checksums.txt
```

On the target machine, after downloading:

```bash
sha256sum -c checksums.txt --ignore-missing
```

---

## 9. Installing on machines

### Option A: manual copy (SCP)

```bash
# From the build machine to the server
scp dist/mycli-linux-amd64 user@server:/tmp/mycli

# On the server
ssh user@server
sudo install -m 755 /tmp/mycli /usr/local/bin/mycli
mycli --version
```

The `install -m 755` command copies the file and already sets the execution permission.

### Option B: direct download via curl (GitHub Releases)

```bash
curl -L -o mycli https://github.com/your-username/mycli/releases/download/v1.0.0/mycli-linux-amd64
chmod +x mycli
sudo mv mycli /usr/local/bin/
```

### Option C: universal install script

Save it as `install.sh` and host it in the repository:

```bash
#!/usr/bin/env bash
set -euo pipefail

REPO="your-username/mycli"
APP="mycli"
VERSION="${VERSION:-latest}"

# Detect architecture
case "$(uname -m)" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  armv7l)        ARCH="armv7" ;;
  *) echo "Unsupported architecture: $(uname -m)"; exit 1 ;;
esac

# Build the URL
if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${APP}-linux-${ARCH}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${APP}-linux-${ARCH}"
fi

echo "Downloading ${APP} (${ARCH})..."
TMP="$(mktemp)"
curl -fsSL "$URL" -o "$TMP"

sudo install -m 755 "$TMP" "/usr/local/bin/${APP}"
rm -f "$TMP"

echo "Installed: $(${APP} --version)"
```

Usage on any machine:

```bash
curl -fsSL https://raw.githubusercontent.com/your-username/mycli/main/install.sh | bash
```

### Option D: install without sudo (user only)

```bash
mkdir -p ~/.local/bin
cp mycli ~/.local/bin/
chmod +x ~/.local/bin/mycli

# Make sure it is in PATH (add this to ~/.bashrc)
export PATH="$HOME/.local/bin:$PATH"
```

### Option E: install directly via Go (if the machine already has Go)

```bash
go install github.com/your-username/mycli@latest
```

The binary goes to `$(go env GOPATH)/bin` (usually `~/go/bin`).

### Installing on multiple machines at once (Ansible)

```yaml
- hosts: servers
  become: true
  tasks:
    - name: Install mycli
      ansible.builtin.copy:
        src: dist/mycli-linux-amd64
        dest: /usr/local/bin/mycli
        mode: "0755"
```

```bash
ansible-playbook -i inventory.ini install.yml
```

---

## 10. Automating releases with GoReleaser

**GoReleaser** does everything at once: it builds for multiple platforms, generates checksums, creates the GitHub release, and can generate `.deb` and `.rpm` packages.

### Installation

```bash
go install github.com/goreleaser/goreleaser/v2@latest
# or via snap: sudo snap install goreleaser --classic
```

### Configuration (`.goreleaser.yaml`)

```bash
goreleaser init
```

Or create it manually:

```yaml
version: 2

project_name: mycli

builds:
  - main: .
    binary: mycli
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
      - arm
    goarm:
      - "7"
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}

archives:
  - formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: checksums.txt

nfpms:
  - package_name: mycli
    formats:
      - deb
      - rpm
    maintainer: "Your Name <you@email.com>"
    description: "Description of my CLI"
    license: MIT
```

### Testing locally (without publishing)

```bash
goreleaser release --snapshot --clean
```

The files are placed in `dist/`.

### Publishing a version

```bash
git tag -a v1.0.0 -m "First version"
git push origin v1.0.0

export GITHUB_TOKEN="your_token"
goreleaser release --clean
```

With the `.deb` packages, installing on Debian/Ubuntu becomes:

```bash
sudo dpkg -i mycli_1.0.0_linux_amd64.deb
```

---

## 11. CI/CD with GitHub Actions

Create `.github/workflows/release.yml` to publish automatically when a tag is created:

```yaml
name: release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: stable

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Final flow: you run `git tag v1.0.1 && git push --tags`, and GitHub builds and publishes everything by itself.

---

## 12. Troubleshooting

| Problem                                             | Likely cause                     | Solution                                                                                                               |
| --------------------------------------------------- | -------------------------------- | ---------------------------------------------------------------------------------------------------------------------- |
| `GLIBC_2.xx not found` when running on another machine | Binary compiled with CGO enabled | Compile with `CGO_ENABLED=0`                                                                                           |
| `exec format error`                                 | Wrong architecture               | Check `uname -m` and use the correct `GOARCH`                                                                          |
| `Permission denied` when executing                  | No execution permission          | `chmod +x mycli`                                                                                                       |
| `command not found` after installing                | Directory outside `PATH`         | Move it to `/usr/local/bin` or adjust `PATH`                                                                           |
| `cannot find package`                               | Missing dependencies             | `go mod tidy`                                                                                                          |
| Binary too large                                    | Debug symbols included           | Use `-ldflags="-s -w"` (and optionally UPX)                                                                            |
| I need CGO (e.g., SQLite with `mattn/go-sqlite3`)   | Library depends on C             | Switch to a pure Go alternative (e.g., `modernc.org/sqlite`) or use `musl-gcc` with `-ldflags '-extldflags "-static"'` |
| DNS error in a static binary                        | Pure Go vs. system resolver      | Use `-tags netgo` or set `GODEBUG=netdns=go`                                                                           |

---

## 13. Final checklist

Before distributing a version:

- [ ] `go mod tidy` run
- [ ] `go vet ./...` with no warnings
- [ ] `go test ./...` passing
- [ ] Built with `CGO_ENABLED=0`
- [ ] `-trimpath` and `-ldflags="-s -w"` flags applied
- [ ] Version injected and `mycli --version` working
- [ ] Tested on at least one clean machine (without Go installed)
- [ ] Binaries generated for all required architectures
- [ ] `checksums.txt` generated
- [ ] Git tag created (`v1.0.0`)
- [ ] Release published and install script tested

---

## Quick reference

```bash
# Production build (current machine)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o mycli .

# Build for ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o mycli-arm64 .

# Install
sudo install -m 755 mycli /usr/local/bin/mycli

# Full release
git tag v1.0.0 && git push --tags && goreleaser release --clean
```
