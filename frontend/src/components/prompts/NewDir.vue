<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("prompts.newDir") }}</h2>
    </div>

    <div class="card-content">
      <p>{{ $t("prompts.newDirMessage") }}</p>
      <input
        class="input input--block"
        type="text"
        @keyup.enter="submit"
        v-model.trim="name"
        v-focus
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
        class="button button--flat"
        :aria-label="$t('buttons.create')"
        :title="$t('buttons.create')"
        @click="submit"
      >
        {{ $t("buttons.create") }}
      </button>
    </div>
  </div>
</template>
<script>
import { files as api } from "@/api";
import url from "@/utils/url";
import { getters, mutations } from "@/store"; // Import your custom store
import { showError } from "@/notify";

export default {
  name: "new-dir",
  props: {
    redirect: {
      type: Boolean,
      default: true,
    },
    base: {
      type: [String, null],
      default: null,
    },
  },
  data() {
    return {
      name: "",
    };
  },
  computed: {
    isFiles() {
      return getters.isFiles();
    },
    isListing() {
      return getters.isListing();
    },
  },
  methods: {
    closeHovers() {
      return mutations.closeHovers();
    },
    async submit(event) {
      event.preventDefault();
      if (this.name === "") return;

      // Build the path of the new directory.
      let uri;
      if (this.base) uri = this.base;
      else if (getters.isFiles()) uri = this.$route.path + "/";
      else uri = "/";

      if (!this.isListing) {
        uri = url.removeLastDir(uri) + "/";
      }

      uri += encodeURIComponent(this.name) + "/";
      uri = uri.replace("//", "/");

      try {
        await api.post(uri);
        if (this.redirect) {
          this.$router.push({ path: uri });
        } else if (!this.base) {
          const res = await api.fetch(url.removeLastDir(uri) + "/");
          mutations.updateRequest(res);
        }
      } catch (e) {
        showError(e);
      }

      mutations.closeHovers();
    },
  },
};
</script>
