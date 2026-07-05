// Package state is the single gateway for all persisted application data.
//
// Architecture (do not bypass or duplicate this layer):
//
//   - sqlDb (*sqldb.SQLStore): the on-disk SQLite database. All durable writes go here first
//     (or atomically with cache updates in the same critical section).
//
//   - In-memory caches (usersByID, sharesByHash, accessDb, indexInfoByPath, …): authoritative
//     for reads at runtime after Open/Initialize. Handlers and business logic must call exported
//     state.* functions or *state.Store methods—not sqlDb directly and not package globals in internal/web.
//
//   - accessDb (*access.Storage): access rules, groups, and API token hashes held in memory with
//     write-through to sqlDb. Use state.AccessPermitted and the other helpers in access.go.
//
// Dependency injection: internal/ports defines narrow interfaces; *state.Store implements them.
// cmd (via internal/app.WireServices) constructs domain services (files, auth, indexing, activity)
// with explicit store dependencies. Domain packages must not import state.
//
// Legacy persistence: only cmd/migrate.go reads the old Storm database format for one-time migration.
//
// When adding new persisted entities: extend sqlDb, add a cache + mutex here, expose state.Foo
// read/write helpers on Store, wire load/save in initialize/Close.

package state
