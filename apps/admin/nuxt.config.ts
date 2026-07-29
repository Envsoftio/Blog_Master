const apiBaseUrl = process.env.NUXT_API_BASE_URL || 'http://localhost:8080'

export default defineNuxtConfig({
  compatibilityDate: '2026-07-29',
  modules: ['@nuxt/ui'],
  devtools: { enabled: process.env.NODE_ENV !== 'production' },
  css: ['~/assets/css/main.css'],
  routeRules: {
    '/api/**': {
      proxy: `${apiBaseUrl}/api/**`,
      headers: {
        'cache-control': 'private, no-store',
        'x-robots-tag': 'noindex, nofollow'
      }
    },
    '/content/**': {
      proxy: `${apiBaseUrl}/content/**`,
      headers: {
        'cache-control': 'private, no-store'
      }
    },
    '/**': {
      headers: {
        'cache-control': 'private, no-store',
        'x-robots-tag': 'noindex, nofollow'
      }
    },
    '/_nuxt/**': {
      headers: {
        'cache-control': 'public, max-age=31536000, immutable'
      }
    }
  },
  runtimeConfig: {
    apiBaseUrl,
    public: {
      appName: 'SEO Blog CMS'
    }
  }
})
