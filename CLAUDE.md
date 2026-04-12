# txid-cli

## Language
- Respond in Korean (한국어로 응답)

## Project Overview
Go CLI for interacting with api.txid.uk. Bearer token-based authentication. Single-file main.go with no external dependencies.

## Tech Stack
- **Language**: Go 1.22 (stdlib only: net/http, encoding/json, os)
- **Storage**: `~/.config/txid/token` (0600 permissions)

## Structure
```
main.go          # Entire CLI (commands, HTTP helper, token storage)
go.mod           # No deps
README.md
```

## Commands
- `txid auth <token>` - Save API token (starts with txid_)
- `txid whoami` - Current user info
- `txid logout` - Delete saved token
- `txid search <q>` - Unified search (posts/glossary/users)
- `txid notif [--unread]` - Recent notifications
- `txid sub [channel]` - List or toggle subscriptions
- `txid channels` - All notification channels
- `txid token list/create <name>` - API token management

## Environment
- `TXID_API` - Override API URL (default https://api.txid.uk)

## Build
```bash
go build -o txid .
```

## Auth Flow
1. User creates token via https://api.txid.uk/admin.html or POST /auth/tokens
2. `txid auth txid_xxx` saves to ~/.config/txid/token
3. All subsequent commands use `Authorization: Bearer txid_xxx`

## GitHub
https://github.com/bc1qwerty/txid-cli
