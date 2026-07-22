# Aurora Heartbeat & Database API Worker

Anonymous telemetry ingestion and database query API for Aurora Gateway.

https://aurora-heartbeat.cortexx.workers.dev/

## Deploy

1. Ensure the Hyperdrive binding `HEARTBEAT_DB` in `wrangler.toml` is configured with your Hyperdrive ID.
2. Deploy the worker to Cloudflare:
   ```bash
   npm run deploy
   ```
3. Set your administrative token:
   ```bash
   npx wrangler secret put ADMIN_KEY
   ```
   *(Enter a secure, random string. This key is used to authorize stats and query endpoints.)*

---

## API Endpoints

### 1. Ingest Heartbeat (Public)
Gateways POST here every 24 hours. Returns `200 ok` on success.
* **Method:** `POST`
* **Path:** `/api/v1/heartbeat`
* **Body:**
  ```json
  {
    "instance_id": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
    "version": "1.0.26",
    "rps_bucket": "1k-10k",
    "uptime_seconds": 86400
  }
  ```

### 2. Live Heartbeat Stats (Admin)
Get live heartbeat count and unique instance count.
* **Method:** `GET`
* **Path:** `/api/v1/stats/live`
* **Headers:** `Authorization: Bearer <ADMIN_KEY>`

### 3. Historical Daily Stats (Admin)
Get aggregated historical statistics.
* **Method:** `GET`
* **Path:** `/api/v1/stats`
* **Headers:** `Authorization: Bearer <ADMIN_KEY>`

### 4. Trigger Aggregation (Admin)
Manually trigger the daily aggregate rollup and cleanup.
* **Method:** `POST`
* **Path:** `/api/v1/stats/aggregate`
* **Headers:** `Authorization: Bearer <ADMIN_KEY>`
* *Note: This endpoint also runs automatically once a day via Cloudflare Cron Triggers.*

### 5. Execute DB Query (Admin)
Run a custom SQL `SELECT` query on the database.
* **Method:** `POST`
* **Path:** `/api/v1/query`
* **Headers:** `Authorization: Bearer <ADMIN_KEY>`
* **Body:**
  ```json
  {
    "sql": "SELECT COUNT(*) FROM anon_instance_heartbeats WHERE rps_bucket = $1",
    "params": ["1k-10k"]
  }
  ```

---

## GDPR & Privacy Compliance
* **No PII:** IP addresses, User-Agents, and locations are discarded and never logged or stored.
* **Ephemeral Raw Data:** Raw heartbeat rows are stored only for deduplication (48 hours max), after which they are deleted.
* **Aggregated Historical Data:** Only daily unique counts (e.g., `date, version, rps_bucket -> count`) are retained permanently.
