<template>
  <table
    class="settings-table border-radius"
    :class="{ 'settings-table--loading': loading }"
    :aria-label="ariaLabel"
    :aria-busy="loading ? 'true' : undefined"
  >
    <thead>
      <tr>
        <th
          v-for="column in columns"
          :key="column.key"
          scope="col"
          :class="[
            alignClass(column),
            headerSortClass(column),
          ]"
          :aria-sort="ariaSortState(column)"
          @click="column.sortable === true ? toggleSort(column) : undefined"
        >
          {{ column.label }}
        </th>
      </tr>
    </thead>
    <tbody>
      <tr v-if="loading" class="settings-table__loading-row">
        <td :colspan="emptyColumnSpan" class="settings-table__loading-cell">
          <div class="settings-table__loading-inner">
            <LoadingSpinner size="small" mode="placeholder" />
          </div>
        </td>
      </tr>
      <template v-else>
        <tr v-for="item in sortedItems" :key="resolvedKey(item)">
          <td
            v-for="column in columns"
            :key="column.key"
            :class="[
              alignClass(column),
              column.narrow === true ? 'settings-table__td--narrow' : '',
            ]"
          >
            <slot
              :name="cellSlot(column.key)"
              :row="item"
              :value="cellValue(item, column)"
              :column="column"
            >
              {{ renderCell(item, column) }}
            </slot>
          </td>
        </tr>
        <tr v-if="sortedItems.length === 0">
          <td
            :colspan="emptyColumnSpan"
            class="settings-table__empty settings-table__empty-cell"
          >
            <slot name="empty">
              <h2 class="message settings-table__lonely">
                <i class="material-symbols-outlined" aria-hidden="true">sentiment_dissatisfied</i>
                <span>{{ lonelyCaption }}</span>
              </h2>
            </slot>
          </td>
        </tr>
      </template>
    </tbody>
  </table>
</template>

<script>
import LoadingSpinner from "@/components/LoadingSpinner.vue";

function defaultCompare(av, bv) {
  if (av === bv) {
    return 0;
  }
  const aStr = av == null ? "" : String(av).toLocaleLowerCase();
  const bStr = bv == null ? "" : String(bv).toLocaleLowerCase();
  return aStr < bStr ? -1 : aStr > bStr ? 1 : 0;
}

/** @returns {unknown} */
function cellValueLookup(row, k) {
  if (!row || k == null) {
    return undefined;
  }
  if (typeof row[k] !== "undefined") {
    return row[k];
  }
  if (typeof k === "string" && k.indexOf(".") !== -1) {
    let cur = row;
    for (const segment of k.split(".")) {
      cur = cur == null ? cur : cur[segment];
    }
    return cur;
  }
  return undefined;
}

