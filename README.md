# portainerctl

`portainerctl` is a Go CLI for browsing and operating Portainer resources from the terminal.

It supports:

- interactive menu-driven browsing
- container, stack, and image listing
- container actions: start, stop, restart, remove
- stack actions: start, stop, remove
- image action: remove
- one-liner commands such as `container ls`, `stack ls`, and `image ls`
- saved config for URL, username, API key, and default environment

## Build

```bash
go build -o portainerctl ./cmd/portainerctl
```

Prebuilt binaries can also be downloaded from the GitHub Releases page.

## Interactive usage

```bash
./portainerctl
./portainerctl --url https://portainer.example.com --username admin
./portainerctl --api-key YOUR_API_KEY
```

The app can save configuration to:

- macOS: `~/Library/Application Support/portainerctl/config.json`
- Linux: `~/.config/portainerctl/config.json`
- Windows: `%AppData%\portainerctl\config.json`

## One-liner commands

```bash
./portainerctl environment ls
./portainerctl container ls
./portainerctl stack ls
./portainerctl image ls
./portainerctl stack view-compose --stack homestack
```

Environment selection for one-liners:

```bash
./portainerctl container ls --env local
./portainerctl stack ls --env-id 2
./portainerctl image ls --format json
./portainerctl stack view-compose --stack-id 12 --env local > docker-compose.yml
```

## Config management

```bash
./portainerctl config view
./portainerctl config clear
./portainerctl config set --url https://portainer.example.com
./portainerctl config set --username admin
./portainerctl config set --api-key YOUR_API_KEY
./portainerctl config set --default-env local
./portainerctl config set --default-env-id 2
./portainerctl config set --clear-api-key
./portainerctl config set --clear-default-env
```

## Authentication

Supported auth modes:

- Portainer API key via `X-API-Key`
- username/password login via `/api/auth`

Auth precedence:

1. CLI flags
2. `PORTAINER_API_KEY`
3. saved config
4. interactive prompt

## Notes

- API keys are stored in plaintext in the config file if you choose to save them.
- The default environment for one-liners is learned from the interactive environment selection flow, or can be set manually with `config set`.
