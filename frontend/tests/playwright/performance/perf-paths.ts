import path from "node:path";
import { fileURLToPath } from "node:url";

/**
 * Module-local directory resolution.
 *
 * Playwright loads these test files as ES modules, where `__dirname` is not
 * defined. `import.meta.url` is the portable way to derive the directory.
 */
export function moduleDir(importMetaUrl: string): string {
  return path.dirname(fileURLToPath(importMetaUrl));
}

/** The `frontend/` package root, derived from this file's location. */
export function frontendRoot(importMetaUrl: string): string {
  // <frontend>/tests/playwright/performance/perf-paths.ts -> <frontend>
  return path.resolve(moduleDir(importMetaUrl), "..", "..", "..");
}

/** The repository root (parent of `frontend/`). */
export function repoRoot(importMetaUrl: string): string {
  return path.resolve(frontendRoot(importMetaUrl), "..");
}

/** The performance test directory. */
export function performanceDir(importMetaUrl: string): string {
  return moduleDir(importMetaUrl);
}
