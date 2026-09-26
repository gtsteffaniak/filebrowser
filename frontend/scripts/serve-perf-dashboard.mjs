import { createServer } from "node:http";
import { existsSync, statSync } from "node:fs";
import { readFile, stat } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const perfRoot = path.join(frontendDir, "tests/playwright/performance");
const port = Number(process.env.PERF_DASHBOARD_PORT ?? "9323");
const host = process.env.PERF_DASHBOARD_HOST ?? "127.0.0.1";

const mimeTypes = {
  ".html": "text/html; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
};

const chartBundle = path.join(
  frontendDir,
  "node_modules/chart.js/dist/chart.umd.min.js",
);

async function resolveFile(urlPath) {
  let rel = decodeURIComponent(urlPath.split("?")[0]);
  if (rel === "/" || rel === "") {
    rel = "/dashboard/report.html";
  }
  if (rel === "/dashboard/chart.umd.min.js" || rel.endsWith("/chart.umd.min.js")) {
    return existsSync(chartBundle) ? chartBundle : null;
  }
  let file = path.normalize(path.join(perfRoot, rel));
  if (!file.startsWith(perfRoot)) {
    return null;
  }
  if (!existsSync(file)) {
    return null;
  }
  const info = await stat(file);
  if (info.isDirectory()) {
    // The dashboard has a single entry point; there is no index.html alias.
    const report = path.join(file, "report.html");
    if (existsSync(report) && !statSync(report).isDirectory()) {
      file = report;
    } else {
      return null;
    }
  }
  return file;
}

const server = createServer(async (req, res) => {
  try {
    const file = await resolveFile(req.url ?? "/");
    if (!file) {
      res.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" });
      res.end(
        "Not found. Run make perf-check first, then open /dashboard/\n",
      );
      return;
    }
    const body = await readFile(file);
    const ext = path.extname(file);
    res.writeHead(200, {
      "Content-Type": mimeTypes[ext] ?? "application/octet-stream",
      "Cache-Control": "no-store",
    });
    res.end(body);
  } catch (err) {
    res.writeHead(500, { "Content-Type": "text/plain; charset=utf-8" });
    res.end(String(err));
  }
});

function printUrls(base) {
  console.log("");
  console.log("Listing performance dashboard");
  console.log(`  ${base}/dashboard/report.html`);
  console.log(`  ${base}/results.json`);
  console.log("");
}

server.on("error", (err) => {
  const base = `http://${host}:${port}`;
  if (err.code === "EADDRINUSE") {
    printUrls(base);
    console.log(
      `Port ${port} is already in use — a dashboard server is probably still running.`,
    );
    console.log("Open the URLs above in your browser, or stop the old server:");
    console.log(`  fuser -k ${port}/tcp`);
    console.log(`  PERF_DASHBOARD_PORT=9400 make perf-dashboard`);
    console.log("");
    process.exit(0);
  }
  console.error(err);
  process.exit(1);
});

server.listen(port, host, () => {
  const base = `http://${host}:${port}`;
  printUrls(base);
  console.log(`Serving: ${perfRoot}`);
  console.log("Press Ctrl+C to stop.");
});
