<template>
  <header>
    <action icon="close" :label="$t('buttons.close')" @action="close()" />
    <title class="topTitle">{{ req.name }}</title>
    <action
      v-if="user.perm.modify"
      id="save-button"
      icon="save"
      :label="$t('buttons.save')"
      @action="save()"
    />
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
import { state, mutations } from "@/store";
import { eventBus } from "@/store/eventBus";
import buttons from "@/utils/buttons";
import url from "@/utils/url";
import { showError, showSuccess } from "@/notify";

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
        showSuccess("File Saved!");
      } catch (e) {
        buttons.done(button);
        showError("Error saving file: ", e);
      }
    },
    close() {
      mutations.replaceRequest({});
      let uri = url.removeLastDir(state.route.path) + "/";
      this.$router.push({ path: uri });
    },
  },
};
</script>
