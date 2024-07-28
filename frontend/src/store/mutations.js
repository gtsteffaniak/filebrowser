import * as i18n from "@/i18n";
import { state } from "./state.js";
import { emitStateChanged } from './eventBus'; // Import the function from eventBus.js

export const mutations = {
  closeHovers: () => {
    state.prompts = [];
    emitStateChanged();
  },
  toggleShell: () => {
    state.showShell = !state.showShell;
    emitStateChanged();
  },
  showHover: (value) => {
    console.log("hover", value)
    if (typeof value === "object") {
      state.prompts.push({
        name: value?.name,
        confirm: value?.confirm,
        action: value?.action,
        props: value?.props,
      });
    } else {
      state.prompts.push({
        name: value,
        confirm: value?.confirm,
        action: value?.action,
        props: value?.props,
      });
    }
    emitStateChanged();
  },
  showError: () => {
    state.prompts.push("error");
    emitStateChanged();
  },
  setLoading: (value) => {
    state.loading = value;
    emitStateChanged();
  },
  setReload: (value) => {
    state.reload = value;
    emitStateChanged();
  },
  setUser: (value) => {
    if (value === null) {
      state.user = null;
      emitStateChanged();
      return;
    }
    if (state.user == null) {
      i18n.default.locale = i18n.detectLocale();
      emitStateChanged();
      return 
    }
    if (value?.locale) {
      state.user.locale = value.locale
    } else if (!state.user?.locale) {
      state.user.locale = i18n.detectLocale();
    }
    i18n.default.locale = locale;
    state.user = value;
    emitStateChanged();
  },
  setJWT: (value) => {
    state.jwt = value;
    emitStateChanged();
  },
  setSession: (value) => {
    state.sessionId = value;
    emitStateChanged();
  },
  setMultiple: (value) => {
    state.multiple = value;
    emitStateChanged();
  },
  addSelected: (value) => {
    state.selected.push(value);
    emitStateChanged();
  },
  removeSelected: (value) => {
    let i = state.selected.indexOf(value);
    if (i === -1) return;
    state.selected.splice(i, 1);
    emitStateChanged();
  },
  resetSelected: () => {
    state.selected = [];
    mutations.setMultiple(false);
    emitStateChanged();
  },
  updateUser: (value) => {
    if (typeof value !== "object") return;
    if (state.user === null) {
      state.user = {};
    }
    for (let field in value) {
      if (field === "locale") {
        i18n.default.locale = value[field];
      }
      state.user[field] = value[field];
    }
    emitStateChanged();
  },
  updateRequest: (value) => {
    state.oldReq = { ...state.req };
    // Ensure the update is reactive
    state.req = { ...state.req, ...value };
    state.selected = [];
    emitStateChanged();
    if (!state.req?.items) return;
    const selectedItems = state.selected.map((i) => state.req.items[i]);
    state.selected = state.req.items
      .filter((item) => selectedItems.some((rItem) => rItem.url === item.url))
      .map((item) => item.index);
  },
  replaceRequest: (value) => {
    state.req = value;
    emitStateChanged();
  },
  setRoute: (value) => {
    state.route = value;
    emitStateChanged();
  },
  updateListingSortConfig: ({ field, asc }) => {
    state.req.sorting.by = field;
    state.req.sorting.asc = asc;
    emitStateChanged();
  },
  updateListingItems: () => {
    state.req.items.sort((a, b) => {
      const valueA = a[state.req.sorting.by];
      const valueB = b[state.req.sorting.by];
      if (state.req.sorting.asc) {
        return valueA > valueB ? 1 : -1;
      } else {
        return valueA < valueB ? 1 : -1;
      }
    });
    emitStateChanged();
  },
  updateClipboard: (value) => {
    state.clipboard.key = value.key;
    state.clipboard.items = value.items;
    state.clipboard.path = value.path;
    emitStateChanged();
  },
  resetClipboard: () => {
    state.clipboard.key = "";
    state.clipboard.items = [];
    emitStateChanged();
  },
};

