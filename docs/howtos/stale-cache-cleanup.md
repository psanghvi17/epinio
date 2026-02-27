# Stale cache cleanup (maintenance)

Application build and rebuild use a Persistent Volume (PV) for cache. If the cache is invalidated or stale, it should be rebuilt like during the initial application process. The maintenance cleanup feature finds and optionally deletes **stale cache PVCs** (unused for a configurable number of days) so that rebuilds get a clean state.

## API

- **POST** `/api/v1/maintenance/cleanup-stale-caches` — request body (optional): `staleDays`, `checkAppExists`, `dryRun`
- **GET** `/api/v1/maintenance/cleanup-stale-caches?staleDays=30&checkAppExists=true&dryRun=false` — same behaviour via query parameters (suitable for cron jobs)

Defaults: `staleDays=30`, `checkAppExists=true` (only delete caches for apps that no longer exist), `dryRun=false`.

## Helm chart configuration (recommended)

To align with Epinio’s philosophy of configuration via the Helm chart rather than manual setup, the **Epinio Helm chart** ([epinio/helm-charts](https://github.com/epinio/helm-charts)) should expose optional automated cleanup:

1. **Values** (example names; chart maintainers may choose different keys):
   - `staleCacheCleanup.enabled` — create an optional CronJob that calls the cleanup API (default: `false`)
   - `staleCacheCleanup.schedule` — cron schedule (e.g. `"0 2 * * *"` for daily at 2 AM)
   - `staleCacheCleanup.staleDays` — days after which a cache is considered stale (default: `30`)
   - `staleCacheCleanup.checkAppExists` — only delete caches for deleted apps (default: `true`)

2. **CronJob** (when enabled): a Job that runs on the schedule and performs a GET request to the Epinio API, e.g.:
   `GET https://<epinio-api>/api/v1/maintenance/cleanup-stale-caches?staleDays=30&checkAppExists=true`
   with appropriate authentication (e.g. API token from a secret mounted by the chart).

## Testing and verification

In local or CI environments you can validate the behavior in two layers:

1. **Unit tests (no cluster required)**  
   Run:

   ```bash
   go test ./internal/api/v1/maintenance/... -v -count=1
   ```

   This verifies that the handlers:

   - Return HTTP 400 for invalid JSON or invalid `staleDays`.
   - Return 500 when cluster access fails, but only after input is valid.

2. **Manual API call against a running Epinio**  
   With Epinio deployed, call the endpoint in dry‑run mode:

   ```bash
   curl -u admin:YOUR_PASSWORD \
     "https://<EPINIO_API>/api/v1/maintenance/cleanup-stale-caches?staleDays=30&checkAppExists=true&dryRun=true"
   ```

   You should see `dryRun: true` in the response and a list of `staleCaches` that would be deleted.

For full end‑to‑end behavior (including the Helm‑managed CronJob), see the user‑facing howto in the docs repo (`docs/howtos/operations/cleanup_stale_caches.md`).

This keeps the cluster tidy without requiring operators to manually configure cron or scripts.
