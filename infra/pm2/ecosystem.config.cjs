const releaseRoot = process.env.SEOBLOG_RELEASE_ROOT || '/srv/seoblog/current'
const sharedRoot = process.env.SEOBLOG_SHARED_ROOT || '/srv/seoblog/shared'
const apiAddr = process.env.SEOBLOG_HTTP_ADDR || `127.0.0.1:${process.env.SEOBLOG_API_PORT || '8080'}`
const adminHost = process.env.NITRO_HOST || '127.0.0.1'
const adminPort = process.env.NITRO_PORT || process.env.SEOBLOG_ADMIN_PORT || '3000'
const dbPath = process.env.SEOBLOG_DB_PATH || `${sharedRoot}/seoblog.db`

module.exports = {
  apps: [
    {
      name: 'seoblog-admin',
      namespace: 'seoblog',
      cwd: releaseRoot,
      script: './admin/.output/server/index.mjs',
      interpreter: 'node',
      exec_mode: 'fork',
      instances: 1,
      watch: false,
      autorestart: true,
      restart_delay: 2000,
      min_uptime: '10s',
      max_restarts: 10,
      kill_timeout: 15000,
      max_memory_restart: '768M',
      env_production: {
        NODE_ENV: 'production',
        NITRO_HOST: adminHost,
        NITRO_PORT: adminPort,
        NUXT_API_BASE_URL: process.env.NUXT_API_BASE_URL || `http://${apiAddr}`
      }
    },
    {
      name: 'seoblog-api',
      namespace: 'seoblog',
      cwd: releaseRoot,
      script: './backend/api',
      exec_mode: 'fork',
      instances: 1,
      watch: false,
      autorestart: true,
      restart_delay: 2000,
      min_uptime: '10s',
      max_restarts: 10,
      kill_timeout: 15000,
      env_production: {
        SEOBLOG_ENV: process.env.SEOBLOG_ENV || 'production',
        SEOBLOG_HTTP_ADDR: apiAddr,
        SEOBLOG_DB_PATH: dbPath,
        SEOBLOG_DEV_AUTH: process.env.SEOBLOG_DEV_AUTH || 'false',
        SEOBLOG_TRUSTED_PROXIES: process.env.SEOBLOG_TRUSTED_PROXIES || '127.0.0.1',
        SEOBLOG_ADMIN_PUBLIC_URL: process.env.SEOBLOG_ADMIN_PUBLIC_URL || '',
        SEOBLOG_SMTP_ADDR: process.env.SEOBLOG_SMTP_ADDR || '',
        SEOBLOG_SMTP_USERNAME: process.env.SEOBLOG_SMTP_USERNAME || '',
        SEOBLOG_SMTP_PASSWORD: process.env.SEOBLOG_SMTP_PASSWORD || '',
        SEOBLOG_SMTP_REQUIRE_STARTTLS: process.env.SEOBLOG_SMTP_REQUIRE_STARTTLS || 'false',
        SEOBLOG_SMTP_FROM: process.env.SEOBLOG_SMTP_FROM || '',
        SEOBLOG_SMTP_FROM_NAME: process.env.SEOBLOG_SMTP_FROM_NAME || '',
        SEOBLOG_WEBHOOK_ENCRYPTION_KEY: process.env.SEOBLOG_WEBHOOK_ENCRYPTION_KEY || '',
        SEOBLOG_WEBHOOK_ALLOWED_HOSTS: process.env.SEOBLOG_WEBHOOK_ALLOWED_HOSTS || ''
      }
    },
    {
      name: 'seoblog-worker',
      namespace: 'seoblog',
      cwd: releaseRoot,
      script: './backend/worker',
      exec_mode: 'fork',
      instances: 1,
      watch: false,
      autorestart: true,
      restart_delay: 2000,
      min_uptime: '10s',
      max_restarts: 10,
      kill_timeout: 30000,
      env_production: {
        SEOBLOG_ENV: process.env.SEOBLOG_ENV || 'production',
        SEOBLOG_HTTP_ADDR: apiAddr,
        SEOBLOG_WORKER_METRICS_ADDR: process.env.SEOBLOG_WORKER_METRICS_ADDR || '127.0.0.1:9092',
        SEOBLOG_DB_PATH: dbPath,
        SEOBLOG_DEV_AUTH: process.env.SEOBLOG_DEV_AUTH || 'false',
        SEOBLOG_TRUSTED_PROXIES: process.env.SEOBLOG_TRUSTED_PROXIES || '127.0.0.1',
        SEOBLOG_ADMIN_PUBLIC_URL: process.env.SEOBLOG_ADMIN_PUBLIC_URL || '',
        SEOBLOG_SMTP_ADDR: process.env.SEOBLOG_SMTP_ADDR || '',
        SEOBLOG_SMTP_USERNAME: process.env.SEOBLOG_SMTP_USERNAME || '',
        SEOBLOG_SMTP_PASSWORD: process.env.SEOBLOG_SMTP_PASSWORD || '',
        SEOBLOG_SMTP_REQUIRE_STARTTLS: process.env.SEOBLOG_SMTP_REQUIRE_STARTTLS || 'false',
        SEOBLOG_SMTP_FROM: process.env.SEOBLOG_SMTP_FROM || '',
        SEOBLOG_SMTP_FROM_NAME: process.env.SEOBLOG_SMTP_FROM_NAME || '',
        SEOBLOG_WEBHOOK_ENCRYPTION_KEY: process.env.SEOBLOG_WEBHOOK_ENCRYPTION_KEY || '',
        SEOBLOG_WEBHOOK_ALLOWED_HOSTS: process.env.SEOBLOG_WEBHOOK_ALLOWED_HOSTS || '',
        SEOBLOG_AI_PROVIDER: process.env.SEOBLOG_AI_PROVIDER || 'openai-compatible',
        SEOBLOG_AI_BASE_URL: process.env.SEOBLOG_AI_BASE_URL || '',
        SEOBLOG_AI_API_KEY: process.env.SEOBLOG_AI_API_KEY || '',
        SEOBLOG_AI_MODEL: process.env.SEOBLOG_AI_MODEL || '',
        SEOBLOG_AI_TIMEOUT: process.env.SEOBLOG_AI_TIMEOUT || '90s',
        SEOBLOG_AI_MAX_INPUT_BYTES: process.env.SEOBLOG_AI_MAX_INPUT_BYTES || '262144',
        SEOBLOG_AI_MAX_OUTPUT_TOKENS: process.env.SEOBLOG_AI_MAX_OUTPUT_TOKENS || '4096'
      }
    }
  ]
}
