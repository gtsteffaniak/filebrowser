<template>
  <div class="card floating">
    <div class="card-title">
      <h2>{{ $t("prompts.newDir") }}</h2>
    </div>

    <div class="card-content">
      <p>{{ $t("prompts.newDirMessage") }}</p>
      <input
        aria-label="New Folder Name"
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
import { filesApi } from "@/api";
import url from "@/utils/url.js";
import { getters, mutations, state } from "@/store"; // Import your custom store
import { notify } from "@/notify";

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
      try {
        event.preventDefault();
        if (this.name === "") return;

        // Build the path of the new directory.
        let uri = decodeURIComponent(state.req.url);
        uri += this.name + "/"; // Ensure the path ends with a slash
        await filesApi.post(uri);
        this.$router.push({ path: state.route.path + encodeURIComponent(this.name) });
      } catch (error) {
        notify.showError(error);
      }
    },
  },
};
</script>