export default {
  name: "SettingsTable",

  components: {
    LoadingSpinner,
  },

  props: {
    columns: {
      type: Array,
      required: true,
    },
    /** Row objects; keyed by unique `itemKey` (default `id`). */
    items: {
      type: Array,
      required: true,
    },
    itemKey: {
      type: String,
      default: "id",
    },
    ariaLabel: {
      type: String,
      default: undefined,
    },
    /** i18n key for empty-state caption (icon + label), default `files.lonely`. */
    lonelyMessageKey: {
      type: String,
      default: "files.lonely",
    },
    /** Override default sort detection: first column sortable ascending. */
    defaultSortKey: {
      type: String,
      default: undefined,
    },
    defaultSortDir: {
      type: String,
      default: "asc",
      validator(value) {
        return value === "asc" || value === "desc";
      },
    },
    /** When true, header stays visible and body shows a placeholder spinner until data is ready. */
    loading: {
      type: Boolean,
      default: false,
    },
  },

  data() {
    return {
      sortKey: undefined,
      sortDir: this.defaultSortDir,
    };
  },

  computed: {
    emptyColumnSpan() {
      const n = Array.isArray(this.columns) ? this.columns.length : 0;
      return Math.max(n, 1);
    },
    lonelyCaption() {
      return this.$t(this.lonelyMessageKey);
    },
    sortedItems() {
      const list = [...(this.items ?? [])];
      const cols = Array.isArray(this.columns) ? this.columns : [];
      const activeKey = this.sortKey;
      if (activeKey === undefined || activeKey === null) {
        return list;
      }
      const column = cols.find((c) => c && c.key === activeKey);
      if (!column || !this.isSortEnabled(column)) {
        return list;
      }
      const dirMul = this.sortDir === "desc" ? -1 : 1;
      if (typeof column.sortFn === "function") {
        return list.sort((a, b) => dirMul * column.sortFn(a, b));
      }
      const key = activeKey;
      return list.sort((a, b) => {
        const av = cellValueLookup(a, key);
        const bv = cellValueLookup(b, key);
        const n = defaultCompare(av, bv);
        return dirMul * n;
      });
    },
  },

  watch: {
    columns: {
      deep: true,
      handler() {
        this.applyDefaultSortKey();
      },
    },
    defaultSortKey: {
      immediate: true,
      handler() {
        this.applyDefaultSortKey();
      },
    },
    defaultSortDir(next) {
      this.sortDir = next;
    },
  },

  methods: {
    applyDefaultSortKey() {
      const next = this.defaultSortKey;
      if (typeof next === "string" && next !== "") {
        const cols = Array.isArray(this.columns) ? this.columns : [];
        const col = cols.find((c) => c && c.key === next);
        if (col && this.isSortEnabled(col)) {
          this.sortKey = next;
          this.sortDir = this.defaultSortDir;
        }
        return;
      }
      if (this.sortKey !== undefined && next === "") {
        this.sortKey = undefined;
        this.sortDir = this.defaultSortDir;
      }
    },

    resolvedKey(row) {
      const k = this.itemKey;
      if (typeof row[k] !== "undefined" && row[k] !== null) {
        return String(row[k]);
      }
      return JSON.stringify(row);
    },

    isSortEnabled(column) {
      return Boolean(column && column.key && column.sortable === true);
    },

    ariaSortState(column) {
      if (!this.isSortEnabled(column)) {
        return undefined;
      }
      if (this.sortKey !== column.key) {
        return "none";
      }
      return this.sortDir === "asc" ? "ascending" : "descending";
    },

    toggleSort(column) {
      if (!this.isSortEnabled(column)) {
        return;
      }
      if (this.sortKey !== column.key) {
        this.sortKey = column.key;
        this.sortDir = "asc";
        return;
      }
      if (this.sortDir === "asc") {
        this.sortDir = "desc";
        return;
      }
      this.sortKey = undefined;
      this.sortDir = this.defaultSortDir;
    },

    alignClass(column) {
      const a = column && column.align;
      if (!a || a === "left") {
        return "settings-table__align-left";
      }
      if (a === "center") {
        return "settings-table__align-center";
      }
      if (a === "right") {
        return "settings-table__align-right";
      }
      return "settings-table__align-left";
    },

    headerSortClass(column) {
      if (!this.isSortEnabled(column)) {
        return "settings-table__th--nosort";
      }
      if (this.sortKey === column.key) {
        return this.sortDir === "desc"
          ? ["settings-table__th", "settings-table__th--sorted-desc"]
          : ["settings-table__th", "settings-table__th--sorted-asc"];
      }
      return ["settings-table__th"];
    },

    /** Vue 3 scoped slot naming: alphanumeric + hyphen (keys use camelCase sanitized). */
    cellSlot(columnKey) {
      const slug = String(columnKey).replace(/[^a-zA-Z0-9_-]/g, "-");
      return `cell-${slug}`;
    },

    /** @returns {unknown} */
    cellValue(row, column) {
      return cellValueLookup(row, column.key);
    },

    renderCell(row, column) {
      const v = this.cellValue(row, column);
      if (v !== undefined && v !== null && typeof v === "object") {
        return "";
      }
      return v;
    },
  },
};
</script>

