import * as i18n from "@/i18n";
import moment from "moment";

const mutations = {
  closeHovers: (state) => {
    state.prompts = [];
  },
  toggleShell: (state) => {
    state.showShell = !state.showShell;
  },
  showHover: (state, value) => {
    if (typeof value !== "object") {
      state.prompts.push({
        prompt: value,
        confirm: null,
        action: null,
        props: null,
      });
      return;
    }

    state.prompts.push({
      prompt: value.prompt, // Should not be null
      confirm: value?.confirm,
      action: value?.action,
      props: value?.props,
    });
  },
  showError: (state) => {
    state.prompts.push("error");
  },
  showSuccess: (state) => {
    state.prompts.push("success");
  },
  setLoading: (state, value) => {
    state.loading = value;
  },
  setReload: (state, value) => {
    state.reload = value;
  },
  setUser: (state, value) => {
    if (value === null) {
      state.user = null;
      return;
    }

    let locale = value.locale;

    if (locale === "") {
      locale = i18n.detectLocale();
    }

    moment.locale(locale);
    i18n.default.locale = locale;
    state.user = value;
  },
  setJWT: (state, value) => (state.jwt = value),
  setSession: (state, value) => (state.sessionId = value),
  multiple: (state, value) => (state.multiple = value),
  addSelected: (state, value) => state.selected.push(value),
  removeSelected: (state, value) => {
    let i = state.selected.indexOf(value);
    if (i === -1) return;
    state.selected.splice(i, 1);
  },
  resetSelected: (state) => {
    state.selected = [];
  },
  updateUser: (state, value) => {
    if (typeof value !== "object") return;

    for (let field in value) {
      if (field === "locale") {
        moment.locale(value[field]);
        i18n.default.locale = value[field];
      }

      state.user[field] = value[field];
    }
  },
  updateRequest: (state, value) => {
    const selectedItems = state.selected.map((i) => state.req.items[i]);
    state.oldReq = state.req;
    state.req = value;
    state.selected = [];

    if (!state.req?.items) return;
    state.selected = state.req.items
      .filter((item) => selectedItems.some((rItem) => rItem.url === item.url))
      .map((item) => item.index);
  },
  // Inside your mutations object
  updateListingSortConfig(state, { field, asc }) {
    state.req.sorting.by = field;
    state.req.sorting.asc = asc;
  },

  updateListingItems(state) {
    // Sort the items array based on the sorting settings
    state.req.items.sort((a, b) => {
      const valueA = a[state.req.sorting.by];
      const valueB = b[state.req.sorting.by];
      if (state.req.sorting.asc) {
        return valueA > valueB ? 1 : -1;
      } else {
        return valueA < valueB ? 1 : -1;
      }
    });
  },

  updateClipboard: (state, value) => {
    state.clipboard.key = value.key;
    state.clipboard.items = value.items;
    state.clipboard.path = value.path;
  },
  resetClipboard: (state) => {
    state.clipboard.key = "";
    state.clipboard.items = [];
  },
};

export default mutations;
