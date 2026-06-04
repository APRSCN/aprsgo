import type { ApiEnvelope, Status, Stats } from '~/types/api'

// useApi exposes typed fetchers for the status and stats endpoints. All calls
// are client-side (the page is a static bundle served by the Go server), so we
// use a plain fetch against the same-origin /api base.
export function useApi() {
  const base = useRuntimeConfig().public.apiBase

  async function get<T>(path: string): Promise<T> {
    const res = await $fetch<ApiEnvelope<T>>(`${base}${path}`)
    return res.data
  }

  return {
    getStatus: () => get<Status>('/status'),
    getStats: () => get<Stats>('/stats'),
  }
}
