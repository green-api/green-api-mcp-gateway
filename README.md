# green-api-mcp-gateway

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/green-api/green-api-mcp-gateway/main)](https://go.dev/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/green-api/green-api-mcp-gateway?label=release)](https://github.com/green-api/green-api-mcp-gateway/tags)
[![Protocol: MCP](https://img.shields.io/badge/Protocol-MCP-orange.svg)](https://modelcontextprotocol.io)
[![Go Report Card](https://goreportcard.com/badge/github.com/green-api/green-api-mcp-gateway)](https://goreportcard.com/report/github.com/green-api/green-api-mcp-gateway)

- [Документация на русском](README_RU.md) 

## Support Links

[![Support](https://img.shields.io/badge/support@green--api.com-D14836?style=for-the-badge&logo=gmail&logoColor=white)](mailto:support@green-api.com)
[![Support](https://img.shields.io/badge/Telegram-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white)](https://t.me/greenapi_support_eng_bot)
[![Support](https://img.shields.io/badge/WhatsApp-25D366?style=for-the-badge&logo=whatsapp&logoColor=white)](https://wa.me/77780739095)

## Guides & News

[![Guides](https://img.shields.io/badge/YouTube-%23FF0000.svg?style=for-the-badge&logo=YouTube&logoColor=white)](https://www.youtube.com/@greenapi-en)
[![News](https://img.shields.io/badge/Telegram-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white)](https://t.me/green_api)
[![News](https://img.shields.io/badge/WhatsApp-25D366?style=for-the-badge&logo=whatsapp&logoColor=white)](https://whatsapp.com/channel/0029VaLj6J4LNSa2B5Jx6s3h)

---

MCP gateway for GREEN-API WhatsApp. Lets AI agents (Claude Desktop, OpenClaw, Cursor, and others) send messages, manage instances, and receive notifications via WhatsApp using the [Model Context Protocol](https://modelcontextprotocol.io).

## What it does

Send text messages, files by URL, locations, contacts, polls. Forward and edit messages. Manage groups (create, fetch data, add/remove participants). Check phone numbers for WhatsApp, fetch contacts and their info. Get instance state and settings, QR code for authorization. Receive incoming notifications via polling or an HTTP receiver. Partner API for creating/deleting instances. Multi-instance support in a single gateway. Prompt templates for common scenarios (customer support, broadcasts). Prometheus metrics and OpenTelemetry.

Instructions for connecting the MCP server can be found on [our website](https://green-api.com/en/docs/integration/mcp/integration-setup/)

## Architecture

Clean Architecture (Ports & Adapters):

```
cmd/server/main.go                    — entry point, DI wiring
internal/
  domain/                             — business entities, errors, constants (zero dependencies)
  application/                        — ports (interfaces)
  infrastructure/
    auth.go                           — CredentialManager (in-memory credential store)
    config/config.go                  — YAML + env configuration
    greenapi/client.go                — WhatsApp client via Go SDK
    mcp/
      server.go                       — MCP server (stdio/sse)
      tools.go                        — MCP tools registration
      resources.go                    — MCP resources
      prompts.go                      — MCP prompts
    webhook/
      bridge.go                       — polling bridge
      receiver.go                     — HTTP receiver
```

## Build

```bash
go build -o green-api-mcp-gateway ./cmd/server
```

Requires Go 1.25+.

## Transports & endpoints

The server supports four transports, selected via `server.transport` (or the `GREEN_API_TRANSPORT` env var):

| Transport | Endpoints (relative to `host:port`)             | Use case                                                   |
|-----------|--------------------------------------------------|------------------------------------------------------------|
| `stdio`   | stdin/stdout                                     | Local CLIs (Claude Desktop, Cursor) launching the binary.  |
| `sse`     | `GET /` (stream) + `POST /message`               | Legacy MCP clients (SSE only).                             |
| `http`    | `POST/GET /mcp`                                  | Streamable HTTP (MCP 2025-03-26+) — preferred for new clients. |
| `hybrid`  | `/sse` + `/message` **and** `/mcp` on one port   | Public server: serves both legacy SSE and Streamable HTTP. |

All HTTP-based transports also expose:

- `GET /health`, `GET /healthz` — health probes (no auth)
- `GET /favicon.ico`, `GET /favicon.png` — GREEN-API icon

When `auth.mode: proxy` is enabled, the OAuth 2.0 PKCE endpoints are added too:

- `GET /.well-known/oauth-authorization-server` — RFC 8414 metadata
- `GET /.well-known/oauth-protected-resource` — RFC 9728 metadata
- `GET /authorize` — browser-facing authorization form
- `POST /token` — token exchange

## Configuration

### Via YAML

```yaml
server:
  transport: stdio   # stdio | sse | http | hybrid
  port: 8090

auth:
  mode: config       # config | proxy
  cache_ttl: 300     # seconds (proxy mode only)

green_api:
  instances:
    - id: 1101000001
      api_token: "YOUR_TOKEN"
      api_url: "https://api.green-api.com"

webhook:
  mode: polling
  polling_timeout: 20

rate_limit:
  requests_per_second: 10
  burst: 20

logging:
  level: info
  format: text
```

### Via environment variables

Environment variables take precedence over the YAML config:

The variables below cover both deployment modes. Credentials variables (`GREEN_API_INSTANCE_ID`, `GREEN_API_TOKEN`, `GREEN_API_URL`) apply only to local stdio runs with `auth.mode: config`. For HTTP transports (`sse` on `/sse` + `/message`, `http` on `/mcp`, or `hybrid`) use `auth.mode: proxy` instead — credentials arrive in HTTP headers or via the built-in OAuth flow and the env credentials are ignored.

- `GREEN_API_INSTANCE_ID` — instance ID
- `GREEN_API_TOKEN` — API token
- `GREEN_API_URL` — API base URL (defaults to `https://api.green-api.com`)
- `GREEN_API_TRANSPORT` — transport: `stdio`, `sse`, `http`, or `hybrid`
- `GREEN_API_PORT` — HTTP port (defaults to `8090`)
- `GREEN_API_BASE_URL` — public base URL (used for OAuth issuer/redirects, e.g. `https://mcp.example.com`)
- `GREEN_API_AUTH_MODE` — `config` or `proxy`
- `GREEN_API_AUTH_CACHE_TTL` — proxy-auth credential cache TTL (seconds)
- `GREEN_API_WEBHOOK_MODE` — `polling` or `receiver`
- `LOG_LEVEL` — log level
- `LOG_FORMAT` — `text` or `json`

## Run

```bash
# With environment variables
export GREEN_API_INSTANCE_ID=1101000001
export GREEN_API_TOKEN=your_token
./green-api-mcp-gateway

# With a config file
./green-api-mcp-gateway --config config.yaml
```

## Integration

### Claude Desktop (local stdio)

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "whatsapp": {
      "command": "/path/to/green-api-mcp-gateway",
      "args": ["--config", "/path/to/config.yaml"]
    }
  }
}
```

### OpenClaw

```yaml
mcp:
  servers:
    - name: whatsapp
      command: /path/to/green-api-mcp-gateway
      args: ["--config", "/path/to/config.yaml"]
```

### Docker

```bash
docker build -t green-api-mcp-gateway .

docker run --rm -i \
  -e GREEN_API_INSTANCE_ID=1101000001 \
  -e GREEN_API_TOKEN=your_token \
  green-api-mcp-gateway
```

## MCP Tools

### Messaging

[*Link to documentation*](https://green-api.com/en/docs/integration/mcp/tools/#sending-messages)

- `whatsapp_send_message` — text message
- `whatsapp_send_file` — file by URL
- `whatsapp_send_location` — location
- `whatsapp_send_contact` — contact (vCard)
- `whatsapp_send_poll` — poll
- `whatsapp_forward_messages` — forward
- `whatsapp_edit_message` — edit
- `whatsapp_delete_message` — delete

### Instance

[*Link to documentation*](https://green-api.com/en/docs/integration/mcp/tools/#instance-management)

- `whatsapp_get_state` — authorization state
- `whatsapp_get_settings` — current settings
- `whatsapp_set_settings` — update settings
- `whatsapp_get_qr` — authorization QR code
- `whatsapp_connect` — connect instance
- `whatsapp_disconnect` — disconnect instance

### Contacts

[*Link to documentation*](https://green-api.com/en/docs/integration/mcp/tools/#contacts)

- `whatsapp_get_contacts` — contact list
- `whatsapp_get_contact_info` — contact information
- `whatsapp_check_whatsapp` — phone number check

### Groups

[*Link to documentation*](https://green-api.com/en/docs/integration/mcp/tools/#groups)

- `whatsapp_create_group` — create a group
- `whatsapp_get_group_data` — group data
- `whatsapp_add_group_participant` — add a participant
- `whatsapp_remove_group_participant` — remove a participant
- `whatsapp_leave_group` — leave a group

### Notifications

[*Link to documentation*](https://green-api.com/en/docs/integration/mcp/tools/#notification-queue)

- `whatsapp_receive_notification` — receive an incoming notification (long-poll)

### Partner API

[*Link to documentation*](https://green-api.com/en/docs/integration/mcp/tools/#partner-api)

- `whatsapp_create_instance` — create an instance
- `whatsapp_delete_instance` — delete an instance
- `whatsapp_get_instances` — list instances

## MCP Resources

- `whatsapp://instance/{id}/state` — instance state
- `whatsapp://instance/{id}/settings` — instance settings

## MCP Prompts

- `whatsapp_customer_support` — template for customer support
- `whatsapp_broadcast` — template for broadcast messaging

## Dependencies

- [mcp-go](https://github.com/mark3labs/mcp-go) — MCP SDK
- [whatsapp-api-client-golang](https://github.com/green-api/whatsapp-api-client-golang) — GREEN-API Go SDK

## Development

```bash
go test ./...
gofmt -w .
GOOS=linux GOARCH=amd64 go build -o green-api-mcp-gateway-linux ./cmd/server
```

## License

MIT — see [LICENSE](LICENSE).
