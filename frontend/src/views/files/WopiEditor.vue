<template>
  <div class="wopi-editor">
    <!-- The access token reaches the editor in a form post, never in the URL. -->
    <form v-if="session" ref="form" :action="session.actionUrl" method="post" :target="frameName" class="wopi-form">
      <input name="access_token" :value="session.accessToken" type="hidden" />
      <input name="access_token_ttl" :value="session.accessTokenTtl" type="hidden" />
    </form>
    <iframe v-if="session" :name="frameName" :title="req.name" class="wopi-frame"
      allow="clipboard-read *; clipboard-write *; fullscreen" allowfullscreen></iframe>
    <p v-if="!loaded" class="wopi-loading">{{ $t("general.loading", { suffix: "..." }) }}</p>
  </div>
  <FloatingActionButton
    v-if="showCloseButton"
    icon="close"
    icon-size="1em"
    size="small"
    position="top-center"
    :slide-in="floatIn"
    :offset="{ top: '6px' }"
    interactive-zone
    :auto-hide="false"
    :label="$t('general.close', { suffix: '' })"
    @click="close"
  />
</template>

<script>
import router from "@/router";
import { state, mutations } from "@/store";
import { removeLastDir } from "@/utils/url";
import { wopiApi } from "@/api";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";

// Opens the current file in a WOPI editor (Collabora Online, OnlyOffice in
// WOPI mode...). The backend hands out the editor URL and an access token; the
// editor then reads and saves the file by calling FileBrowser itself.
export default {
  name: "wopiEditor",
  inheritAttrs: false,
  components: {
    FloatingActionButton,
  },
  data() {
    return {
      session: null,
      loaded: false,
      floatIn: false,
      path: "",
      frameName: `wopi-frame-${Math.random().toString(36).slice(2)}`,
    };
  },
  computed: {
    req() {
      return state.req;
    },
    editorOrigin() {
      return this.session ? new URL(this.session.actionUrl).origin : "";
    },
    // Collabora draws its own close button (closebutton=1) and posts UI_Close.
    showCloseButton() {
      return this.session !== null && this.session.product !== "collabora";
    },
  },
  async mounted() {
    this.path = state.req.path;
    window.addEventListener("message", this.onMessage);
    try {
      this.session = await wopiApi.getSession(state.req);
      await this.$nextTick();
      this.$refs.form.submit();
      setTimeout(() => {
        this.floatIn = true;
      }, 100);
    } catch (error) {
      console.error("Error opening the WOPI editor:", error);
    }
  },
  beforeUnmount() {
    window.removeEventListener("message", this.onMessage);
  },
  methods: {
    frameWindow() {
      return document.getElementsByName(this.frameName)[0]?.contentWindow;
    },
    postToEditor(messageId, values = {}) {
      const target = this.frameWindow();
      if (!target || !this.editorOrigin) return;
      target.postMessage(JSON.stringify({
        MessageId: messageId,
        SendTime: Date.now(),
        Values: values,
      }), this.editorOrigin);
    },
    onMessage(event) {
      if (!this.session || event.origin !== this.editorOrigin) return;
      let msg = event.data;
      if (typeof msg === "string") {
        try {
          msg = JSON.parse(msg);
        } catch {
          return;
        }
      }
      switch (msg?.MessageId) {
        case "App_LoadingStatus":
          if (msg.Values?.Status === "Frame_Ready" || msg.Values?.Status === "Document_Loaded") {
            this.loaded = true;
            this.postToEditor("Host_PostmessageReady");
          }
          break;
        case "UI_Close":
          this.close();
          break;
      }
    },
    close() {
      mutations.replaceRequest({});
      const uri = `${removeLastDir(state.route.path)}/`;
      const filename = this.path.split("/").pop() || "";
      void router.push({ path: uri, hash: `#${filename}` });
    },
  },
};
</script>

<style scoped>
.wopi-editor {
  position: relative;
  width: 100%;
  height: 100vh;
}

.wopi-form {
  display: none;
}

.wopi-frame {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
}

.wopi-loading {
  position: absolute;
  top: 1em;
  left: 0;
  right: 0;
  text-align: center;
}
</style>
