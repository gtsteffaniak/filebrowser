<template>
  <div ref="root" class="text-preview" :class="{ 'text-preview--dark': isDark }" aria-hidden="true">
    <span v-if="status === 'loading'" class="material-symbols text-preview-placeholder">{{ PLACEHOLDER_ICON }}</span>
    <!-- Small tiles: markdown/html cannot fit, so show the readable text only -->
    <pre v-else-if="status === 'ready' && compactText !== null" class="text-preview-body text-preview-plain text-preview-compact">{{ compactText }}</pre>
    <div v-else-if="status === 'ready' && csvTable" class="text-preview-body text-preview-table-wrap" :class="{ 'text-preview-compact': compact }">
      <table class="text-preview-table">
        <thead>
          <tr>
            <th v-for="(cell, c) in csvTable.header" :key="c">{{ cell }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, r) in csvTable.body" :key="r">
            <td v-for="(cell, c) in row" :key="c">{{ cell }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else-if="status === 'ready' && renderedHtml" class="text-preview-body text-preview-html" :class="{ 'text-preview-compact': compact }" v-html="renderedHtml"></div>
    <pre v-else-if="status === 'ready' && highlighted !== null" class="text-preview-body text-preview-plain hljs" :class="{ 'text-preview-compact': compact }" v-html="highlighted"></pre>
    <pre v-else-if="status === 'ready' && payload" class="text-preview-body text-preview-plain" :class="{ 'text-preview-compact': compact }">{{ payload.content }}</pre>
    <div v-if="status === 'ready' && payload?.truncated" class="text-preview-fade"></div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from "vue";
import { getters } from "@/store";
import { applyHighlightTheme, loadHljs } from "@/utils/highlight.ts";
import { parseDelimited } from "@/utils/delimited.ts";
import { fetchTextPreview, languageFromPreviewUrl } from "@/utils/textPreview.ts";
import {
  plainTextFromHtml,
  renderMarkdownTile,
  sanitizeTileHtml,
  type TextPreviewPayload,
} from "@/utils/textPreviewRender.ts";

const props = defineProps<{
  /** Text preview API URL (see textPreviewUrlFor). */
  url: string;
  /** Modified time, used with the URL as the cache key. */
  modified?: string | null;
}>();


const emit = defineEmits<(e: "error") => void>();


// Tiles narrower than this cannot fit rendered markdown or HTML, so they show plain text.
const COMPACT_WIDTH_PX = 240;
// Only plain source files are syntax highlighted. Markdown and HTML are rendered instead.
const HIGHLIGHT_KINDS = new Set(["code", "css"]);
// Limits keep the DOM small; the tile only shows its top-left area anyway.
const CSV_MAX_ROWS = 100;
const CSV_MAX_COLUMNS = 30;

const PLACEHOLDER_ICON = "description";

type Status = "loading" | "ready" | "error";

const status = ref<Status>("loading");
const payload = shallowRef<TextPreviewPayload | null>(null);
let requestId = 0;

// Only markdown, html, and rtf render as HTML. Everything else (plain, css, code) is shown as escaped text.
const renderedHtml = computed(() => {
  const data = payload.value;
  if (!data) {
    return "";
  }
  if (data.kind === "markdown") {
    return renderMarkdownTile(data.content);
  }
  if (data.kind === "html" || data.format === "html") {
    return sanitizeTileHtml(data.content);
  }
  return "";
});

const root = ref<HTMLElement | null>(null);
const compact = ref(false);
let resizeObserver: ResizeObserver | null = null;

// Plain text shown on compact tiles. Markdown is shown as its source, HTML as its visible text.
const compactText = computed<string | null>(() => {
  const data = payload.value;
  // CSV keeps its table, HTML keeps its rendering, and code/css keep their highlighting on compact tiles.
  if (!compact.value || !data || data.kind === "csv" || data.kind === "html" || HIGHLIGHT_KINDS.has(data.kind)) {
    return null;
  }
  // Converted RTF is HTML too, so it still falls back to text on compact tiles.
  if (data.format === "html") {
    return plainTextFromHtml(sanitizeTileHtml(data.content));
  }
  return data.content;
});

// CSV/TSV as a table: first row is the header. A truncated file may end mid-row, so that row is dropped.
const csvTable = computed<{ header: string[]; body: string[][] } | null>(() => {
  const data = payload.value;
  if (data?.kind !== "csv") {
    return null;
  }
  const delimiter = languageFromPreviewUrl(props.url) === "tsv" ? "\t" : ",";
  let rows = parseDelimited(data.content, delimiter);
  if (data.truncated && rows.length > 1) {
    rows = rows.slice(0, -1);
  }
  if (rows.length === 0) {
    return null;
  }
  const trim = (row: string[]) => row.slice(0, CSV_MAX_COLUMNS);
  const [header = [], ...body] = rows.slice(0, CSV_MAX_ROWS + 1).map(trim);
  return { header, body };
});

const isDark = computed(() => getters.isDarkMode());
const highlighted = ref<string | null>(null);
let highlightId = 0;

async function updateHighlight(): Promise<void> {
  const token = ++highlightId;
  highlighted.value = null;
  const data = payload.value;
  if (!data || !HIGHLIGHT_KINDS.has(data.kind)) {
    return;
  }
  const language = languageFromPreviewUrl(props.url);
  if (!language) {
    return;
  }
  try {
    const hljs = await loadHljs();
    if (!hljs.getLanguage(language)) {
      return; // unknown language: keep the plain text
    }
    await applyHighlightTheme(isDark.value);
    const html = hljs.highlight(data.content, { language, ignoreIllegals: true }).value;
    if (token === highlightId) {
      // highlight.js escapes its input; sanitizing again is cheap defense in depth.
      highlighted.value = sanitizeTileHtml(html);
    }
  } catch (err) {
    console.error("Failed to highlight text preview:", err);
  }
}

watch([payload, compact, isDark], () => {
  void updateHighlight();
});

onMounted(() => {
  if (root.value && typeof ResizeObserver !== "undefined") {
    resizeObserver = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width ?? 0;
      compact.value = width > 0 && width < COMPACT_WIDTH_PX;
    });
    resizeObserver.observe(root.value);
  }
});

