# Backend import path migration

## Public library API (`pkg/`)

| Old | New |
|-----|-----|
| `.../common/settings` | `.../pkg/settings` |
| `.../indexing` | `.../pkg/indexing` |
| `.../indexing/iteminfo` | `.../pkg/indexing/iteminfo` |

## Internal application code

| Old | New |
|-----|-----|
| `.../http` | `.../internal/web` |
| `.../common/utils` | `.../internal/utils` |
| `.../common/errors` | `.../internal/errors` |
| `.../common/version` | `.../internal/version` |
| `.../state` | `.../internal/state` |
| `.../auth` | `.../internal/auth` |
| `.../database/users` | `.../internal/database/users` |
| `.../database/share` | `.../internal/database/share` |
| `.../database/access` | `.../internal/database/access` |
| `.../database/activity` | `.../internal/database/activity` |
| `.../database/sqldb` | `.../internal/database/sqldb` |
| `.../database/sql` | `.../internal/database/sql` |
| Activity query/record/filter (was in `internal/web`) | `.../internal/activity` |

## Build paths

| Old | New |
|-----|-----|
| `backend/http/dist` | `backend/internal/web/dist` |
| `backend/http/embed` | `backend/internal/web/embed` |

## Entry points

- `cmd` imports `internal/web` for `StartHttp` and `GetEmbeddedAssets`.
- External modules should use `pkg/settings` and `pkg/indexing` only; everything else is module-private via `internal/`.
