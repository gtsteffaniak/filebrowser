<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("prompts.rename") }}</h2>
    </div>

    <div class="card-content">
      <p>
        {{ $t("prompts.renameMessage") }} <code>{{ oldName() }}</code
        >:
      </p>
      <input
        class="input input--block"
        v-focus
        type="text"
        @keyup.enter="submit"
        v-model.trim="name"
      />
    </div>

    <div class="card-action">
      <button
        class="button button--flat button--grey"
        @click="closeHovers"
        :aria-label="$t('buttons.cancel')"
        :title="$t('buttons.cancel')"
      >
        {{ $t("buttons.cancel") }}
      </button>
      <button
        @click="submit"
        class="button button--flat"
        type="submit"
        :aria-label="$t('buttons.rename')"
        :title="$t('buttons.rename')"
      >
        {{ $t("buttons.rename") }}
      </button>
    </div>
  </div>
</template>
<script>
import url from "@/utils/url.js";
import { filesApi } from "@/api";
import { state, getters, mutations } from "@/store";

export default {
  name: "rename",
  data() {
    return {
      name: "",
    };
  },
  created() {
    this.name = this.oldName();
  },
  computed: {
    req() {
      return state.req;
    },
    selected() {
      return state.selected;
    },
    selectedCount() {
      return state.selectedCount;
    },
    isListing() {
      return getters.isListing();
    },
    closeHovers() {
      return mutations.closeHovers;
    },
  },
  methods: {
    cancel() {
      mutations.closeHovers();
    },
    oldName() {
      if (!this.isListing) {
        return state.req.name;
      }

      if (getters.selectedCount() === 0 || getters.selectedCount() > 1) {
        return;
      }

      return state.req.items[this.selected[0]].name;
    },
    async submit() {
      let oldLink = "";
      let newLink = "";

      if (!this.isListing) {
        oldLink = state.req.url;
      } else {
        oldLink = state.req.items[this.selected[0]].url;
      }

      newLink = url.removeLastDir(oldLink) + "/" + encodeURIComponent(this.name);

      await filesApi.moveCopy([{ from: oldLink, to: newLink }], "move");
      if (!this.isListing) {
        this.$router.push({ path: newLink });
        return;
      }

      setTimeout(() => {
        mutations.setReload(true);
      }, 50);

      mutations.closeHovers();
    },
  },
};
</script>
