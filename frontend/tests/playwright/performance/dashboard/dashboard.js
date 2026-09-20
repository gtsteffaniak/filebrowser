/* global Chart */
const SCENARIO_COLORS = {
  load: "#5b9fd4",
  scroll: "#3d9a6a",
  resize: "#c9a227",
  select: "#b86dbb",
};

const BROWSER_COLORS = {
  chromium: "#5b9fd4",
  firefox: "#e87a2e",
  webkit: "#b86dbb",
};

const SCENARIOS = ["load", "scroll", "resize", "select"];
const REPEAT_COLORS = ["#5b9fd4", "#3d9a6a", "#c9a227"];

let data;
const liveCharts = [];

async function loadResults() {
  // `report.json` is canonical; `results.json` is kept as a compatibility alias.
  const paths = [
    "/report.json",
    "../report.json",
    "report.json",
    "/results.json",
    "../results.json",
    "results.json",
  ];
  for (const p of paths) {
    try {
      const res = await fetch(new URL(p, window.location.href));
      if (res.ok) return res.json();
    } catch (_) {
      /* try next */
    }
  }
  throw new Error(
    "Could not load report.json — run make perf-check, then make perf-dashboard.",
  );
}

function scaleLabel(s) {
  return `${s}+${s}`;
}

function esc(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function inlineMd(text) {
  return esc(text).replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
}

function fillMeta() {
  const browsers = data.browsers?.join(", ") ?? "—";
  document.getElementById("meta").textContent =
    `Generated ${new Date(data.generatedAt).toLocaleString()} · browsers: ${browsers} · scales ${data.scales.join(", ")}`;
}

function fillSummary() {
  const ul = document.getElementById("summary");
  ul.innerHTML = "";
  for (const row of data.summary ?? []) {
    const li = document.createElement("li");
    li.innerHTML =
      `<strong>${esc(row.browser)}</strong> @ ${scaleLabel(row.scale)} — slowest: ` +
      `<code>${esc(row.slowestScenario)}</code> (${row.slowestMs} ms)`;
    ul.appendChild(li);
  }
}

function severityClass(sev) {
  if (sev === "high") return "sev-high";
  if (sev === "medium") return "sev-med";
  return "sev-low";
}

function fillSlowestContributors() {
  const root = document.getElementById("slowest-root");
  root.innerHTML = "";
  for (const row of data.summary ?? []) {
    const card = document.createElement("article");
    card.className = "detail-card";
    const title = document.createElement("h3");
    title.textContent = `${row.browser} @ ${scaleLabel(row.scale)} — ${row.slowestScenario} (${row.slowestMs} ms)`;
    card.appendChild(title);

    const pre = document.createElement("pre");
    pre.className = "root-cause";
    pre.innerHTML = inlineMd(row.rootCause || "");
    card.appendChild(pre);

    if (row.contributors?.length) {
      const h4 = document.createElement("h4");
      h4.textContent = "Top contributors";
      card.appendChild(h4);
      const ol = document.createElement("ol");
      ol.className = "contributors";
      for (const c of row.contributors.slice(0, 6)) {
        const li = document.createElement("li");
        li.className = severityClass(c.severity);
        li.innerHTML =
          `<span class="factor">${esc(c.factor)}</span> — ${esc(c.detail)}` +
          `<pre class="evidence">${esc(JSON.stringify(c.evidence, null, 2))}</pre>`;
        ol.appendChild(li);
      }
      card.appendChild(ol);
    }
    root.appendChild(card);
  }
}

function findRun(browser, scale, scenario) {
  return (data.runs ?? []).find(
    (r) => r.browser === browser && r.scale === scale && r.scenario === scenario,
  );
}

function repeatSamples(browser, scale, scenario) {
  const run = findRun(browser, scale, scenario);
  const samples = run?.metrics?.__samples?.scenarioMs;
  if (Array.isArray(samples) && samples.length) return samples;
  if (typeof run?.durationMs === "number") return [run.durationMs];
  return [];
}

function fillRuns() {
  const tbody = document.getElementById("runs-body");
  tbody.innerHTML = "";
  const runs = [...(data.runs ?? [])].sort((a, b) => {
    const scale = a.scale - b.scale;
    if (scale) return scale;
    const browser = String(a.browser).localeCompare(String(b.browser));
    if (browser) return browser;
    return SCENARIOS.indexOf(a.scenario) - SCENARIOS.indexOf(b.scenario);
  });
  for (const run of runs) {
    const dom = run.metrics.dom || {};
    const probe = run.metrics.probe || {};
    const samples = repeatSamples(run.browser, run.scale, run.scenario);
    const tr = document.createElement("tr");
    tr.dataset.browser = run.browser;
    tr.dataset.scale = String(run.scale);
    tr.dataset.scenario = run.scenario;
    const badge = run.thresholds.passed
      ? '<span class="badge ok">pass</span>'
      : `<span class="badge warn" title="${esc(run.thresholds.warnings.join("; "))}">warn</span>`;
    tr.innerHTML =
      `<td>${esc(run.browser)}</td>` +
      `<td>${scaleLabel(run.scale)}</td>` +
      `<td>${esc(run.scenario)}</td>` +
      `<td class="num">${run.durationMs}</td>` +
      `<td class="num">${samples.map(fmt).join(" · ") || "—"}</td>` +
      `<td class="num">${dom.listingItemCount ?? "—"}</td>` +
      `<td class="num">${probe.addListenerCalls ?? "—"}</td>` +
      `<td class="num">${probe.intersectionObservers ?? "—"}</td>` +
      `<td>${badge}</td>`;
    tr.addEventListener("click", () => showRunDetail(run));
    tbody.appendChild(tr);
  }
}

function showRunDetail(run) {
  const panel = document.getElementById("run-detail");
  panel.hidden = false;
  panel.innerHTML = "";

  const h3 = document.createElement("h3");
  h3.textContent = `${run.browser} / ${scaleLabel(run.scale)} / ${run.scenario}`;
  panel.appendChild(h3);

  const pre = document.createElement("pre");
  pre.className = "root-cause";
  pre.innerHTML = inlineMd(run.rootCause);
  panel.appendChild(pre);

  if (run.contributors?.length) {
    const h4 = document.createElement("h4");
    h4.textContent = "Contributors (ranked)";
    panel.appendChild(h4);
    const ol = document.createElement("ol");
    ol.className = "contributors";
    for (const c of run.contributors) {
      const li = document.createElement("li");
      li.className = severityClass(c.severity);
      li.innerHTML =
        `#${c.rank} <span class="factor">${esc(c.factor)}</span> — ${esc(c.detail)}` +
        `<pre class="evidence">${esc(JSON.stringify(c.evidence, null, 2))}</pre>`;
      ol.appendChild(li);
    }
    panel.appendChild(ol);
  }

  const h4p = document.createElement("h4");
  h4p.textContent = "Profiling snapshot";
  panel.appendChild(h4p);
  const preProf = document.createElement("pre");
  preProf.className = "json-block";
  preProf.textContent = JSON.stringify(run.profiling, null, 2);
  panel.appendChild(preProf);

  if (!run.thresholds.passed) {
    const warn = document.createElement("p");
    warn.className = "warn-line";
    warn.textContent = run.thresholds.warnings.join(" · ");
    panel.appendChild(warn);
  }

  panel.scrollIntoView({ behavior: "smooth", block: "nearest" });
}

function fillAnalysisTable() {
  const tbody = document.getElementById("analysis-body");
  tbody.innerHTML = "";
  const rows = data.analysisTable ?? [];
  const max = 500;
  for (let i = 0; i < Math.min(rows.length, max); i++) {
    const r = rows[i];
    const tr = document.createElement("tr");
    tr.innerHTML =
      `<td>${esc(r.browser)}</td>` +
      `<td>${r.scale}</td>` +
      `<td>${esc(r.scenario)}</td>` +
      `<td><code>${esc(r.metric)}</code></td>` +
      `<td class="num wrap">${esc(r.value)}</td>`;
    tbody.appendChild(tr);
  }
  if (rows.length > max) {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td colspan="5">… ${rows.length - max} more rows in results.json</td>`;
    tbody.appendChild(tr);
  }
}

function makeChart(canvas, config) {
  const chart = new Chart(canvas, config);
  liveCharts.push(chart);
  return chart;
}

function commonBarOptions(yTitle) {
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { position: "bottom", labels: { boxWidth: 10, font: { size: 10 } } },
    },
    scales: {
      x: { ticks: { font: { size: 10 } } },
      y: {
        beginAtZero: true,
        title: { display: true, text: yTitle || "ms", font: { size: 10 } },
        ticks: { font: { size: 10 } },
      },
    },
  };
}

function medianOf(values) {
  const nums = (values ?? []).filter((n) => typeof n === "number" && Number.isFinite(n));
  if (!nums.length) return null;
  const sorted = [...nums].sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
}

function hexAlpha(hex, a) {
  const n = (hex || "#888").replace("#", "");
  const r = parseInt(n.slice(0, 2), 16);
  const g = parseInt(n.slice(2, 4), 16);
  const b = parseInt(n.slice(4, 6), 16);
  return `rgba(${r}, ${g}, ${b}, ${a})`;
}

function growthCaption(scenario) {
  const scales = data.scales ?? [];
  if (scales.length < 2) return "";
  const bits = [];
  for (const browser of data.browsers ?? []) {
    const durs = scales.map((s) => medianOf(repeatSamples(browser, s, scenario)));
    const steps = [];
    for (let i = 1; i < scales.length; i++) {
      const prev = durs[i - 1];
      const cur = durs[i];
      const dataX = scales[i] / scales[i - 1];
      if (!prev || !cur) continue;
      const timeX = cur / prev;
      steps.push(
        `${scales[i - 1]}→${scales[i]} ${timeX.toFixed(1)}×` +
          (Math.abs(dataX - 10) < 0.01 ? "" : ` (data ${dataX}×)`),
      );
    }
    if (steps.length) bits.push(`${browser}: ${steps.join(", ")}`);
  }
  return bits.join(" · ") + (bits.length ? " · linear would be 10× each step" : "");
}

function lineChartOptions(yTitle) {
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: "index", intersect: false },
    plugins: {
      legend: {
        position: "bottom",
        labels: {
          boxWidth: 12,
          font: { size: 10 },
          filter: (item) => !String(item.text).includes("linear") && !String(item.text).includes("repeat"),
        },
      },
    },
    scales: {
      x: {
        title: { display: true, text: "listing size (files)", font: { size: 10 } },
        ticks: { font: { size: 10 } },
      },
      y: {
        beginAtZero: true,
        title: { display: true, text: yTitle || "ms", font: { size: 10 } },
        ticks: { font: { size: 10 } },
      },
    },
  };
}

function renderScalingCharts() {
  const root = document.getElementById("scaling-grid");
  if (!root) return;
  root.innerHTML = "";
  const browsers = data.browsers ?? [];
  const scales = data.scales ?? [];
  const labels = scales.map(String);

  for (const scenario of SCENARIOS) {
    const panel = document.createElement("div");
    panel.className = "chart-panel";
    const title = document.createElement("h3");
    title.textContent = scenario;
    panel.appendChild(title);
    const wrap = document.createElement("div");
    wrap.className = "chart-wrap compact";
    const canvas = document.createElement("canvas");
    wrap.appendChild(canvas);
    panel.appendChild(wrap);
    const cap = document.createElement("p");
    cap.className = "chart-caption";
    cap.textContent = growthCaption(scenario);
    panel.appendChild(cap);
    root.appendChild(panel);

    const datasets = [];
    for (const browser of browsers) {
      const color = BROWSER_COLORS[browser] || "#888";
      const medians = scales.map((s) => medianOf(repeatSamples(browser, s, scenario)));
      const t0 = medians.find((n) => n && n > 0) ?? medians[0];
      const n0 = scales[0] || 1;

      datasets.push({
        label: browser,
        data: medians,
        borderColor: color,
        backgroundColor: color,
        tension: 0.12,
        spanGaps: true,
        pointRadius: 4,
        borderWidth: 2,
      });
      datasets.push({
        label: `${browser} linear`,
        data: scales.map((s) => (t0 ? t0 * (s / n0) : null)),
        borderColor: hexAlpha(color, 0.45),
        borderDash: [5, 4],
        borderWidth: 1.5,
        pointRadius: 0,
        tension: 0,
        spanGaps: true,
      });

      scales.forEach((scale, idx) => {
        for (const sample of repeatSamples(browser, scale, scenario)) {
          datasets.push({
            label: `${browser} repeat`,
            data: labels.map((_, j) => (j === idx ? sample : null)),
            borderWidth: 0,
            pointRadius: 3,
            pointBackgroundColor: hexAlpha(color, 0.35),
            pointBorderWidth: 0,
            showLine: false,
          });
        }
      });
    }

    makeChart(canvas, {
      type: "line",
      data: { labels, datasets },
      options: lineChartOptions("ms"),
    });
  }
}

function renderPerRowCharts() {
  const root = document.getElementById("per-row-grid");
  if (!root) return;
  root.innerHTML = "";
  const browsers = data.browsers ?? [];
  const scales = data.scales ?? [];
  const labels = scales.map(String);

  for (const scenario of SCENARIOS) {
    const panel = document.createElement("div");
    panel.className = "chart-panel";
    const title = document.createElement("h3");
    title.textContent = scenario;
    panel.appendChild(title);
    const wrap = document.createElement("div");
    wrap.className = "chart-wrap compact";
    const canvas = document.createElement("canvas");
    wrap.appendChild(canvas);
    panel.appendChild(wrap);
    root.appendChild(panel);

    const datasets = browsers.map((browser) => {
      const color = BROWSER_COLORS[browser] || "#888";
      return {
        label: browser,
        data: scales.map((s) => {
          const ms = medianOf(repeatSamples(browser, s, scenario));
          const items = s * 2;
          return ms && items ? Math.round((ms / items) * 1000 * 100) / 100 : null;
        }),
        borderColor: color,
        backgroundColor: color,
        tension: 0.12,
        spanGaps: true,
        pointRadius: 4,
        borderWidth: 2,
      };
    });

    makeChart(canvas, {
      type: "line",
      data: { labels, datasets },
      options: lineChartOptions("ms / 1000 rows"),
    });
  }
}

function renderDomCharts() {
  const root = document.getElementById("dom-grid");
  root.innerHTML = "";
  for (const browser of data.browsers ?? []) {
    const d = data.charts.domAtLoad[browser];
    if (!d) continue;
    const panel = document.createElement("div");
    panel.className = "chart-panel";
    const title = document.createElement("h3");
    title.textContent = browser;
    panel.appendChild(title);
    const wrap = document.createElement("div");
    wrap.className = "chart-wrap compact";
    const canvas = document.createElement("canvas");
    wrap.appendChild(canvas);
    panel.appendChild(wrap);
    root.appendChild(panel);

    makeChart(canvas, {
      type: "line",
      data: {
        labels: d.scales.map(String),
        datasets: [
          {
            label: "listing rows",
            data: d.listingItems,
            borderColor: "#5b9fd4",
            tension: 0.2,
          },
          {
            label: "listeners",
            data: d.listeners,
            borderColor: "#c9a227",
            tension: 0.2,
          },
          {
            label: "intersection observers",
            data: d.intersectionObservers,
            borderColor: "#3d9a6a",
            tension: 0.2,
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        plugins: {
          legend: { position: "bottom", labels: { boxWidth: 10, font: { size: 10 } } },
        },
        scales: { y: { beginAtZero: true } },
      },
    });
  }
}

const DELTA_BUCKETS = [
  { id: "g50", label: "≤-50%", test: (p) => p <= -50, color: "#157a45" },
  { id: "g20", label: "-50:-20", test: (p) => p <= -20, color: "#2f9e63" },
  { id: "g10", label: "-20:-10", test: (p) => p <= -10, color: "#5bb87e" },
  { id: "g2", label: "-10:-2", test: (p) => p <= -2, color: "#8fd4a8" },
  { id: "flat", label: "≈0", test: (p) => p < 2, color: "#6b7788" },
  { id: "r2", label: "+2:+10", test: (p) => p < 10, color: "#e8a0a0" },
  { id: "r10", label: "+10:+20", test: (p) => p < 20, color: "#e07a7a" },
  { id: "r20", label: "+20:+50", test: (p) => p < 50, color: "#d44c4c" },
  { id: "r50", label: ">+50%", test: () => true, color: "#b91c1c" },
];

let deltaAll = [];
let deltaFilter = {
  status: "all",
  scale: "all",
  scenario: "all",
  klass: "all",
  bucket: "all",
};
let deltaSort = { key: "signedPct", dir: "desc" };
let deltaBound = false;

/** Dashboard judgement: |Δ| ≥ 10% is a regression or improvement, ignoring CI's looser gates. */
const DISPLAY_DELTA_PCT = 10;

function signedPct(m) {
  if (typeof m.deltaPct !== "number" || !Number.isFinite(m.deltaPct)) return null;
  return m.direction === "higher-is-better" ? -m.deltaPct : m.deltaPct;
}

function displayStatus(m) {
  if (m.status === "missing") return "missing";
  const pct = signedPct(m);
  if (pct == null) return "pass";
  if (pct >= DISPLAY_DELTA_PCT) return "regression";
  if (pct <= -DISPLAY_DELTA_PCT) return "improvement";
  return "pass";
}

function bucketOf(pct) {
  if (pct == null) return null;
  return DELTA_BUCKETS.find((b) => b.test(pct)) ?? null;
}

function deltaTone(pct) {
  if (pct == null || pct === 0) return { bg: "transparent", border: "var(--border)", fill: "#6b7788" };
  const mag = Math.min(Math.abs(pct), 50) / 50;
  const better = pct < 0;
  const base = better ? "var(--ok)" : "var(--high)";
  const mix = Math.round(18 + mag * 62);
  return {
    bg: `color-mix(in srgb, ${base} ${mix}%, transparent)`,
    border: base,
    fill: better ? "#2f9e63" : "#d44c4c",
  };
}

function statusBadge(status) {
  if (status === "regression") return '<span class="badge reg">regression</span>';
  if (status === "improvement") return '<span class="badge imp">improved</span>';
  if (status === "missing") return '<span class="badge miss">missing</span>';
  return '<span class="badge pass">pass</span>';
}

function flattenComparison(cmp) {
  return (cmp.runs ?? []).flatMap((r) =>
    (r.metrics ?? []).map((m) => {
      const pct = signedPct(m);
      return {
        ...m,
        ciStatus: m.status,
        status: displayStatus(m),
        scale: r.scale,
        scenario: r.scenario,
        signedPct: pct,
        bucket: bucketOf(pct)?.id ?? "flat",
        klass: m.toleranceClass || "other",
      };
    }),
  ).filter((row) => !isUnmeasuredPlaceholder(row));
}

/**
 * These collectors are scenario-specific. A 0→0 "pass" usually means the
 * probe never ran (programmatic scrollTop, no Event Timing samples, no frame
 * window), not that the listing was free.
 */
const ZERO_IS_UNMEASURED = new Set([
  "effectiveFps",
  "frameP95",
  "frameP99",
  "droppedFrames",
  "interactionP95",
  "layoutDurationDelta",
  "recalcStyleDurationDelta",
  "layoutCountDelta",
]);

const METRIC_SCENARIOS = {
  effectiveFps: ["load", "scroll", "resize", "select"],
  frameP95: ["load", "scroll", "resize", "select"],
  frameP99: ["load", "scroll", "resize", "select"],
  droppedFrames: ["load", "scroll", "resize", "select"],
  interactionP95: ["select"],
};

function isUnmeasuredPlaceholder(row) {
  if (row.status === "missing") return true;
  const allowed = METRIC_SCENARIOS[row.key];
  if (allowed && !allowed.includes(row.scenario)) return true;
  if (
    ZERO_IS_UNMEASURED.has(row.key) &&
    (row.baseline === 0 || row.baseline == null) &&
    (row.current === 0 || row.current == null)
  ) {
    return true;
  }
  return false;
}

function matchesDeltaFilter(row, override = {}) {
  const f = { ...deltaFilter, ...override };
  if (f.status !== "all" && row.status !== f.status) return false;
  if (f.scale !== "all" && String(row.scale) !== String(f.scale)) return false;
  if (f.scenario !== "all" && row.scenario !== f.scenario) return false;
  if (f.klass !== "all" && row.klass !== f.klass) return false;
  if (f.bucket !== "all" && row.bucket !== f.bucket) return false;
  return true;
}

function sortDeltaRows(rows) {
  const { key, dir } = deltaSort;
  const mul = dir === "asc" ? 1 : -1;
  return [...rows].sort((a, b) => {
    const va = a[key];
    const vb = b[key];
    if (va == null && vb == null) return 0;
    if (va == null) return 1;
    if (vb == null) return -1;
    if (typeof va === "number" && typeof vb === "number") return (va - vb) * mul;
    return String(va).localeCompare(String(vb)) * mul;
  });
}

function chip(group, value, label, count) {
  const active = String(deltaFilter[group]) === String(value) ? " active" : "";
  const n = count == null ? "" : `<span class="n">${count}</span>`;
  return `<button type="button" data-filter="${esc(group)}" data-value="${esc(value)}" class="${active.trim()}">${esc(label)}${n}</button>`;
}

function renderDeltaPills() {
  const counts = { all: deltaAll.length, regression: 0, improvement: 0, pass: 0, missing: 0 };
  for (const r of deltaAll) counts[r.status] = (counts[r.status] ?? 0) + 1;
  document.getElementById("delta-pills").innerHTML =
    `<div class="filter-group"><span class="lbl">Status</span>` +
    chip("status", "all", "all", counts.all) +
    chip("status", "regression", "regression", counts.regression) +
    chip("status", "improvement", "improved", counts.improvement) +
    chip("status", "pass", "pass", counts.pass) +
    chip("status", "missing", "missing", counts.missing) +
    `</div>`;
}

function renderDeltaHist() {
  const visibleForHist = deltaAll.filter((r) =>
    matchesDeltaFilter(r, { bucket: "all" }),
  );
  const counts = Object.fromEntries(DELTA_BUCKETS.map((b) => [b.id, 0]));
  for (const r of visibleForHist) {
    if (r.bucket) counts[r.bucket] += 1;
  }
  const max = Math.max(1, ...Object.values(counts));
  const root = document.getElementById("delta-hist");
  root.innerHTML = "";
  for (const b of DELTA_BUCKETS) {
    const n = counts[b.id];
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "bar" + (deltaFilter.bucket === b.id ? " active" : "");
    btn.dataset.filter = "bucket";
    btn.dataset.value = b.id;
    btn.title = `${b.label}: ${n} metric(s)`;
    const fillH = Math.round((n / max) * 88);
    btn.innerHTML =
      `<span class="cap">${n}</span>` +
      `<span class="fill" style="height:${fillH}px;background:${b.color}"></span>` +
      `<span class="hl">${esc(b.label)}</span>`;
    root.appendChild(btn);
  }

  const strip = document.getElementById("delta-strip");
  const sorted = [...visibleForHist].sort(
    (a, b) => (b.signedPct ?? -Infinity) - (a.signedPct ?? -Infinity),
  );
  strip.innerHTML = "";
  for (const row of sorted) {
    const tone = deltaTone(row.signedPct);
    const el = document.createElement("button");
    el.type = "button";
    el.style.background = tone.fill;
    el.style.opacity = String(
      0.35 + Math.min(Math.abs(row.signedPct ?? 0), 50) / 50 * 0.65,
    );
    if (deltaFilter.bucket !== "all" && row.bucket === deltaFilter.bucket) {
      el.classList.add("active");
    }
    el.title =
      `${row.scale}+${row.scale} / ${row.scenario} / ${row.label}: ` +
      (row.signedPct == null ? "—" : fmtPct(row.signedPct));
    el.dataset.filter = "bucket";
    el.dataset.value = row.bucket;
    strip.appendChild(el);
  }
}

function renderDeltaFilters() {
  const scales = [...new Set(deltaAll.map((r) => r.scale))].sort((a, b) => a - b);
  const scenarios = [...new Set(deltaAll.map((r) => r.scenario))];
  const klasses = [...new Set(deltaAll.map((r) => r.klass))];
  const root = document.getElementById("delta-filters");
  root.innerHTML =
    `<div class="filter-group"><span class="lbl">Scale</span>` +
    chip("scale", "all", "all") +
    scales.map((s) => chip("scale", s, scaleLabel(s))).join("") +
    `</div>` +
    `<div class="filter-group"><span class="lbl">Scenario</span>` +
    chip("scenario", "all", "all") +
    scenarios.map((s) => chip("scenario", s, s)).join("") +
    `</div>` +
    `<div class="filter-group"><span class="lbl">Class</span>` +
    chip("klass", "all", "all") +
    klasses.map((k) => chip("klass", k, k)).join("") +
    `</div>`;
}

function renderDeltaTable() {
  const tbody = document.getElementById("delta-body");
  const rows = sortDeltaRows(deltaAll.filter(matchesDeltaFilter));
  document.getElementById("delta-shown").textContent =
    `Showing ${rows.length} of ${deltaAll.length} compared metrics` +
    (deltaFilter.bucket === "all" ? "" : ` · bucket ${DELTA_BUCKETS.find((b) => b.id === deltaFilter.bucket)?.label ?? ""}`);

  for (const th of document.querySelectorAll("#delta-table th[data-sort]")) {
    th.classList.remove("sort-asc", "sort-desc");
    if (th.dataset.sort === deltaSort.key) {
      th.classList.add(deltaSort.dir === "asc" ? "sort-asc" : "sort-desc");
    }
  }

  tbody.innerHTML = "";
  if (rows.length === 0) {
    tbody.innerHTML = '<tr><td colspan="8">No metrics match these filters.</td></tr>';
    return;
  }
  for (const m of rows) {
    const tone = deltaTone(m.signedPct);
    const tr = document.createElement("tr");
    tr.className = "delta-row";
    tr.style.background = tone.bg;
    tr.style.setProperty("--delta-border", tone.border);
    const pct = m.status === "missing" || m.signedPct == null ? "—" : fmtPct(m.signedPct);
    const width = Math.min(Math.abs(m.signedPct ?? 0), 50) / 50 * 50;
    const left = (m.signedPct ?? 0) < 0 ? 28 - width : 28;
    tr.innerHTML =
      `<td>${statusBadge(m.status)}</td>` +
      `<td>${scaleLabel(m.scale)}</td>` +
      `<td>${esc(m.scenario)}</td>` +
      `<td>${esc(m.label)}<br><code>${esc(m.key)}</code></td>` +
      `<td class="num">${fmt(m.baseline)}</td>` +
      `<td class="num">${fmt(m.current)}</td>` +
      `<td class="num">${m.delta == null ? "—" : fmt(m.delta)}</td>` +
      `<td class="num"><div class="delta-pct">${pct}` +
      `<span class="delta-bar" title="±${DISPLAY_DELTA_PCT}% display bar (CI limit ${m.allowedPct}%)"><i style="left:${left}px;width:${width}px;background:${tone.fill}"></i></span>` +
      `</div></td>`;
    tbody.appendChild(tr);
  }
}

function renderDeltaView() {
  renderDeltaPills();
  renderDeltaFilters();
  renderDeltaHist();
  renderDeltaTable();
}

function onDeltaClick(ev) {
  const btn = ev.target.closest("[data-filter]");
  if (!btn) return;
  const group = btn.dataset.filter;
  const value = btn.dataset.value;
  if (deltaFilter[group] === value && value !== "all") {
    deltaFilter[group] = "all";
  } else {
    deltaFilter[group] = value;
  }
  renderDeltaView();
}

function onDeltaSort(ev) {
  const th = ev.target.closest("th[data-sort]");
  if (!th) return;
  const key = th.dataset.sort;
  if (deltaSort.key === key) {
    deltaSort.dir = deltaSort.dir === "asc" ? "desc" : "asc";
  } else {
    deltaSort = { key, dir: key === "label" || key === "scenario" || key === "status" ? "asc" : "desc" };
  }
  renderDeltaTable();
}

/**
 * Baseline comparison (replaces the old deltaVsLastRun section, which compared
 * against a scratch file and could report "hasPrevious: true" with zero rows).
 */
function fillDelta() {
  const cmp = data.comparison;
  const section = document.getElementById("delta-section");
  const heading = document.getElementById("delta-heading");
  const note = document.getElementById("delta-note");

  section.hidden = false;
  if (!deltaBound) {
    document.getElementById("delta-pills").addEventListener("click", onDeltaClick);
    document.getElementById("delta-filters").addEventListener("click", onDeltaClick);
    document.getElementById("delta-hist").addEventListener("click", onDeltaClick);
    document.getElementById("delta-strip").addEventListener("click", onDeltaClick);
    document.getElementById("delta-table").querySelector("thead").addEventListener("click", onDeltaSort);
    deltaBound = true;
  }

  if (!cmp || !cmp.hasBaseline) {
    if (heading) heading.textContent = "Baseline comparison";
    document.getElementById("delta-hist").innerHTML = "";
    document.getElementById("delta-strip").innerHTML = "";
    document.getElementById("delta-pills").innerHTML = "";
    document.getElementById("delta-filters").innerHTML = "";
    document.getElementById("delta-shown").textContent = "";
    document.getElementById("delta-body").innerHTML =
      '<tr><td colspan="8">No baseline yet — run <code>make perf-baseline</code> ' +
      "(chromium, in CI).</td></tr>";
    if (note) {
      note.textContent =
        "The baseline is a committed chromium artifact; only chromium gates CI.";
    }
    return;
  }

  const warnings = [];
  if (cmp.environmentWarnings?.length) {
    warnings.push(`Environment differs from baseline: ${cmp.environmentWarnings.join("; ")}`);
  }
  if (cmp.gatingSkipped) {
    warnings.push("Gating skipped — run is not comparable to the baseline.");
  }
  warnings.push(
    `CI gates: ${cmp.totals.regressions} regression(s) at per-metric limits ` +
      `(timing typically +${cmp.runs?.[0]?.metrics?.find((m) => m.toleranceClass === "timing")?.allowedPct ?? 100}%).`,
  );
  if (note) note.textContent = warnings.join(" ");

  deltaAll = flattenComparison(cmp);
  const nReg = deltaAll.filter((r) => r.status === "regression").length;
  const nImp = deltaAll.filter((r) => r.status === "improvement").length;
  if (heading) {
    heading.textContent =
      `Baseline comparison — ${nReg} regression(s), ` +
      `${nImp} improvement(s) at ±${DISPLAY_DELTA_PCT}%, ${deltaAll.length} compared`;
  }
  deltaFilter = { status: "all", scale: "all", scenario: "all", klass: "all", bucket: "all" };
  deltaSort = { key: "signedPct", dir: "desc" };
  renderDeltaView();
}

function fmt(n) {
  if (typeof n !== "number" || !Number.isFinite(n)) return "—";
  return Math.abs(n) >= 1000 ? Math.round(n).toLocaleString() : String(n);
}

function fmtPct(n) {
  if (typeof n !== "number" || !Number.isFinite(n)) return "—";
  return `${n > 0 ? "+" : ""}${n}%`;
}

/** Ranked cross-browser findings. */
function fillHeadlines() {
  const list = document.getElementById("headlines");
  const section = document.getElementById("headlines-section");
  if (!list || !section) return;
  const items = data.headlines ?? [];
  if (items.length === 0) {
    section.hidden = true;
    return;
  }
  section.hidden = false;
  list.innerHTML = "";
  for (const h of items) {
    const li = document.createElement("li");
    const cls =
      h.severity === "critical" || h.severity === "high"
        ? "sev-high"
        : h.severity === "medium"
          ? "sev-med"
          : "sev-low";
    li.className = cls;
    li.innerHTML =
      `<span class="badge sev">${esc(h.severity)}</span> ` +
      `<span class="cat">${esc(h.category)}</span> ${esc(h.summary)}`;
    list.appendChild(li);
  }
}

/** Reproducibility header. */
function fillEnvironment() {
  const el = document.getElementById("env-line");
  if (!el) return;
  const env = data.environment ?? {};
  const git = data.git ?? {};
  const parts = [
    env.browser ? `${env.browser} ${env.browserVersion}` : null,
    env.playwrightVersion ? `playwright ${env.playwrightVersion}` : null,
    env.cpuCount ? `${env.cpuCount} CPUs` : null,
    env.workers !== undefined ? `workers=${env.workers}` : null,
    env.scales ? `scales=[${env.scales.join(", ")}]` : null,
    git.sha ? `rev ${git.sha.slice(0, 12)}${git.dirty ? " (dirty)" : ""}` : null,
  ].filter(Boolean);
  el.textContent = parts.join(" · ");
}

async function main() {
  try {
    data = await loadResults();
    fillMeta();
    fillEnvironment();
    fillHeadlines();
    fillSummary();
    fillSlowestContributors();
    fillDelta();
    renderScalingCharts();
    renderPerRowCharts();
    renderDomCharts();
    fillRuns();
    fillAnalysisTable();
    document.getElementById("content").hidden = false;
  } catch (e) {
    const el = document.getElementById("load-error");
    el.hidden = false;
    el.textContent = String(e.message || e);
  }
}

void main();
