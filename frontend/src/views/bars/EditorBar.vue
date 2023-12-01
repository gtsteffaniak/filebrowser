<template>
  <header-bar>
    <action icon="close" :label="$t('buttons.close')" @action="close()" />
    <title class="topTitle">{{ req.name }}</title>
    <action v-if="user.perm.modify" id="save-button" icon="save" :label="$t('buttons.save')"
      @action="save()" />
  </header-bar>
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
import { mapState } from "vuex";
import { eventBus } from "@/main";

import buttons from "@/utils/buttons";
import url from "@/utils/url";

import HeaderBar from "@/components/header/HeaderBar";
import Action from "@/components/header/Action";

export default {
  name: "editorBar",
  components: {
    HeaderBar,
    Action,
  },
  data: function () {
    return {};
  },
  computed: {
    ...mapState(["req", "user", "currentView"]),
    breadcrumbs() {
      let parts = this.$route.path.split("/");

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
      let uri = url.removeLastDir(this.$route.path) + "/";
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
        eventBus.$emit("handleEditorValueRequest", "data");
        buttons.success(button);
      } catch (e) {
        buttons.done(button);
        this.$showError(e);
      }
    },
    close() {
      this.$store.commit("updateRequest", {});
      let uri = url.removeLastDir(this.$route.path) + "/";
      this.$router.push({ path: uri });
    },
  },
};
</script>