// API response types mirroring the Go model.Return* structs.

export interface ApiEnvelope<T> {
  code: number
  msg: string
  data: T
}

// Memory values are reported in MEGABYTES (float) by the Go backend, except
// the count/duration fields. Do NOT pass these to formatBytes.
export interface Memory {
  total: number // MB
  used: number // MB
  self: number // MB (this process RSS)
  total_allocated: number // MB
  current_allocated: number // MB
  malloc: number // count
  free: number // count
  heap: number // MB
  num_gc: number // count
  pause_total_sec: number // seconds
  last_gc: string
  last_pause_total_sec: number
  next_gc: number // MB
  lookups: number
}

export interface ServerInfo {
  admin: string
  email: string
  os: string
  arch: string
  id: string
  software: string
  codename: string
  version: string
  now: string
  uptime: number
  model: string
  percent: number
  memory: Memory
}

export interface Uplink {
  id: string
  mode: string
  protocol: string
  host: string
  real_addr: string
  port: number
  server_id: string
  server: string
  up: boolean
  uptime: string
  last: string
  packet_rx: number
  packet_rx_dup: number
  packet_rx_err: number
  packet_rx_rate: number
  packet_tx: number
  packet_tx_rate: number
  bytes_rx: number
  bytes_rx_rate: number
  bytes_tx: number
  bytes_tx_rate: number
}

export interface Peer {
  name: string
  id: string
  addr: string
}

export interface Listener {
  name: string
  mode: string
  protocol: string
  host: string
  port: number
  filter: string
  online_client: number
  peak_client: number
  packet_rx: number
  packet_rx_rate: number
  packet_tx: number
  packet_tx_rate: number
  bytes_rx: number
  bytes_rx_rate: number
  bytes_tx: number
  bytes_tx_rate: number
}

export interface ClientInfo {
  at: string
  port: number
  id: string
  verified: boolean
  addr: string
  uptime: string
  last: string
  last_tx: string
  software: string
  version: string
  filter: string
  out_q: number
  msg_rcpts: number
  packet_rx: number
  packet_rx_dup: number
  packet_rx_err: number
  packet_rx_rate: number
  packet_tx: number
  packet_tx_rate: number
  bytes_rx: number
  bytes_rx_rate: number
  bytes_tx: number
  bytes_tx_rate: number
}

export interface Totals {
  clients: number
  packet_rx: number
  packet_tx: number
  bytes_rx: number
  bytes_tx: number
  packet_rx_rate: number
  packet_tx_rate: number
  bytes_rx_rate: number
  bytes_tx_rate: number
  dupes: number
  position_cache: number
}

export interface Status {
  msg: string
  server: ServerInfo
  totals: Totals
  uplink: Uplink | null
  peers: Peer[]
  listeners: Listener[]
  clients: ClientInfo[]
}

export type Series = [number, number][]

export interface Stats {
  msg: string
  memory: Series
  uplink_packet_rx: Series
  uplink_packet_tx: Series
  uplink_bytes_rx: Series
  uplink_bytes_tx: Series
}
