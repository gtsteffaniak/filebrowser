<template>
  <div class="card-title">
    <h2>{{ $t("prompts.newDir") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.newDirMessage") }}</p>
    <input aria-label="New Folder Name" class="input" type="text" @keyup.enter="submit" v-model.trim="name"
      v-focus />
  </div>

  <div class="card-action">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('buttons.cancel')"
      :title="$t('buttons.cancel')">
      {{ $t("buttons.cancel") }}
    </button>
    <button class="button button--flat" :aria-label="$t('buttons.create')" :title="$t('buttons.create')"
      @click="submit">
      {{ $t("buttons.create") }}
    </button>
  </div>
</template>
<script>
import { filesApi } from "@/api";
import { getters, mutations, state } from "@/store"; // Import your custom store
import { notify } from "@/notify";
import { goToItem } from "@/utils/url";

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
        await filesApi.post(state.req.source, state.req.path + "/" + this.name + "/");
        goToItem(state.req.source, state.req.path + "/" + this.name);
      } catch (error) {
        notify.showError(error);
      }
    },
  },
};
</script>
