const releaseRoot = '/srv/seoblog/current'

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
        NITRO_HOST: '127.0.0.1',
        NITRO_PORT: '3000',
        NUXT_API_BASE_URL: 'http://127.0.0.1:8080'
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
        SEOBLOG_ENV: 'production',
        SEOBLOG_HTTP_ADDR: '127.0.0.1:8080'
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
        SEOBLOG_ENV: 'production'
      }
    }
  ]
}
