# Configurable Copy Path

The existing **Copy Path** action copies the path shown inside FileBrowser by
default. Configure each source independently under `server.sources[].config`:

```yaml
server:
  sources:
    - name: Home
      path: /mnt/home
      config:
        copyPathPrefix: /home/rt
    - name: Data
      path: /mnt/data
      config:
        copyPathPrefix: /data
    - name: RT
      path: /mnt/rt
      config:
        copyPathWindows: true
```

| Source | Path in FileBrowser | Copied path |
| --- | --- | --- |
| Home | `/pact_runs/books/` | `/home/rt/pact_runs/books/` |
| Data | `/movies/test.mkv` | `/data/movies/test.mkv` |
| RT | `/D/qwen36-longtest-nomtp.txt` | `D:\qwen36-longtest-nomtp.txt` |

`copyPathPrefix` is an optional absolute POSIX prefix, starting with `/`. Trailing
slashes on the prefix are accepted. `copyPathWindows` defaults to `false` and
converts a leading single-letter drive component (`/D/` or `/D`) into `D:\`, with
backslashes in the rest of the path. Other paths retain POSIX separators. When
both options are set, the prefix is applied first, then Windows conversion.
Directory trailing slashes, spaces, Unicode and literal `%` characters are preserved
(directory separators become backslashes for Windows paths).

For a user scoped to a subdirectory, an enabled mapping includes that source-relative
scope before the displayed path. For example, prefix `/home`, scope `/rt/`, and
displayed path `/books/` produce `/home/rt/books/`. Search results use the selected
item's source settings. Public shares retain share-relative paths.

## Privacy and compatibility

Setting these options explicitly permits exposing the configured clipboard mapping
to authenticated users with access to that source through `/api/settings/sources`
and source SSE updates. Choose a prefix that those users are allowed to see; it may
describe a host mount or a path on their own machines. The backend never derives a
clipboard prefix from the source's filesystem `path`, resolves symlinks for this
action, or sends additional host paths. With neither option enabled, Copy Path
retains its previous output, including for scoped users.

These settings affect clipboard text only. They do not change source paths, access
rules, resource API paths, download URLs or filesystem operations. Apply them in the
server configuration and restart FileBrowser to load the changes.
