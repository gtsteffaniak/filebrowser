<template>
  <header>
    <action icon="close" :label="$t('buttons.close')" @action="close()" />
    <title class="topTitle">{{ req.name }}</title>
    <action
      v-if="user.permissions.modify"
      id="save-button"
      icon="save"
      :label="$t('buttons.save')"
      @action="save()"
    />
    <action v-else icon="hide_source" />
  </header>
</template>

<style>
.flexbar {
  display: flex;
  flex-direction: block;
  justify-content: space-between;
}

.topTitle {
  display: flex;
  justify-content: center;
}
</style>

<script>
import { state } from "@/store";
import { eventBus } from "@/store/eventBus";
import buttons from "@/utils/buttons";
import url from "@/utils/url.js";
import { notify } from "@/notify";
import router from "@/router";

import Action from "@/components/Action.vue";

export default {
  name: "editorBar",
  components: {
    Action,
  },
  data: function () {
    return {};
  },
  computed: {
    user() {
      return state.user;
    },
    req() {
      return state.req;
    },
    breadcrumbs() {
      let parts = state.route.path.split("/");

      if (parts[0] === "") {
        parts.shift();
      }

      if (parts[parts.length - 1] === "") {
        parts.pop();
      }

      let breadcrumbs = [];

      for (let i = 0; i < parts.length; i++) {
        breadcrumbs.push({ name: decodeURIComponent(parts[i]) });
      }

      breadcrumbs.shift();

      if (breadcrumbs.length > 3) {
        while (breadcrumbs.length !== 4) {
          breadcrumbs.shift();
        }

        breadcrumbs[0].name = "...";
      }

      return breadcrumbs;
    },
  },
  created() {
    window.addEventListener("keydown", this.keyEvent);
  },
  beforeUnmount() {
    window.removeEventListener("keydown", this.keyEvent);
  },
  methods: {
    back() {
      let uri = url.removeLastDir(state.route.path) + "/";
      this.$router.push({ path: uri });
    },
    keyEvent(event) {
      if (!event.ctrlKey && !event.metaKey) {
        return;
      }

      if (String.fromCharCode(event.which).toLowerCase() !== "s") {
        return;
      }

      event.preventDefault();
      this.save();
    },
    async save() {
      const button = "save";
      buttons.loading("save");
      try {
        eventBus.emit("handleEditorValueRequest", "data");
        buttons.success(button);
        notify.showSuccess("File Saved!");
      } catch (e) {
        buttons.done(button);
        notify.showError("Error saving file: ", e);
      }
    },
    close() {
      router.go(-1)
    },
  },
};
</script>
