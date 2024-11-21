import * as i18n from "@/i18n";
import { state } from "./state.js";
import router from "@/router";
import { emitStateChanged } from './eventBus'; // Import the function from eventBus.js
import { usersApi } from "@/api";
import { notify } from "@/notify";

export const mutations = {
  setGallerySize: (value) => {
    state.user.gallerySize = value
    emitStateChanged();
    usersApi.update(state.user,['gallerySize']);
  },
  setActiveSettingsView: (value) => {
    state.activeSettingsView = value;
    router.push({ hash: "#" + value });
    const element = document.getElementById(value);
    if (element) {
      element.scrollIntoView({
        behavior: "smooth",
        block: "center",
        inline: "nearest",
      });
    }
    emitStateChanged();
  },
  setSettings: (value) => {
    state.settings = value;
    emitStateChanged();
  },
  setMobile() {
    state.isMobile = window.innerWidth <= 800
    emitStateChanged();
  },
  toggleDarkMode() {
    mutations.updateCurrentUser({ "darkMode": !state.user.darkMode });
    emitStateChanged();
  },
  toggleSidebar() {
    if (state.user.stickySidebar) {
      localStorage.setItem("stickySidebar", "false");
      mutations.updateCurrentUser({ "stickySidebar": false }); // turn off sticky when closed
      state.showSidebar = false;
    } else {
      state.showSidebar = !state.showSidebar;
    }
    emitStateChanged();
  },
  closeSidebar() {
    if (state.showSidebar) {
      state.showSidebar = false;
      emitStateChanged();
    }
  },
  setUpload(value) {
    state.upload = value;
    emitStateChanged();
  },
  setUsage: (value) => {
    state.usage = value;
    emitStateChanged();
  },
  closeHovers: () => {
    state.prompts = [];
    if (!state.stickySidebar) {
      state.showSidebar = false;
    }
    emitStateChanged();
  },
  showHover: (value) => {
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
  setLoading: (loadType, status) => {
    if (status === false) {
      delete state.loading[loadType];
    } else {
      state.loading = { ...state.loading, [loadType]: true };
    }
    emitStateChanged();
  },
  setReload: (value) => {
    state.reload = value;
    emitStateChanged();
  },
  setCurrentUser: (value) => {
    // If value is null or undefined, emit state change and exit early
    if (!value) {
      state.user = value;
      emitStateChanged();
      return;
    }
    // Ensure locale exists and is valid
    if (!value.locale) {
      value.locale = i18n.detectLocale();  // Default to detected locale if missing
    }
    state.user = value;
    // Emit state change after setting the user and locale
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
    if (value == true) {
      notify.showMultipleSelection()
    } else {
      notify.closePopUp()
    }
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
  setRaw: (value) => {
    state.previewRaw = value;
    emitStateChanged();
  },
  updateCurrentUser: (value) => {
    // Ensure the input is a valid object
    if (typeof value !== "object" || value === null) return;

    // Initialize state.user if it's null
    if (!state.user) {
      state.user = {};
    }

    // Store previous state for comparison
    const previousUser = { ...state.user };

    // Merge the new values into the current user state
    state.user = { ...state.user, ...value };

    // Handle locale change
    if (state.user.locale !== previousUser.locale) {
      state.user.locale = i18n.detectLocale();
      i18n.setLocale(state.user.locale);
      i18n.default.locale = state.user.locale;
    }

    // Update localStorage if stickySidebar exists
    if ('stickySidebar' in state.user) {
      localStorage.setItem("stickySidebar", state.user.stickySidebar);
    }
    // Update users if there's any change in state.user
    if (JSON.stringify(state.user) !== JSON.stringify(previousUser)) {
      usersApi.update(state.user,Object.keys(value));
    }

    // Emit state change event
    emitStateChanged();
  },
  updateRequest: (value) => {
    const selectedItems = state.selected.map((i) => state.req.items[i]);
    state.oldReq = state.req;
    state.req = value;
    state.selected = [];
    if (!state.req?.items) return;
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
    state.user.sorting.by = field;
    state.user.sorting.asc = asc;
    emitStateChanged();
  },
  updateListingItems: () => {
    state.req.items.sort((a, b) => {
      const valueA = a[state.user.sorting.by];
      const valueB = b[state.user.sorting.by];
      if (state.user.sorting.asc) {
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