watch(
  [() => props.url, () => props.modified],
  async ([url, modified]) => {
    const current = ++requestId;
    status.value = "loading";
    payload.value = null;
    try {
      const data = await fetchTextPreview(url, modified ?? null);
      if (current !== requestId) return;
      payload.value = data;
      status.value = "ready";
    } catch {
      if (current !== requestId) return;
      status.value = "error";
      emit("error");
    }
  },
  { immediate: true },
);

onBeforeUnmount(() => {
  // Drop any in-flight result so it never updates an unmounted tile.
  requestId++;
  highlightId++;
  resizeObserver?.disconnect();
  resizeObserver = null;
});
</script>

<style scoped>
.text-preview {
  position: relative;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  max-width: 100%;
  max-height: 100%;
  overflow: hidden;
  box-sizing: border-box;
  /* Same background for every text kind: white in light mode, dark in dark mode */
  background: #fff;
  color: #24292f;
  /* Paint and size containment: nothing inside can draw outside the tile, and the
     tile never grows to fit its content. Also lets text scale with the tile width. */
  contain: strict;
  container-type: size;
}

.text-preview.text-preview--dark {
  background: #0d1117;
  color: #c9d1d9;
}

/* The highlight.js theme paints its own background; let the tile background show through */
.text-preview .hljs {
  background: transparent;
}

.text-preview-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  opacity: 0.6;
}

.text-preview-body {
  /* Pinned to the tile so the content can never size the tile or spill out of it */
  position: absolute;
  inset: 0;
  box-sizing: border-box;
  padding: 0.6em;
  margin: 0;
  overflow: hidden;
  font-size: 3.4cqw;
  line-height: 1.35;
  text-align: left;
  overflow-wrap: anywhere;
}

.text-preview-plain {
  font-family: SFMono-Regular, Monaco, Inconsolata, "Liberation Mono", "Courier New", monospace;
  white-space: pre-wrap;
}

/* CSV/TSV table: small text, cells never wrap, the tile clips the overflow */
.text-preview-table-wrap {
  overflow: hidden;
}

.text-preview-table {
  border-collapse: collapse;
  font-size: 1em;
  white-space: nowrap;
}

.text-preview-table th,
.text-preview-table td {
  border: 1px solid var(--divider);
  padding: 0.2em 0.5em;
  max-width: 12em;
  overflow: hidden;
  text-overflow: ellipsis;
}

.text-preview-table th {
  background: var(--background);
  font-weight: 600;
  text-align: left;
}

/* Compact tiles: keep text at a readable minimum size and let the tile clip the rest */
.text-preview-body.text-preview-compact {
  font-size: max(6px, 3.4cqw);
  line-height: 1.3;
}

.text-preview-html :deep(h1),
.text-preview-html :deep(h2),
.text-preview-html :deep(h3),
.text-preview-html :deep(h4) {
  font-weight: 600;
  line-height: 1.2;
  margin: 0.6em 0 0.3em;
}

.text-preview-html :deep(h1) { font-size: 1.8em; }
.text-preview-html :deep(h2) { font-size: 1.45em; }
.text-preview-html :deep(h3) { font-size: 1.2em; }

.text-preview-html :deep(p),
.text-preview-html :deep(ul),
.text-preview-html :deep(ol),
.text-preview-html :deep(blockquote),
.text-preview-html :deep(pre) {
  margin: 0 0 0.6em;
}

.text-preview-html :deep(ul),
.text-preview-html :deep(ol) {
  padding-left: 1.5em;
}

.text-preview-html :deep(pre),
.text-preview-html :deep(code) {
  font-family: SFMono-Regular, Monaco, Inconsolata, "Liberation Mono", "Courier New", monospace;
  background: var(--background);
  border-radius: 0.3em;
}

.text-preview-html :deep(pre) {
  padding: 0.4em;
  white-space: pre-wrap;
}

/* Keep the rendered document inside the tile: nothing wider or taller than the frame */
.text-preview-html :deep(img),
.text-preview-html :deep(video),
.text-preview-html :deep(svg) {
  max-width: 100%;
  height: auto;
}

.text-preview-html :deep(*) {
  max-width: 100%;
  box-sizing: border-box;
}

.text-preview-html :deep(blockquote) {
  border-left: 0.25em solid var(--primaryColor);
  padding-left: 0.6em;
  color: color-mix(in srgb, var(--textPrimary) 75%, transparent);
}

.text-preview-html :deep(table) {
  border-collapse: collapse;
  max-width: 100%;
}

.text-preview-html :deep(th),
.text-preview-html :deep(td) {
  border: 1px solid var(--divider);
  padding: 0.2em 0.4em;
}

.text-preview-html :deep(a) {
  color: var(--primaryColor);
  text-decoration: underline;
}

.text-preview-fade {
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 30%;
  background: linear-gradient(to bottom, transparent, var(--surfacePrimary));
  pointer-events: none;
}
</style>
