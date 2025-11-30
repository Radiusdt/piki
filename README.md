# Vector-DSP

**Demand-Side Platform** для закупки мобильного трафика с интеграцией MMP (AppsFlyer, Adjust, Singular).

## Архитектура

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              TRAFFIC SOURCES                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   S2S Partners                              RTB Exchanges                    │
│   ┌───────────┐                             ┌───────────┐                   │
│   │ Partner A │ ──GET /s2s/partner_a/ad──▶  │  SSP #1   │ ──BidRequest──▶   │
│   └───────────┘                             └───────────┘                   │
│   ┌───────────┐                             ┌───────────┐                   │
│   │ Partner B │ ──GET /s2s/partner_b/ad──▶  │  SSP #2   │ ──BidRequest──▶   │
│   └───────────┘                             └───────────┘                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              VECTOR-DSP                                      │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                    │
│   │  S2S API    │    │  RTB Bidder │    │  Tracking   │                    │
│   │  Handler    │    │             │    │  Service    │                    │
│   └──────┬──────┘    └──────┬──────┘    └──────┬──────┘                    │
│          │                  │                  │                           │
│          └────────┬─────────┴──────────────────┘                           │
│                   │                                                         │
│          ┌────────▼────────┐                                               │
│          │ Campaign Store  │◀── PostgreSQL                                  │
│          │ (campaigns,     │                                                │
│          │  sources, MMP)  │                                                │
│          └────────┬────────┘                                               │
│                   │                                                         │
│          ┌────────▼────────┐                                               │
│          │  Event Store    │◀── ClickHouse                                  │
│          │ (clicks, imps,  │                                                │
│          │  conversions)   │                                                │
│          └─────────────────┘                                               │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                              MMP INTEGRATION                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   User clicks ad                                                            │
│        │                                                                    │
│        ▼                                                                    │
│   ┌─────────────────────┐                                                  │
│   │ Vector-DSP          │                                                  │
│   │ /track/click        │ ──────┐                                          │
│   │ (log click,         │       │                                          │
│   │  replace macros)    │       ▼                                          │
│   └─────────────────────┘   ┌─────────────────────┐                        │
│                             │ MMP Click URL       │                        │
│                             │ (AppsFlyer/Adjust)  │                        │
│                             └──────────┬──────────┘                        │
│                                        │                                   │
│                                        ▼                                   │
│                             ┌─────────────────────┐                        │
│                             │ App Store           │                        │
│                             │ (user installs app) │                        │
│                             └──────────┬──────────┘                        │
│                                        │                                   │
│                                        ▼                                   │
│   ┌─────────────────────┐   ┌─────────────────────┐                        │
│   │ Vector-DSP          │◀──│ MMP Postback        │                        │
│   │ /postback/appsflyer │   │ (install, events)   │                        │
│   │ (log conversion,    │   └─────────────────────┘                        │
│   │  send to S2S)       │                                                  │
│   └─────────────────────┘                                                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Быстрый старт

### Требования

- Docker & Docker Compose
- Go 1.21+ (для локальной разработки)

### Запуск

```bash
# Клонировать репозиторий
git clone https://github.com/your-org/vector-dsp.git
cd vector-dsp

# Запустить все сервисы
docker-compose up -d

# Проверить статус
curl http://localhost:8080/health
```

### Доступные сервисы

| Сервис | URL | Описание |
|--------|-----|----------|
| Vector-DSP API | http://localhost:8080 | Main API |
| Prometheus | http://localhost:9091 | Metrics |
| Grafana | http://localhost:3000 | Dashboards (admin/admin) |
| PostgreSQL | localhost:5432 | Config DB |
| ClickHouse | http://localhost:8123 | Event DB |
| Redis | localhost:6379 | Cache |

## API Endpoints

### Tracking

```bash
# Click tracking (redirects to MMP)
GET /track/click?cid={campaign_id}&cr={creative_id}&src={source_id}&st=s2s&gaid={gaid}&sub1={sub1}

# View tracking (returns 1x1 pixel)
GET /track/view?cid={campaign_id}&cr={creative_id}&src={source_id}&st=s2s&gaid={gaid}
```

### Postbacks (from MMPs)

```bash
# AppsFlyer
GET /postback/appsflyer?click_id={clickid}&event={event_name}&revenue={event_revenue}&currency={currency}&gaid={advertising_id}

# Adjust
GET /postback/adjust?click_id={click_id}&event_token={event_token}&revenue={revenue}&currency={currency}&gps_adid={gps_adid}

# Singular
GET /postback/singular?click_id={click_id}&event={event}&revenue={revenue}&aifa={aifa}

# Generic
GET /postback?click_id={click_id}&event={event}&revenue={revenue}
```

