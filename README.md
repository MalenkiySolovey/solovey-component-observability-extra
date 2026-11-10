# Solovey Observability Extra

Optional in-process history and sampling for Solovey hosts, with Audit and Diagnostics UI contributions.

The component keeps bounded host and CoreRuntime history in memory. Its existing scheduler samples every two seconds and rolls samples into 2s, 30s, 1m and 5m buckets. CPU and numeric memory/network fields retain their existing averages; CoreRuntime rollup keeps the latest observation. The default configured memory policy is 32 MB, with bounded ring capacities. History resets on restart and is not a durable database.

## Integration

The Go module is `github.com/MalenkiySolovey/solovey-component-observability-extra`. It uses `solovey-host@v0.2.0` public observation and ComponentHost contracts. `Manifest()` returns the canonical `observability-extra` descriptor (manifest version 1, Host API `solovey.component-host/v1`). `New(warning)` creates an inactive lifecycle. The host admits and starts it with declared Scheduler and ObservationSampler capabilities. Stop drains its one job and retains retry state on failure.

The `history` package owns rings, memory policy and queries. Construct a `Store` per host and bind a `Service` to that store, a settings reader and current-snapshot provider. Each store is isolated; importing packages does not schedule work, register a host or access storage. HostRef, fact source and freshness remain with the consuming host. Historical samples do not establish current Health.

Current metrics, Health, Audit storage/retention/redaction, Diagnostics reports, logs and Support Bundle remain with their existing owners. Panel adapters retain authentication, session/token scope, rate limiting and query validation. No Panel database, Gin, broad Runtime, filesystem or network authority is required by this module.

## Frontend contribution

`frontend/index.ts` contributes the local `/audit` and `/diagnostics` routes through the existing Classic/Nexus frontend. The shared frontend owns routing, authorization, locales, UI widgets and the HTTP client. Audit needs `audit.read`; Diagnostics needs `diagnostics.read`. Six existing locales are included. The product builds exact pinned source and validates its full dynamic-chunk/CSS closure; the contribution has no standalone app or runtime CDN loader.

`HOST_UI_PIN.json` identifies the exact first-party UI API, lock and toolchain used by independent frontend checks. Product source integration consumes an immutable accepted module and frontend digest. A new source commit does not automatically update an installed product.

## Testing

Use Go 1.26.10 with `GOWORK=off` and no module replacements:

```sh
go test -race -count=1 ./...
go vet ./...
go test -tags minimal ./...
node scripts/check-source.mjs
```

For frontend checks, check out the Panel source revision in `HOST_UI_PIN.json`, install its frontend dependencies with its pinned Node/npm lock, then run `node scripts/check-frontend.mjs /path/to/panel/frontend`. Source CI checks both Linux and Windows. Test-only independent host consumers use fake snapshots and schedulers; this is not a production Node telemetry transport.

## License and provenance

GPL-3.0-only, retaining the original Solovey UI license. Filtered component Git history preserves original authors. `SOURCE_PROVENANCE.json` records original source commits, blobs, per-file history and adaptations, including history policy originally in `service/observability`. See LICENSE and NOTICE. Product releases and signed update trust remain owned by Solovey UI.
