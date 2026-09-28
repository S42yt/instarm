# instarm

Modern Instagram activity cleanup CLI written in Go using Playwright.

## Requirements

- Go 1.24+
- A normal Instagram account
- Playwright Chromium

## Setup

Install Chromium:

```bash
go run github.com/mxschmitt/playwright-go/cmd/playwright@v0.6201.1 install chromium
go mod tidy
```

## First login

Run once with a visible browser:

```bash
go run . -mode likes -headless=false
```

Log into Instagram normally. The browser profile is persisted in `data/browser/`.

After that:

```bash
go run . -mode likes
go run . -mode comments
go run . -mode all
```

## Options

```text
-mode likes|comments|all
-headless=true|false
-batch 20
-dry-run
-profile data/browser
```

Use `-dry-run` to select items without performing the destructive action.

## Build

```bash
go build -o instarm .
```

The tool uses Instagram's normal web UI and does not attempt to bypass authentication, CAPTCHAs, rate limits, or other platform protections.
