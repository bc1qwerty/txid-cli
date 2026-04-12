# txid-cli

Command-line interface for interacting with api.txid.uk.

## Install

```bash
go install github.com/bc1qwerty/txid-cli@latest
# or build from source
git clone https://github.com/bc1qwerty/txid-cli
cd txid-cli && go build -o txid .
```

## Auth

Create an API token via the admin page at https://api.txid.uk/admin.html or via the web client, then:

```bash
txid auth txid_xxxxxxxxxxxx
```

Token is saved to `~/.config/txid/token` with 0600 permissions.

## Commands

```
txid whoami              Show current user
txid search <query>      Unified search (posts, glossary, users)
txid channels            List available notification channels
txid sub                 Show your subscriptions
txid sub <channel>       Toggle subscription
txid notif               List notifications
txid notif --unread      Only unread
txid token list          List your API tokens
txid token create <name> Create a new token
txid logout              Delete saved token
```

## Environment

- `TXID_API` — override API URL (default: `https://api.txid.uk`)
