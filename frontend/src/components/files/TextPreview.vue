<template>
  <div ref="root" class="text-preview" :class="{ 'text-preview--dark': isDark }" aria-hidden="true">
    <span v-if="status === 'loading'" class="material-symbols text-preview-placeholder">{{ PLACEHOLDER_ICON }}</span>
    <div v-else-if="status === 'ready' && csvTable" class="text-preview-body text-preview-table-wrap">
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
    <div v-else-if="status === 'ready' && renderedHtml" class="text-preview-body text-preview-html" v-html="renderedHtml"></div>
    <pre v-else-if="status === 'ready' && highlighted !== null" class="text-preview-body text-preview-plain hljs" v-html="highlighted"></pre>
    <pre v-else-if="status === 'ready' && payload" class="text-preview-body text-preview-plain">{{ payload.content }}</pre>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from "vue";
import { getters } from "@/store";
import { applyHighlightTheme, loadHljs } from "@/utils/highlight.ts";
import { parseDelimited } from "@/utils/delimited.ts";
import {
  fetchTextPreview,
  getRenderedTextPreview,
  languageFromPreviewUrl,
  setRenderedTextPreview,
  textPreviewCacheKey,
} from "@/utils/textPreview.ts";
import {
  renderMarkdownTile,
  sanitizeTileHtml,
  type TextPreviewPayload,
} from "@/utils/textPreviewRender.ts";

const props = defineProps<{
  /** Text preview API URL (see getTextPreviewURL / getTextPreviewURLPublic). */
  url: string;
  /** Modified time, used with the URL as the cache key. */
  modified?: string | null;
  /** Lowercase extension without the dot; derived from the URL when omitted. */
  language?: string | null;
}>();

const emit = defineEmits<(e: "error") => void>();

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

/** Cache key shared with the tile and the popup, so rendering happens once per file. */
const cacheKey = computed(() => textPreviewCacheKey(props.url, props.modified ?? null));

/** Language for highlighting/CSV, preferring the caller-provided extension. */
const language = computed(() => props.language ?? languageFromPreviewUrl(props.url));

// Only markdown, html, and rtf render as HTML. Everything else (plain, css, code) is shown as escaped text.
// Rendered HTML is cached so scroll-back and hover do not re-parse and re-sanitize.
const renderedHtml = computed(() => {
  const data = payload.value;
  if (!data) {
    return "";
  }
  if (data.kind !== "markdown" && data.kind !== "html" && data.format !== "html") {
    return "";
  }
  const key = cacheKey.value;
  const cached = getRenderedTextPreview(key);
  if (cached !== undefined) {
    return cached;
  }
  const html = data.kind === "markdown" ? renderMarkdownTile(data.content) : sanitizeTileHtml(data.content);
  setRenderedTextPreview(key, html);
  return html;
});

// CSV/TSV as a table: first row is the header. A truncated file may end mid-row, so that row is dropped.
const csvTable = computed<{ header: string[]; body: string[][] } | null>(() => {
  const data = payload.value;
  if (data?.kind !== "csv") {
    return null;
  }
  const delimiter = language.value === "tsv" ? "\t" : ",";
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
  const data = payload.value;
  if (!data || !HIGHLIGHT_KINDS.has(data.kind)) {
    highlighted.value = null;
    return;
  }
  const lang = language.value;
  if (!lang) {
    highlighted.value = null;
    return;
  }
  // Highlighted output is theme-independent, so it can be cached and reused.
  const cacheKeyWithLang = `${cacheKey.value}|hl`;
  const cached = getRenderedTextPreview(cacheKeyWithLang);
  if (cached !== undefined) {
    highlighted.value = cached;
    return;
  }
  highlighted.value = null;
  try {
    const hljs = await loadHljs();
    if (!hljs.getLanguage(lang)) {
      return; // unknown language: keep the plain text
    }
    await applyHighlightTheme(isDark.value);
    const html = hljs.highlight(data.content, { language: lang, ignoreIllegals: true }).value;
    if (token === highlightId) {
      // highlight.js escapes its input; sanitizing again is cheap defense in depth.
      const safe = sanitizeTileHtml(html);
      setRenderedTextPreview(cacheKeyWithLang, safe);
      highlighted.value = safe;
    }
  } catch (err) {
    console.error("Failed to highlight text preview:", err);
  }
}

watch([payload, language, isDark], () => {
  void updateHighlight();
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
  /* Lay out at a readable width and let the tile clip, instead of wrapping aggressively */
  min-width: var(--text-preview-min-width, 320px);
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
</style>
