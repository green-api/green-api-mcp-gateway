# GREEN-API WhatsApp Skill

OpenClaw skill for sending and receiving WhatsApp messages via [GREEN-API](https://green-api.com).

## What it does

Gives OpenClaw access to 35+ WhatsApp tools: messaging, file sharing, polls, contacts, groups, chat history, instance management, and more — all through the GREEN-API MCP gateway.

## Prerequisites

1. A GREEN-API account — sign up at [console.green-api.com](https://console.green-api.com)
2. A created and authorized instance (with QR code or phone number)
3. The GREEN-API MCP gateway server running (see below)

## Setup

### 1. Install the skill

```bash
openclaw skills install green-api
```

### 2. Connect the MCP server

Add to your OpenClaw config (`~/.openclaw/openclaw.json`):

```json
{
  "mcpServers": {
    "green-api": {
      "url": "https://<your-host>/mcp"
    }
  }
}
```

If you deploy the server in hybrid mode, legacy SSE-only clients can use `https://<your-host>/sse` instead.

### 3. Authorize

On the first request, the MCP client will open a browser tab to `https://<your-host>/authorize`. Paste your **Instance ID** and **API Token** (from [console.green-api.com](https://console.green-api.com)) into the form. The bearer token issued by the OAuth flow is bound to those credentials for 24 hours — you won't be prompted again until it expires.

### 4. Start chatting

Once authorized, just describe what you want:

> "Send 'Hello!' to +7 987 654 3210 from instance 1234567"

## Links

- [GREEN-API Documentation](https://green-api.com/en/docs/)
- [GREEN-API Console](https://console.green-api.com)
