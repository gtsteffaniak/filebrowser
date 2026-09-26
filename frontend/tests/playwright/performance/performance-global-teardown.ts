import { readFile } from "node:fs/promises";
import path from "node:path";
import { writeFinalArtifacts } from "./perf-finalize";
import { perfResultsDir } from "./perf-helpers";

/**
 * Finalize the run: aggregate iterations, compare against the committed
 * baseline, and emit report.json / report.md.
 *
 * This can now throw, which is intentional: a run that produced no
 * measurements must fail rather than write an empty report and exit green.
 */
async function globalTeardown() {
  let browserVersion = "unknown";
  try {
    const raw = JSON.parse(
      await readFile(
        path.join(perfResultsDir(), "..", "perf-browser-version.json"),
        "utf8",
      ),
    ) as { browserVersion?: string };
    browserVersion = raw.browserVersion ?? "unknown";
  } catch {
    /* version capture is best-effort */
  }

  await writeFinalArtifacts({ browserVersion });
}

export default globalTeardown;
