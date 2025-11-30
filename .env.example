# Vector-DSP Configuration
# Copy this file to .env and adjust values

# ===========================================
# SERVER
# ===========================================
VECTOR_DSP_HTTP_ADDR=:8080
VECTOR_DSP_ENV=development  # development | staging | production

# ===========================================
# DATABASE (PostgreSQL)
# ===========================================
VECTOR_DSP_DB_HOST=localhost
VECTOR_DSP_DB_PORT=5432
VECTOR_DSP_DB_USER=vectordsp
VECTOR_DSP_DB_PASSWORD=vectordsp_secret
VECTOR_DSP_DB_NAME=vectordsp
VECTOR_DSP_DB_SSLMODE=disable
VECTOR_DSP_DB_MAX_CONNS=25
VECTOR_DSP_DB_MIN_CONNS=5

# ===========================================
# REDIS
# ===========================================
VECTOR_DSP_REDIS_ADDR=localhost:6379
VECTOR_DSP_REDIS_PASSWORD=
VECTOR_DSP_REDIS_DB=0

# ===========================================
# AUTHENTICATION
# ===========================================
# Master API key for admin operations (generate a secure one!)
VECTOR_DSP_API_KEY_MASTER=your-secure-master-key-here

# Enable/disable auth (disable only for local development)
VECTOR_DSP_AUTH_ENABLED=true

# Endpoints that don't require auth (comma-separated)
VECTOR_DSP_AUTH_SKIP_PATHS=/health,/openrtb2/bid,/openrtb2/win,/openrtb2/loss

# ===========================================
# RATE LIMITING
# ===========================================
VECTOR_DSP_RATE_LIMIT_ENABLED=true
VECTOR_DSP_RATE_LIMIT_RPS=1000           # requests per second (bid endpoint)
VECTOR_DSP_RATE_LIMIT_BURST=100          # burst allowance
VECTOR_DSP_RATE_LIMIT_MGMT_RPS=100       # management API rps
VECTOR_DSP_RATE_LIMIT_MGMT_BURST=20

# ===========================================
# LOGGING
# ===========================================
VECTOR_DSP_LOG_LEVEL=info    # debug | info | warn | error
VECTOR_DSP_LOG_FORMAT=json   # json | console

# ===========================================
# GRACEFUL SHUTDOWN
# ===========================================
VECTOR_DSP_SHUTDOWN_TIMEOUT=30s
