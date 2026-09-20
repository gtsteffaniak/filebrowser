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

let data;
let interactionChart;
let domChart;
let compareChart;

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

function fillRuns() {
  const filterBrowser = document.getElementById("runs-browser-filter").value;
  const tbody = document.getElementById("runs-body");
  tbody.innerHTML = "";
  const runs = (data.runs ?? []).filter(
    (r) => filterBrowser === "all" || r.browser === filterBrowser,
  );
  for (const run of runs) {
    const dom = run.metrics.dom || {};
    const probe = run.metrics.probe || {};
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
  const filterBrowser = document.getElementById("analysis-browser-filter").value;
  const tbody = document.getElementById("analysis-body");
  tbody.innerHTML = "";
  const rows = (data.analysisTable ?? []).filter(
    (r) => filterBrowser === "all" || r.browser === filterBrowser,
  );
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

function browserSelect(id, browsers, onChange, includeAll) {
  const sel = document.getElementById(id);
  sel.innerHTML = "";
  if (includeAll) {
    const all = document.createElement("option");
    all.value = "all";
    all.textContent = "all browsers";
    sel.appendChild(all);
  }
  for (const b of browsers) {
    const o = document.createElement("option");
    o.value = b;
    o.textContent = b;
    sel.appendChild(o);
  }
  sel.onchange = () => onChange(sel.value);
  onChange(sel.value || browsers[0]);
}

function renderInteractionChart(browser) {
  const series = data.charts.interactionMsByScale[browser];
  const labels = data.scales.map(scaleLabel);
  const datasets = Object.keys(series).map((scenario) => ({
    label: scenario,
    data: series[scenario],
    backgroundColor: SCENARIO_COLORS[scenario] || "#888",
  }));
  if (interactionChart) interactionChart.destroy();
  interactionChart = new Chart(document.getElementById("chart-interaction"), {
    type: "bar",
    data: { labels, datasets },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: { legend: { position: "bottom" } },
      scales: {
        y: { beginAtZero: true, title: { display: true, text: "ms" } },
      },
    },
  });
}

function renderDomChart(browser) {
  const d = data.charts.domAtLoad[browser];
  const labels = d.scales.map(scaleLabel);
  if (domChart) domChart.destroy();
  domChart = new Chart(document.getElementById("chart-dom"), {
    type: "line",
    data: {
      labels,
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
      plugins: { legend: { position: "bottom" } },
      scales: { y: { beginAtZero: true } },
    },
  });
}

function renderCompareChart(scale) {
  const scenarios = ["load", "scroll", "resize", "select"];
  const labels = scenarios;
  const datasets = data.browsers.map((browser) => {
    const series = data.charts.interactionMsByScale[browser];
    const scaleIdx = data.scales.indexOf(Number(scale));
    return {
      label: browser,
      data: scenarios.map((s) => series[s]?.[scaleIdx] ?? 0),
      backgroundColor: BROWSER_COLORS[browser] || "#888",
    };
  });
  if (compareChart) compareChart.destroy();
  compareChart = new Chart(document.getElementById("chart-compare"), {
    type: "bar",
    data: { labels, datasets },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      plugins: { legend: { position: "bottom" } },
      scales: {
        y: { beginAtZero: true, title: { display: true, text: "ms" } },
      },
    },
  });
}

/**
 * Baseline comparison (replaces the old deltaVsLastRun section, which compared
 * against a scratch file and could report "hasPrevious: true" with zero rows).
 */
function fillDelta() {
  const cmp = data.comparison;
  const section = document.getElementById("delta-section");
  const tbody = document.getElementById("delta-body");
  const heading = document.getElementById("delta-heading");
  const note = document.getElementById("delta-note");

  if (!cmp || !cmp.hasBaseline) {
    section.hidden = false;
    if (heading) heading.textContent = "Baseline comparison";
    tbody.innerHTML =
      '<tr><td colspan="5">No baseline yet — run <code>make perf-baseline</code> ' +
      "(chromium, in CI).</td></tr>";
    if (note) {
      note.textContent =
        "The baseline is a committed chromium artifact; only chromium gates CI.";
    }
    return;
  }

  section.hidden = false;
  if (heading) {
    heading.textContent =
      `Baseline comparison — ${cmp.totals.regressions} regression(s), ` +
      `${cmp.totals.improvements} improvement(s), ${cmp.totals.compared} compared`;
  }

  const warnings = [];
  if (cmp.environmentWarnings?.length) {
    warnings.push(`Environment differs from baseline: ${cmp.environmentWarnings.join("; ")}`);
  }
  if (cmp.gatingSkipped) {
    warnings.push("Gating skipped — run is not comparable to the baseline.");
  }
  if (note) note.textContent = warnings.join(" ");

  tbody.innerHTML = "";
  // Show regressions first, then improvements, then a sample of passes.
  const order = { regression: 0, missing: 1, improvement: 2, pass: 3 };
  const rows = (cmp.runs ?? [])
    .flatMap((r) =>
      r.metrics.map((m) => ({ ...m, scale: r.scale, scenario: r.scenario })),
    )
    .filter((m) => m.status !== "pass")
    .sort((a, b) => order[a.status] - order[b.status]);

  if (rows.length === 0) {
    tbody.innerHTML =
      `<tr><td colspan="5">All ${cmp.totals.compared} compared metrics are within tolerance.</td></tr>`;
    return;
  }

  for (const m of rows.slice(0, 200)) {
    const tr = document.createElement("tr");
    const cls =
      m.status === "regression"
        ? "sev-high"
        : m.status === "missing"
          ? "sev-med"
          : "sev-low";
    tr.className = cls;
    tr.innerHTML =
      `<td>${esc(m.label)}<br><code>${esc(m.key)}</code></td>` +
      `<td>${scaleLabel(m.scale)} / ${esc(m.scenario)}</td>` +
      `<td class="num">${fmt(m.baseline)}</td>` +
      `<td class="num">${fmt(m.current)}</td>` +
      `<td class="num">${m.status === "missing" ? "—" : fmtPct(m.deltaPct)} ` +
      `<span class="limit">(limit ${m.allowedPct}%)</span></td>`;
    tbody.appendChild(tr);
  }
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

function fillScaleSelect() {
  const sel = document.getElementById("compare-scale");
  sel.innerHTML = "";
  for (const s of data.scales) {
    const o = document.createElement("option");
    o.value = String(s);
    o.textContent = scaleLabel(s);
    sel.appendChild(o);
  }
  sel.onchange = () => renderCompareChart(sel.value);
  renderCompareChart(String(data.scales[data.scales.length - 1]));
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
    fillScaleSelect();
    browserSelect("browser-interaction", data.browsers, renderInteractionChart);
    browserSelect("browser-dom", data.browsers, renderDomChart);
    browserSelect(
      "runs-browser-filter",
      data.browsers,
      () => fillRuns(),
      true,
    );
    browserSelect(
      "analysis-browser-filter",
      data.browsers,
      () => fillAnalysisTable(),
      true,
    );
    document.getElementById("content").hidden = false;
  } catch (e) {
    const el = document.getElementById("load-error");
    el.hidden = false;
    el.textContent = String(e.message || e);
  }
}

void main();
