<template>
  <div class="card floating">
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
        <strong>{{ $t("prompts.size") }}:</strong>
        <span id="content_length"></span> {{ humanSize }}
      </p>
      <p v-if="selected.length < 2" :title="modTime">
        <strong>{{ $t("prompts.lastModified") }}:</strong> {{ humanTime }}
      </p>

      <template v-if="dir && selected.length === 0">
        <p>
          <strong>{{ $t("prompts.numberFiles") }}:</strong> {{ req.numFiles }}
        </p>
        <p>
          <strong>{{ $t("prompts.numberDirs") }}:</strong> {{ req.numDirs }}
        </p>
      </template>

      <template v-if="!dir">
        <p>
          <strong>MD5: </strong
          ><code
            ><a @click="checksum($event, 'md5')">{{ $t("prompts.show") }}</a></code
          >
        </p>
        <p>
          <strong>SHA1: </strong
          ><code
            ><a @click="checksum($event, 'sha1')">{{ $t("prompts.show") }}</a></code
          >
        </p>
        <p>
          <strong>SHA256: </strong
          ><code
            ><a @click="checksum($event, 'sha256')">{{ $t("prompts.show") }}</a></code
          >
        </p>
        <p>
          <strong>SHA512: </strong
          ><code
            ><a @click="checksum($event, 'sha512')">{{ $t("prompts.show") }}</a></code
          >
        </p>
      </template>
    </div>

    <div class="card-action">
      <button
        type="submit"
        @click="closeHovers"
        class="button button--flat"
        :aria-label="$t('buttons.ok')"
        :title="$t('buttons.ok')"
      >
        {{ $t("buttons.ok") }}
      </button>
    </div>
  </div>
</template>
<script>
import { getHumanReadableFilesize } from "@/utils/filesizes";
import moment from "moment";
import { files as api } from "@/api";
import { state, getters,mutations } from "@/store"; // Import your custom store
import { showError } from "@/notify";

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
      if (this.selectedCount === 0 || !this.isListing) {
        return getHumanReadableFilesize(state.req.size);
      }

      let sum = 0;

      for (let selected of this.selected) {
        sum += state.req.items[selected].size;
      }

      return getHumanReadableFilesize(sum);
    },
    humanTime() {
      if (this.selectedCount === 0) {
        return moment(state.req.modified).fromNow();
      }

      return moment(state.req.items[this.selected[0]].modified).fromNow();
    },
    modTime() {
      return new Date(Date.parse(state.req.modified)).toLocaleString();
    },
    name() {
      return this.selectedCount === 0
        ? state.req.name
        : state.req.items[this.selected[0]].name;
    },
    dir() {
      return (
        this.selectedCount > 1 ||
        (this.selectedCount === 0
          ? state.req.isDir
          : state.req.items[this.selected[0]].isDir)
      );
    },
  },
  methods: {
    async checksum(event, algo) {
      event.preventDefault();

      let link;

      if (this.selectedCount) {
        link = state.req.items[this.selected[0]].url;
      } else {
        link = this.$route.path;
      }

      try {
        const hash = await api.checksum(link, algo);
        event.target.innerHTML = hash;
      } catch (e) {
        showError(e);
      }
    },
  },
};
</script>
