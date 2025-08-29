<template>
  <div class="card-title">
    <h2>{{ $t("prompts.newFile") }}</h2>
  </div>

  <div class="card-content">
    <p>{{ $t("prompts.newFileMessage") }}</p>
    <input class="input" aria-label="FileName Field" v-focus type="text" @keyup.enter="submit"
      v-model.trim="name" />
  </div>

  <div class="card-action">
    <button class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('buttons.cancel')"
      :title="$t('buttons.cancel')">
      {{ $t("buttons.cancel") }}
    </button>
    <button class="button button--flat" @click="submit" :aria-label="$t('buttons.create')"
      :title="$t('buttons.create')">
      {{ $t("buttons.create") }}
    </button>
  </div>
</template>
<script>
import { state } from "@/store";
import { filesApi } from "@/api";
import { getters, mutations } from "@/store"; // Import your custom store
import { notify } from "@/notify";
import { goToItem } from "@/utils/url";

export default {
  name: "new-file",
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
    closeHovers() {
      return mutations.closeHovers;
    },
  },
  methods: {
    async submit(event) {
      try {
        event.preventDefault();
        if (this.name === "") return;
        await filesApi.post(state.req.source,state.req.path + "/" + this.name, "", true );
        goToItem(state.req.source, state.req.path + "/" + this.name);
      } catch (error) {
        notify.showError(error);
      }
    },
  },
};
</script>
