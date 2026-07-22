/// <reference types="@cloudflare/workers-types" />

import { Client } from 'pg'

interface Env {
  HEARTBEAT_DB: Hyperdrive
  ADMIN_KEY?: string
}

function json(data: unknown, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: {
      'Content-Type': 'application/json',
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
      'Access-Control-Allow-Headers': 'Authorization, Content-Type',
    },
  })
}

async function withDb<T>(connStr: string, fn: (c: Client) => Promise<T>): Promise<T> {
  const c = new Client(connStr)
  await c.connect()
  try { return await fn(c) } finally { await c.end() }
}

// ── Routes ──────────────────────────────────────────

async function handleHeartbeat(req: Request, env: Env): Promise<Response> {
  let body: any
  try { body = await req.json() } catch { return json({ error: 'invalid json' }, 400) }
  if (!body?.instance_id) return json({ error: 'missing instance_id' }, 400)

  try {
    await withDb(env.HEARTBEAT_DB.connectionString, c =>
      c.query(
        `INSERT INTO anon_instance_heartbeats
         (instance_id, version, rps_bucket, uptime_seconds)
         VALUES ($1, $2, $3, $4)`,
        [body.instance_id, body.version ?? 'unknown', body.rps_bucket ?? 'unknown', body.uptime_seconds ?? 0]
      )
    )
  } catch (err) {
    console.error('heartbeat insert failed', err)
  }
  return json({ ok: true })
}

async function handleHealth(): Promise<Response> {
  return json({ status: 'ok' })
}

async function handleStats(env: Env): Promise<Response> {
  try {
    const rows = await withDb(env.HEARTBEAT_DB.connectionString, c =>
      c.query(`
        SELECT date, version, rps_bucket, unique_instances, total_pings
        FROM anon_daily_stats ORDER BY date DESC, version
      `)
    )
    return json(rows.rows)
  } catch (err) {
    return json({ error: 'database error', detail: String(err) }, 500)
  }
}

async function handleLive(env: Env): Promise<Response> {
  try {
    const total = await withDb(env.HEARTBEAT_DB.connectionString, c =>
      c.query('SELECT COUNT(*)::int AS count FROM anon_instance_heartbeats')
    )
    const unique = await withDb(env.HEARTBEAT_DB.connectionString, c =>
      c.query('SELECT COUNT(DISTINCT instance_id)::int AS count FROM anon_instance_heartbeats')
    )
    return json({ total_heartbeats: total.rows[0].count, unique_instances_live: unique.rows[0].count })
  } catch (err) {
    return json({ error: 'database error', detail: String(err) }, 500)
  }
}

async function handleAggregate(env: Env): Promise<Response> {
  await withDb(env.HEARTBEAT_DB.connectionString, c =>
    c.query('SELECT anon_aggregate_and_cleanup()')
  )
  return json({ ok: true })
}

async function handleQuery(req: Request, env: Env): Promise<Response> {
  let body: any
  try { body = await req.json() } catch { return json({ error: 'invalid json' }, 400) }

  const sql = body?.sql
  if (!sql || typeof sql !== 'string') return json({ error: 'missing sql' }, 400)
  if (!/^\s*SELECT\b/i.test(sql)) return json({ error: 'only SELECT allowed' }, 403)

  const params = Array.isArray(body?.params) ? body.params : []
  const rows = await withDb(env.HEARTBEAT_DB.connectionString, c => c.query(sql, params))
  return json({ rows: rows.rows, count: rows.rowCount })
}

// ── Router ──────────────────────────────────────────

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url)
    const path = url.pathname
    const method = request.method

    // CORS preflight
    if (method === 'OPTIONS') {
      return new Response(null, {
        headers: {
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
          'Access-Control-Allow-Headers': 'Authorization, Content-Type',
          'Access-Control-Max-Age': '86400',
        },
      })
    }

    // Public — no auth
    if (method === 'POST' && path === '/api/v1/heartbeat') return handleHeartbeat(request, env)
    if (method === 'GET' && path === '/health') return handleHealth()

    // Admin — requires ADMIN_KEY
    const authHeader = request.headers.get('authorization') ?? ''
    const token = authHeader.replace(/^Bearer\s+/i, '')
    if (!token || token !== env.ADMIN_KEY) {
      return json({ error: 'unauthorized' }, 401)
    }

    switch (`${method} ${path}`) {
      case 'GET /api/v1/stats':          return handleStats(env)
      case 'GET /api/v1/stats/live':     return handleLive(env)
      case 'POST /api/v1/stats/aggregate': return handleAggregate(env)
      case 'POST /api/v1/query':         return handleQuery(request, env)
      default:                           return json({ error: 'not found' }, 404)
    }
  },

  // Cron: runs daily at 03:00 UTC — aggregate & cleanup
  async scheduled(_controller: ScheduledController, env: Env): Promise<void> {
    await withDb(env.HEARTBEAT_DB.connectionString, c =>
      c.query('SELECT anon_aggregate_and_cleanup()')
    )
    console.log('Daily aggregate & cleanup completed')
  },
}
