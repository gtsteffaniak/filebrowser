<template>
  <div class="card-title">
    <h2>{{ $t("prompts.fileInfo") }}</h2>
  </div>

  <div class="card-content">
    <p v-if="selected.length > 1">
      {{ $t("prompts.filesSelected", { count: selected.length }) }}
    </p>

    <p class="break-word" v-if="selected.length < 2">
      <strong>{{ $t("prompts.displayName") }}</strong> {{ name }}
    </p>
    <p v-if="!dir || selected.length > 1">
      <strong>{{ $t("prompts.size") }}:</strong> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      <span id="content_length"></span> {{ humanSize }}
    </p>
    <p v-if="!dir || selected.length > 1">
      <strong>{{ $t('prompts.typeName') }}</strong>
      <span id="content_length"></span> {{ type }}
    </p>
    <p v-if="selected.length < 2" :title="modTime">
      <strong>{{ $t("prompts.lastModified", { suffix: ":" }) }}</strong>
      <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
    </p>

    <template v-if="dir && selected.length === 0">
      <p>
        <strong>{{ $t("prompts.numberFiles", { suffix: ":" }) }}</strong> {{ req.numFiles }}
        <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      </p>
      <p>
        <strong>{{ $t("prompts.numberDirs", { suffix: ":" }) }}</strong> {{ req.numDirs }}
        <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      </p>
    </template>

    <template v-if="!dir">
      <p>
        <strong>MD5: </strong> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <code><a @click="checksum($event, 'md5')">{{ $t("prompts.show") }}</a></code>
      </p>
      <p>
        <strong>SHA1: </strong> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <code><a @click="checksum($event, 'sha1')">{{ $t("prompts.show") }}</a></code>
      </p>
      <p>
        <strong>SHA256: </strong> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <code><a @click="checksum($event, 'sha256')">{{ $t("prompts.show") }}</a></code>
      </p>
      <p>
        <strong>SHA512: </strong> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        <code><a @click="checksum($event, 'sha512')">{{ $t("prompts.show") }}</a></code>
      </p>
    </template>
  </div>

  <div class="card-action">
    <button type="submit" @click="closeHovers" class="button button--flat" :aria-label="$t('buttons.close')"
      :title="$t('buttons.close')">
      {{ $t("buttons.close") }}
    </button>
  </div>
</template>
<script>
import { getHumanReadableFilesize } from "@/utils/filesizes";
import { formatTimestamp } from "@/utils/moment";
import { filesApi } from "@/api";
import { state, getters, mutations } from "@/store"; // Import your custom store

export default {
  name: "info",
  computed: {
    closeHovers() {
      return mutations.closeHovers;
    },
    req() {
      return state.req;
    },
    selected() {
      return state.selected;
    },
    selectedCount() {
      return getters.selectedCount();
    },
    isListing() {
      return getters.isListing();
    },
    humanSize() {
      if (state.isSearchActive) {
        return getHumanReadableFilesize(state.selected[0].size);
      }
      if (getters.selectedCount() === 0 || !this.isListing) {
        return getHumanReadableFilesize(state.req.size);
      }

      let sum = 0;

      for (let selected of this.selected) {
        sum += state.req.items[selected].size;
      }

      return getHumanReadableFilesize(sum);
    },
    humanTime() {
      if (state.isSearchActive) {
        return "unknown";
      }
      if (getters.selectedCount() === 0) {
        return formatTimestamp(state.req.modified, state.user.locale);
      }
      return formatTimestamp(
        state.req.items[this.selected[0]].modified,
        state.user.locale
      );
    },
    modTime() {
      if (state.isSearchActive) {
        return "";
      }
      return new Date(Date.parse(state.req.modified)).toLocaleString();
    },
    name() {
      if (state.isSearchActive) {
        return state.selected[0].name;
      }
      return getters.selectedCount() === 0
        ? state.req.name
        : state.req.items[this.selected[0]].name;
    },
    type() {
      if (state.isSearchActive) {
        return state.selected[0].type;
      }
      return getters.selectedCount() === 0
        ? state.req.type
        : state.req.items[this.selected[0]].type;
    },
    dir() {
      if (state.isSearchActive) {
        return state.selected[0].type === "directory";
      }
      return (
        getters.selectedCount() > 1 ||
        (getters.selectedCount() === 0
          ? state.req.type == "directory"
          : state.req.items[this.selected[0]].type == "directory")
      );
    },
  },
  methods: {
    async checksum(event, algo) {
      event.preventDefault();
      let link;
      if (state.isSearchActive) {
        const hash = await filesApi.checksum(state.selected[0].source, state.selected[0].path, algo);
        event.target.innerHTML = hash;
        return;
      }
      if (getters.selectedCount()) {
        link = state.req.items[this.selected[0]].path;
      } else {
        link = state.route.path;
      }

      const hash = await filesApi.checksum(state.sources.current, link, algo);
      event.target.innerHTML = hash;
    },
  },
};
</script>
