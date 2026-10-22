# WebDAV write coordination (proposed)

This change enables real `LOCK` and `UNLOCK` for users with source modify permission.
The same underlying source lock system is used for scoped WebDAV views and API
write, upload finalization, move, copy destination, delete, and directory creation.
A client which does not present the appropriate lock token receives a conflict
rather than silently overwriting an actively locked file.

HTTP `If-Match`, `If-None-Match`, and `If-Unmodified-Since` are checked before the
first filesystem mutation, while the DAV operation's lease is active. Failed
preconditions return 412 without truncating or replacing existing data. GET/HEAD
conditional handling remains with the existing HTTP handler. Tagged DAV `If`
conditions must refer to the actual source/destination, not an unrelated locked
resource. Unsupported DAV `If` ETag/Not conditions fail closed rather than being
silently ignored by the underlying in-memory lock implementation.

## Deployment requirements

Reverse proxies must forward conditional headers and real LOCK/UNLOCK requests.
Do not synthesize successful lock responses or strip `If`, `If-Match`,
`If-None-Match`, `If-Unmodified-Since`, or `Lock-Token` on writes. Otherwise the
backend cannot enforce client preconditions correctly. This does not change the
existing token authentication and access-control checks.

## Deliberate limitations / review points

* Coordination is process-local and per source index. Multiple replicas,
  overlapping source configurations referring to the same physical files,
  hardlink aliases, and direct external filesystem/S3 writers are not coordinated.
* Locks are in memory, as with the existing index lock system. Restarting the
  process discards them; this is not durable distributed locking.
* Source roots may be canonicalized through filesystem aliases, but mutation
  paths containing symlinks inside an exposed tree are rejected, except removal
  of the symlink entry itself without following it. Supporting these
  safely requires an agreed canonical identity model.
* A conservative source-wide mutation gate serializes writes across WebDAV and
  the API, including COPY metadata updates. GET/HEAD/PROPFIND do not acquire this
  gate. This closes validation/commit and metadata races, but large uploads can
  delay unrelated writes in the same source. A finer-grained shared hierarchy
  could replace it after the required semantics and tests are agreed.
* This is a proposal for review, not a claim of full RFC4918 compliance. In
  particular PROPPATCH/dead-property support and client-specific Office/Finder
  interoperability require separate validation. Do not replace unsupported
  operations with fabricated successful responses.

## Tests

The regression tests cover lock lifecycle, scoped paths identifying the same
physical resource, separate scopes, parent/child locks, expiration while held,
wrong-URI tokens, tagged-If substitution, API write exclusion, conditional writes
without truncation, DELETE/MOVE preconditions, and concurrent writes using one
old ETag (at most one succeeds). Run:

```
cd backend
go test ./internal/web ./internal/adapters/fs/files ./pkg/indexing
go test -race ./internal/web ./internal/adapters/fs/files ./pkg/indexing -run 'Test(DAV|ScopedLock|WebDAV|MutationGuard|LockDAVMutations)'
```