<style scoped>
/* Radius: separate .border-radius utility (_variables.css) so it stays easy to override */
.settings-table {
  width: 100%;
  border-collapse: collapse;
  font-family: inherit;
  font-size: 1em;
  margin: 0;
  background: var(--surfacePrimary);
  border: 1px solid var(--divider);
  outline: 1px solid var(--divider);
  outline-offset: -1px;
  overflow: hidden;
  box-shadow: 0 0 5px rgba(0, 0, 0, 0.05);
}

.settings-table thead th,
.settings-table tbody td {
  padding: 0.5em 0;
  vertical-align: middle;
  border-bottom: 1px solid var(--divider);
}

.settings-table thead th {
  font-size: 1.0625rem;
  font-weight: 500;
  line-height: 1.35;
  padding-top: 0.65em;
  padding-bottom: 0.65em;
}

.settings-table tr > *:first-child {
  padding-left: 1em;
}

.settings-table tr > *:last-child {
  padding-right: 1em;
}

body.rtl .settings-table tr > *:first-child {
  padding-left: unset;
  padding-right: 1em;
}

body.rtl .settings-table tr > *:last-child {
  padding-right: unset;
  padding-left: 1em;
}

.settings-table thead th.settings-table__th {
  color: var(--primaryColor);
  background: color-mix(in srgb, var(--primaryColor) 12%, var(--surfacePrimary));
  cursor: pointer;
  user-select: none;
  position: relative;
  padding-inline-end: 1.75em;
  transition: 0.1s ease all;
}

.settings-table thead th.settings-table__th:hover {
  background: color-mix(in srgb, var(--primaryColor) 22%, var(--surfacePrimary));
}

.settings-table thead th.settings-table__th--nosort {
  color: var(--primaryColor);
  background: color-mix(in srgb, var(--primaryColor) 12%, var(--surfacePrimary));
  cursor: default;
  user-select: auto;
}

.settings-table thead th.settings-table__th--nosort:hover {
  background: color-mix(in srgb, var(--primaryColor) 12%, var(--surfacePrimary));
}

.settings-table thead th.settings-table__th::after {
  content: "↕";
  position: absolute;
  inset-inline-end: 0.5em;
  top: 50%;
  transform: translateY(-50%);
  opacity: 0.35;
  font-size: 0.85em;
  line-height: 1;
}

.settings-table thead th.settings-table__th--sorted-asc::after {
  content: "↑";
  opacity: 1;
}

.settings-table thead th.settings-table__th--sorted-desc::after {
  content: "↓";
  opacity: 1;
}

.settings-table tbody tr:last-child td {
  border-bottom: none;
}

.settings-table tbody tr:hover td {
  background-color: var(--surfaceSecondary);
}

.settings-table--loading thead th.settings-table__th {
  pointer-events: none;
}

.settings-table__loading-cell {
  text-align: center;
  padding: 2em 1em;
  vertical-align: middle;
  border-bottom: none;
}

.settings-table__loading-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  margin: 1em;
}

.settings-table__loading-row:hover td {
  background-color: transparent;
}

.settings-table__td--narrow {
  /* 1px forced too small for multi-button cells; prefer minimum content width */
  width: 1%;
  min-width: max-content;
  white-space: nowrap;
  vertical-align: middle;
}

.settings-table__empty {
  text-align: center;
  color: var(--textSecondary, #757575);
  padding-top: 1rem;
  padding-bottom: 1rem;
  vertical-align: middle;
}

/* Match settings empty UX (listing `.message`; icon stacks above caption) */
.settings-table__empty-cell .settings-table__lonely.message {
  margin: 1em auto;
  width: 100%;
  max-width: 100%;
}

.settings-table tbody .settings-table__align-left {
  text-align: start;
}

.settings-table__align-center {
  text-align: center;
}

.settings-table__align-right {
  text-align: end;
}

body.rtl .settings-table thead th,
body.rtl .settings-table tbody td {
  text-align: start;
}

body.rtl .settings-table__align-right {
  text-align: left;
}

body.rtl .settings-table__align-left {
  text-align: right;
}

body.rtl .settings-table__align-center {
  text-align: center;
}
</style>

