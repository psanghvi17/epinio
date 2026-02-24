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

This keeps the cluster tidy without requiring operators to manually configure cron or scripts.