### S2S Partners

```bash
# Get ad (returns creative + tracking URLs)
GET /s2s/{partner_name}/ad?country=US&os=android&device_type=phone&gaid={gaid}&sub1={sub1}&token={api_token}

# Response:
{
  "success": true,
  "campaign_id": "camp-123",
  "app_bundle": "com.example.app",
  "creative": {
    "id": "cr-456",
    "type": "banner",
    "url": "https://cdn.example.com/banner.jpg",
    "w": 320,
    "h": 50
  },
  "click_url": "https://track.vector-dsp.com/track/click?cid=camp-123&cr=cr-456&src=partner-id&st=s2s&gaid=xxx",
  "view_url": "https://track.vector-dsp.com/track/view?cid=camp-123&cr=cr-456&src=partner-id&st=s2s&gaid=xxx",
  "payout": 0.50
}
```

### OpenRTB 2.5

```bash
# Bid request
POST /openrtb2/bid
Content-Type: application/json

{
  "id": "request-123",
  "imp": [{"id": "1", "banner": {"w": 320, "h": 50}, "bidfloor": 0.01}],
  "app": {"bundle": "com.publisher.app"},
  "device": {"ifa": "xxx", "os": "android", "geo": {"country": "US"}}
}

# Win notification
GET /openrtb2/win?campaign_id={campaign_id}&price={win_price}&imp_id={imp_id}

# Loss notification
GET /openrtb2/loss?campaign_id={campaign_id}&reason={reason_code}
```

### Admin API

```bash
# Campaigns
GET    /api/campaigns
POST   /api/campaigns
GET    /api/campaigns/{id}
PUT    /api/campaigns/{id}

# Advertisers
GET    /api/advertisers
POST   /api/advertisers
GET    /api/advertisers/{id}

# S2S Sources
GET    /api/sources/s2s
POST   /api/sources/s2s
GET    /api/sources/s2s/{id}

# RTB Sources
GET    /api/sources/rtb
POST   /api/sources/rtb
GET    /api/sources/rtb/{id}

# Reports
GET    /api/reports/campaigns
GET    /api/reports/sources
GET    /api/reports/geo?campaign_id={id}
GET    /api/reports/time-series?campaign_id={id}&start_date=2025-01-01&end_date=2025-01-31
```

## Интеграция с MMP

### 1. Настройка в MMP (AppsFlyer)

1. Войти в AppsFlyer Dashboard
2. Перейти в Integrated Partners → Add Partner
3. Найти или создать "Vector-DSP" как партнёра
4. Скопировать **Click URL** и **Impression URL**

**Click URL пример:**
```
https://app.appsflyer.com/com.your.app?pid=vector_dsp&c={campaign_name}&clickid={click_id}&advertising_id={gaid}&idfa={idfa}&af_siteid={source_id}&af_c_id={campaign_id}
```

**Impression URL пример:**
```
https://impression.appsflyer.com/com.your.app?pid=vector_dsp&c={campaign_name}&clickid={impression_id}&advertising_id={gaid}
```

### 2. Настройка Postback в MMP

Настроить postback URL на:
```
https://track.vector-dsp.com/postback/appsflyer?click_id={clickid}&event={event_name}&revenue={event_revenue}&currency={currency}&gaid={advertising_id}
```

### 3. Создание кампании в Vector-DSP

```bash
curl -X POST http://localhost:8080/api/campaigns \
  -H "Content-Type: application/json" \
  -d '{
    "id": "camp-001",
    "name": "My App Install Campaign",
    "advertiser_id": "adv-001",
    "status": "active",
    "app_bundle": "com.your.app",
    "app_name": "Your App",
    "app_store_url": "https://play.google.com/store/apps/details?id=com.your.app",
    "mmp": {
      "type": "appsflyer",
      "click_url": "https://app.appsflyer.com/com.your.app?pid=vector_dsp&c={campaign_name}&clickid={click_id}&advertising_id={gaid}",
      "view_url": "https://impression.appsflyer.com/com.your.app?pid=vector_dsp&clickid={impression_id}&advertising_id={gaid}",
      "postback_events": ["install", "registration", "purchase"]
    },
    "payout_type": "fixed",
    "payout_amount": 0.50,
    "payout_event": "install",
    "line_items": [{
      "id": "li-001",
      "name": "US Android",
      "status": "active",
      "bid_type": "cpi",
      "bid_amount": 1.50,
      "targeting": {
        "countries": ["US"],
        "os": ["android"],
        "device_types": ["phone"]
      },
      "creatives": [{
        "id": "cr-001",
        "format": "banner",
        "w": 320,
        "h": 50,
        "adm_template": "https://cdn.example.com/banners/app_banner.jpg"
      }]
    }]
  }'
```

