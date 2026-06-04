// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-01-01',
  ssr: true,

  // Static site generation: output to dist/ for embedding into the Go binary.
  nitro: {
    output: {
      publicDir: 'dist',
    },
  },

  modules: ['@element-plus/nuxt', '@nuxtjs/tailwindcss', '@nuxtjs/i18n'],

  // i18n: single-route SPA-style (no URL prefix) so the Go server can serve one
  // index.html. The locale is chosen client-side and remembered in a cookie.
  // Add a new language by dropping a file in i18n/locales/ and listing it here.
  i18n: {
    strategy: 'no_prefix',
    defaultLocale: 'en',
    locales: [
      { code: 'en', name: 'English', file: 'en.json' },
      { code: 'zh', name: '中文', file: 'zh.json' },
    ],
    detectBrowserLanguage: {
      useCookie: true,
      cookieKey: 'aprsgo_lang',
      redirectOn: 'root',
    },
  },

  // The API base. In dev, we proxy to the local Go server; in production the
  // static bundle is served by the Go server itself so the API is same-origin.
  runtimeConfig: {
    public: {
      apiBase: '/api',
    },
  },

  app: {
    head: {
      title: 'APRSGo',
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      ],
      link: [{ rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' }],
    },
  },

  // Dev-only proxy so `nuxt dev` can reach a locally running Go server.
  $development: {
    nitro: {
      devProxy: {
        '/api': { target: 'http://localhost:14501/api', changeOrigin: true },
      },
    },
  },

  devtools: { enabled: false },
})
