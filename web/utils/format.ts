// Formatting helpers shared across components.

export function formatBytes(n: number): string {
  if (!n || n < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatRate(n: number): string {
  return `${formatBytes(n)}/s`
}

export function formatNumber(n: number): string {
  return new Intl.NumberFormat().format(n ?? 0)
}

export function formatUptime(seconds: number): string {
  if (!seconds || seconds < 0) return '0s'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  const parts: string[] = []
  if (d) parts.push(`${d}d`)
  if (h) parts.push(`${h}h`)
  if (m) parts.push(`${m}m`)
  parts.push(`${s}s`)
  return parts.join(' ')
}

export function formatTime(iso: string): string {
  if (!iso) return '-'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString()
}

// timeAgo returns a short relative time like "12s ago".
export function timeAgo(iso: string): string {
  if (!iso) return '-'
  const then = new Date(iso).getTime()
  if (isNaN(then)) return '-'
  const secs = Math.max(0, Math.floor((Date.now() - then) / 1000))
  return `${formatUptime(secs)} ago`
}

// formatMB formats a value already expressed in megabytes.
export function formatMB(mb: number): string {
  if (!mb || mb < 0) return '0 MB'
  if (mb >= 1024) return `${(mb / 1024).toFixed(2)} GB`
  return `${mb.toFixed(2)} MB`
}

// serverSoftwareNames are the APRS-IS server packages that host their own
// status dashboard on the HTTP/status port. Matched as whole names
// (case-insensitive) to avoid false positives like "APRSdroid" ~ "aprsd".
const serverSoftwareNames = new Set(['aprsc', 'aprsd', 'aprsgo', 'javaprssrvr'])

// isServerSoftware reports whether a software name denotes an APRS-IS server
// (vs. an ordinary client). The software field is the bare program name (the
// version is carried separately), so an exact, whole-name match is correct.
export function isServerSoftware(software: string): boolean {
  if (!software) return false
  // Use the first whitespace-delimited token in case a version slipped in.
  const name = software.trim().split(/\s+/)[0].toLowerCase()
  return serverSoftwareNames.has(name)
}

// hostOnly strips a trailing :port from an address while preserving IPv6
// bracket notation. "192.0.2.1:10152" -> "192.0.2.1"; "[::1]:8080" -> "[::1]".
export function hostOnly(addr: string): string {
  if (!addr) return ''
  if (addr.includes(']:')) {
    return addr.substring(0, addr.lastIndexOf(']:') + 1)
  }
  if (addr.includes(':') && !addr.startsWith('[')) {
    const i = addr.lastIndexOf(':')
    const port = addr.substring(i + 1)
    if (/^\d+$/.test(port)) {
      let h = addr.substring(0, i)
      // bare IPv6 without brackets -> add them
      if (h.includes(':')) h = `[${h}]`
      return h
    }
  }
  return addr
}

// serverStatusLink builds the URL to another server's status dashboard
// (port 14501), given any address form.
export function serverStatusLink(addr: string): string {
  return `http://${hostOnly(addr)}:14501`
}

// callsignLink builds a map lookup link for an ordinary station callsign.
export function callsignLink(call: string): string {
  return `https://aprs.cn/callsign/${encodeURIComponent(call)}`
}

