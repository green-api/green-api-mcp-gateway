# green-api-mcp-gateway

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/github/go-mod/go-version/green-api/green-api-mcp-gateway/main)](https://go.dev/)
[![GitHub tag (latest by date)](https://img.shields.io/github/v/tag/green-api/green-api-mcp-gateway?label=release)](https://github.com/green-api/green-api-mcp-gateway/tags)
[![Protocol: MCP](https://img.shields.io/badge/Protocol-MCP-orange.svg)](https://modelcontextprotocol.io)
[![Go Report Card](https://goreportcard.com/badge/github.com/green-api/green-api-mcp-gateway)](https://goreportcard.com/report/github.com/green-api/green-api-mcp-gateway)

## Поддержка

[![Support](https://img.shields.io/badge/support@green--api.com-D14836?style=for-the-badge&logo=gmail&logoColor=white)](mailto:support@green-api.com)
[![Support](https://img.shields.io/badge/Telegram-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white)](https://t.me/greenapi_support_ru_bot)
[![Support](https://img.shields.io/badge/WhatsApp-25D366?style=for-the-badge&logo=whatsapp&logoColor=white)](https://wa.me/77780739095)

## Руководства и новости

[![Guides](https://img.shields.io/badge/YouTube-%23FF0000.svg?style=for-the-badge&logo=YouTube&logoColor=white)](https://www.youtube.com/@green-api)
[![News](https://img.shields.io/badge/Telegram-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white)](https://t.me/green_api)
[![News](https://img.shields.io/badge/WhatsApp-25D366?style=for-the-badge&logo=whatsapp&logoColor=white)](https://whatsapp.com/channel/0029VaHUM5TBA1f7cG29nO1C)

---

MCP-шлюз для GREEN-API WhatsApp. Позволяет агентам ИИ (Claude Desktop, OpenClaw, Cursor и другие) отправлять сообщения, управлять инстансами и получать уведомления через WhatsApp, используя [Model Context Protocol](https://modelcontextprotocol.io).

## Функциональные возможности

Отправка текстовых сообщений, файлов по URL, геолокаций, контактов и опросов. Пересылка и редактирование сообщений. Создание групп, получение данных о группах, добавление и удаление участников. Проверка номеров телефонов на наличие WhatsApp, получение списка контактов и информации о них. Получение статуса и настроек инстанса, генерация QR-кода для авторизации. Получение входящих уведомлений через метод длинных опросов (polling) или HTTP-вебхуки (receiver). Создание и удаление инстансов. Поддержка работы с несколькими инстансами в рамках одного шлюза. Готовые шаблоны для распространенных сценариев (клиентская поддержка, рассылки). Метрики Prometheus и интеграция с OpenTelemetry.

Документация к REST API находится по [ссылке](https://green-api.com/docs/api/).   
С инструкцией по подключению MCP сервера можно ознакомиться [на нашем сайте](https://green-api.com/docs/integration/mcp/integration-setup/)

## Архитектура

Чистая Архитектура (Порты и Адаптеры):

```
cmd/server/main.go           — точка входа, связывание зависимостей (DI)
internal/
  domain/                    — бизнес-сущности, ошибки, константы (ноль зависимостей)
  application/               — порты (интерфейсы)
  infrastructure/
    auth.go                  — CredentialManager (хранилище учетных данных в оперативной памяти)
    config/config.go         — конфигурация YAML + env
    greenapi/client.go       — WhatsApp-клиент через Go SDK
    mcp/
      server.go              — MCP-сервер (stdio/sse)
      tools.go               — регистрация инструментов (tools) MCP
      resources.go           — ресурсы MCP
      prompts.go             — промпты MCP
    webhook/
      bridge.go              — мост для длинных опросов (polling)
      receiver.go            — HTTP-приемник (receiver)
```

## Сборка

```bash
go build -o green-api-mcp-gateway ./cmd/server
```

Требуется Go 1.25+.

## Транспорты и эндпоинты

Сервер поддерживает четыре транспорта, выбираемых через параметр `server.transport` (или переменную окружения `GREEN_API_TRANSPORT`):

| Транспорт | Эндпоинты (относительно `host:port`)            | Сценарий использования                                                   |
|-----------|--------------------------------------------------|--------------------------------------------------------------------------|
| `stdio`   | stdin/stdout                                     | Локальные CLI (Claude Desktop, Cursor), запускающие бинарный файл.       |
| `sse`     | `GET /` (stream) + `POST /message`               | Устаревшие MCP-клиенты (только SSE).                                     |
| `http`    | `POST/GET /mcp`                                  | Streamable HTTP (MCP 2025-03-26+) — предпочтительно для новых клиентов.  |
| `hybrid`  | `/sse` + `/message` **и** `/mcp` на одном порту  | Public server: serves both legacy SSE and Streamable HTTP.               |

Все транспорты на базе HTTP также предоставляют:

- `GET /health`, `GET /healthz` — проверки работоспособности (без авторизации)
- `GET /favicon.ico`, `GET /favicon.png` — иконка GREEN-API

Когда включен режим `auth.mode: proxy`, также добавляются эндпоинты OAuth 2.0 PKCE:

- `GET /.well-known/oauth-authorization-server` — метаданные RFC 8414
- `GET /.well-known/oauth-protected-resource` — метаданные RFC 9728
- `GET /authorize` — форма авторизации для браузера
- `POST /token` — обмен токенов

## Конфигурация

### Через YAML

```yaml
server:
  transport: stdio   # stdio | sse | http | hybrid
  port: 8090

auth:
  mode: config       # config | proxy
  cache_ttl: 300     # секунды (только для режима proxy)

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

### Через переменные окружения

Переменные окружения имеют приоритет над конфигурацией из YAML:

Переменные ниже охватывают оба режима развертывания. Переменные учетных данных (`GREEN_API_INSTANCE_ID`, `GREEN_API_TOKEN`, `GREEN_API_URL`) применяются только при локальном запуске через `stdio` с режимом `auth.mode: config`. Для HTTP-транспортов (`sse` на `/sse` + `/message`, `http` на `/mcp` или `hybrid`) используйте режим `auth.mode: proxy` — в этом случае учетные данные передаются в HTTP-заголовках или через встроенный процесс OAuth, а переменные окружения для учетных данных игнорируются.

- `GREEN_API_INSTANCE_ID` — ID инстанса
- `GREEN_API_TOKEN` — API-токен
- `GREEN_API_URL` — базовый URL API (по умолчанию `https://api.green-api.com`)
- `GREEN_API_TRANSPORT` — транспорт: `stdio`, `sse`, `http` или `hybrid`
- `GREEN_API_PORT` — HTTP-порт (по умолчанию `8090`)
- `GREEN_API_BASE_URL` — публичный базовый URL (используется для эмиттера/редиректов OAuth, например, `https://mcp.example.com`)
- `GREEN_API_AUTH_MODE` — `config` или `proxy`
- `GREEN_API_AUTH_CACHE_TTL` — время жизни (TTL) кэша учетных данных для proxy-авторизации (в секундах)
- `GREEN_API_WEBHOOK_MODE` — `polling` или `receiver`
- `LOG_LEVEL` — уровень логирования
- `LOG_FORMAT` — `text` или `json`

## Запуск

```bash
# С переменными окружения
export GREEN_API_INSTANCE_ID=1101000001
export GREEN_API_TOKEN=your_token
./green-api-mcp-gateway

# С файлом конфигурации
./green-api-mcp-gateway --config config.yaml
```

## Интеграция

### Claude Desktop (локальный stdio)

Добавьте в `claude_desktop_config.json`:

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


## Инструменты MCP

### Сообщения

[*Ссылка на документацию*](https://green-api.com/docs/integration/mcp/tools/#_3)

- `whatsapp_send_message` — текстовое сообщение
- `whatsapp_send_file` — файл по URL
- `whatsapp_send_location` — геолокация
- `whatsapp_send_contact` — контакт (vCard)
- `whatsapp_send_poll` — опрос
- `whatsapp_forward_messages` — пересылка сообщений
- `whatsapp_edit_message` — редактирование сообщения
- `whatsapp_delete_message` — удаление сообщения

### Инстанс

[*Ссылка на документацию*](https://green-api.com/docs/integration/mcp/tools/#_2)

- `whatsapp_get_state` — статус авторизации
- `whatsapp_get_settings` — текущие настройки
- `whatsapp_set_settings` — обновление настроек
- `whatsapp_get_qr` — QR-код для авторизации
- `whatsapp_connect` — подключение инстанса
- `whatsapp_disconnect` — отключение инстанса

### Контакты

[*Ссылка на документацию*](https://green-api.com/docs/integration/mcp/tools/#_7)

- `whatsapp_get_contacts` — список контактов
- `whatsapp_get_contact_info` — информация о контакте
- `whatsapp_check_whatsapp` — проверка номера телефона на наличие WhatsApp

### Группы

[*Ссылка на документацию*](https://green-api.com/docs/integration/mcp/tools/#_9)

- `whatsapp_create_group` — создание группы
- `whatsapp_get_group_data` — данные группы
- `whatsapp_add_group_participant` — добавление участника в группу
- `whatsapp_remove_group_participant` — удаление участника из группы
- `whatsapp_leave_group` — выход из группы

### Уведомления

[*Ссылка на документацию*](https://green-api.com/docs/integration/mcp/tools/#_6)

- `whatsapp_receive_notification` — получение входящего уведомления (long-poll)

### Партнёрские методы

[*Ссылка на документацию*](https://green-api.com/docs/integration/mcp/tools/#api)

- `whatsapp_create_instance` — создание инстанса
- `whatsapp_delete_instance` — удаление инстанса
- `whatsapp_get_instances` — список инстансов


## Ресурсы MCP

- `whatsapp://instance/{id}/state` — статус инстанса
- `whatsapp://instance/{id}/settings` — настройки инстанса


## Промпты MCP

- `whatsapp_customer_support` — шаблон для клиентской поддержки
- `whatsapp_broadcast` — шаблон для массовых рассылок


## Зависимости (Dependencies)

- [mcp-go](https://github.com/mark3labs/mcp-go) — MCP SDK
- [whatsapp-api-client-golang](https://github.com/green-api/whatsapp-api-client-golang) — GREEN-API Go SDK


## Разработка (Development)

```bash
go test ./...
gofmt -w .
GOOS=linux GOARCH=amd64 go build -o green-api-mcp-gateway-linux ./cmd/server

```


## Лицензия (License)

MIT — см. [LICENSE](LICENSE).