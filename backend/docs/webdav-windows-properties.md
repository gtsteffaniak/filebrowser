# Windows WebDAV file properties

Windows Explorer may upload a file successfully (`PUT 201`), acquire a write
lock (`LOCK 200`), then send `PROPPATCH` to preserve timestamps and attributes.
Rejecting that request with `403` can cause Explorer to delete the upload and
report `0x80070780`, including for very small files.

This patch adds persistent dead properties in a separate
`webdav-properties.sqlite` beside the configured application database. Back up
and restore both databases. Preserve the source's physical path when restoring:
property keys include that path.

`PROPPATCH` requires modify permission and respects read-only sources, real
WebDAV locks and HTTP preconditions. Protected DAV live properties remain
protected. Windows timestamps are stored as client properties; they do not
change filesystem/S3 modification times or grant file permissions.

Values retain namespaces and XML. Updates are transactional within the property
database, with limits of 128 properties and 64 KiB serialized data per object,
and 256 KiB per PROPPATCH request. MOVE, COPY and DELETE reconcile stored
properties; API mutation callbacks use the existing mutation coordination.

## Validation

The upstream implementation passed the full backend race suite (32 packages
with tests), backend lint (0 issues), and HTTP/property regressions. A separate
stable backport was built with the existing analytics/muPDF/musl runtime.

Runtime checks on that stable backport:

- Isolated filesystem instance: 23 Windows-property checks passed, including
  property persistence across a container restart.
- Deployed instance: 12 Windows-copy checks and 26 lock/conditional-write checks
  passed. The probe checked actual property values and file bytes, not only
  response codes. Only probe-owned temporary files were created and removed.
- A user confirmed Windows Explorer successfully copied and retained a file
  after deployment. This does not establish compatibility with every Office
  editing/saving workflow or all Windows configurations.

## Limits and storage-layer findings

The filesystem and the two SQLite databases do not share a distributed
transaction. A filesystem success followed by a property-database failure may
require reconciliation. Direct external filesystem/S3 writes do not invoke
property callbacks. Intermediate MkdirAll directories and staged uploads that
directly acquire DAV mutation gates do not all have lifecycle callbacks.

An independent storage test reproduced an immediate directory-rename/read
failure (`EIO`) on rclone 1.60.1-DEV with VFS write caching, even without
FileBrowser. A separate mount using rclone 1.75.2 passed 10 immediate-rename
content checks and the strict 23-step HTTP scenario. Waiting for old write-back
also made its diagnostic test pass. No artificial wait was added to this patch;
these are bounded test results, not a claim that all rclone configurations are
fixed. The production storage binary was not changed by the properties rollout.

This implementation does not solve Office policies blocking Basic
authentication. See `webdav-locking.md` for process-local locking assumptions.
