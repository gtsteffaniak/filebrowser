import type { CopyPathConfig } from "@/store/types.ts";

/** Format a clipboard path using only an explicitly configured source mapping. */
export function formatCopyPath(path: string, config?: CopyPathConfig, scope = "/"): string {
  if (!path || (!config?.copyPathPrefix && !config?.copyPathWindows)) return path;

  // API paths are relative to the user's scope, which is relative to the source.
  const scopePrefix = scope.replace(/\/+$/, "");
  let mappedPath = scopePrefix ? `${scopePrefix}/${path.replace(/^\/+/, "")}` : path;
  if (config.copyPathPrefix) {
    mappedPath = `${config.copyPathPrefix.replace(/\/+$/, "")}/${mappedPath.replace(/^\/+/, "")}`;
  }

  if (config.copyPathWindows) {
    const drive = /^\/([a-zA-Z])(?:\/|$)/.exec(mappedPath);
    if (drive) {
      return `${drive[1]}:\\${mappedPath.slice(drive[0].length).replace(/\//g, "\\")}`;
    }
  }
  return mappedPath;
}