### 4. Создание S2S партнёра

```bash
curl -X POST http://localhost:8080/api/sources/s2s \
  -H "Content-Type: application/json" \
  -d '{
    "id": "src-001",
    "name": "Partner Network A",
    "internal_name": "partner_a",
    "status": "active",
    "api_token": "secret-token-123",
    "postback_url": "https://partner-a.com/postback?click_id={click_id}&event={event}&payout={payout}",
    "postback_method": "GET",
    "postback_events": ["install", "registration"],
    "default_payout": 0.30,
    "default_payout_type": "fixed"
  }'
```

## Поддерживаемые макросы

| Макрос | Описание |
|--------|----------|
| `{click_id}` / `{clickid}` | Уникальный ID клика |
| `{impression_id}` | Уникальный ID показа |
| `{campaign_id}` | ID кампании |
| `{campaign_name}` / `{campaign}` | Название кампании |
| `{creative_id}` | ID креатива |
| `{source_id}` / `{source_name}` | ID/имя источника |
| `{gaid}` / `{advertising_id}` | Google Advertising ID |
| `{idfa}` | Apple IDFA |
| `{ip}` | IP адрес пользователя |
| `{user_agent}` / `{ua}` | User-Agent |
| `{country}` / `{geo_country}` | Страна (ISO 2) |
| `{city}` / `{geo_city}` | Город |
| `{region}` / `{geo_region}` | Регион |
| `{device_os}` / `{os}` | ОС (android/ios) |
| `{device_type}` | Тип устройства |
| `{device_make}` | Производитель |
| `{device_model}` | Модель |
| `{sub1}` - `{sub5}` | Sub ID параметры |
| `{timestamp}` / `{ts}` | Unix timestamp |
| `{event}` | Название события |
| `{revenue}` | Доход |
| `{currency}` | Валюта |
| `{payout}` | Выплата |

## Конфигурация

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VECTOR_DSP_HTTP_ADDR` | `:8080` | HTTP server address |
| `VECTOR_DSP_ENV` | `development` | Environment (development/production) |
| `VECTOR_DSP_DB_HOST` | `localhost` | PostgreSQL host |
| `VECTOR_DSP_DB_PORT` | `5432` | PostgreSQL port |
| `VECTOR_DSP_DB_USER` | `vectordsp` | PostgreSQL user |
| `VECTOR_DSP_DB_PASSWORD` | `vectordsp_secret` | PostgreSQL password |
| `VECTOR_DSP_DB_NAME` | `vectordsp` | PostgreSQL database |
| `VECTOR_DSP_REDIS_ADDR` | `localhost:6379` | Redis address |
| `VECTOR_DSP_AUTH_ENABLED` | `true` | Enable API authentication |
| `VECTOR_DSP_API_KEY_MASTER` | - | Master API key (required if auth enabled) |
| `VECTOR_DSP_TRACKING_BASE_URL` | `https://track.vector-dsp.com` | Base URL for tracking links |
| `VECTOR_DSP_GEO_ENABLED` | `false` | Enable GeoIP detection |
| `VECTOR_DSP_GEO_DB_PATH` | `/app/data/GeoLite2-City.mmdb` | MaxMind GeoIP database path |

## Структура проекта

```
vector-dsp/
├── cmd/
│   └── vector-dsp/
│       └── main.go
├── internal/
│   ├── config/           # Configuration
│   ├── database/         # Database connections
│   ├── dsp/              # Core DSP logic
│   │   ├── bid.go        # Bid service
│   │   ├── campaign.go   # Campaign service
│   │   ├── tracking.go   # Tracking service
│   │   ├── postback.go   # Postback handler
│   │   └── source_service.go
│   ├── httpserver/       # HTTP handlers
│   ├── metrics/          # Prometheus metrics
│   ├── models/           # Data models
│   ├── storage/          # Storage interfaces
│   └── targeting/        # Targeting engine
├── migrations/           # Database migrations
├── deploy/               # Deployment configs
├── docker-compose.yml
├── Dockerfile
└── README.md
```

## License

MIT License
