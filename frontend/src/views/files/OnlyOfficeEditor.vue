<template>
  <!-- Conditionally render the DocumentEditor component -->
  <DocumentEditor v-if="ready" id="docEditor" :documentServerUrl="globalVars.onlyOfficeUrl" :config="clientConfig"
    :onLoadComponentError="onLoadComponentError" />
  <div v-else>
    <p>{{ $t("general.loading", { suffix: "..." }) }}</p>
  </div>
  <FloatingActionButton
    icon="close"
    icon-size="1em"
    size="small"
    position="top-center"
    :slide-in="floatIn"
    :offset="{ top: '6px' }"
    interactive-zone
    :auto-hide="false"
    :label="$t('general.close', { suffix: '' })"
    @press="close"
  />
</template>

<script>
import { DocumentEditor } from "@onlyoffice/document-editor-vue";
import { globalVars } from "@/utils/constants.ts";
import { router } from "@/router/index.ts";
import { state, mutations, getters } from "@/store/index.ts";
import { removeLastDir } from "@/utils/url.js";
import { officeApi } from "@/api/index.js";
import { toStandardLocale } from "@/i18n/index.ts";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";

export default {
  name: "onlyOfficeEditor",
  inheritAttrs: false,
  components: {
    DocumentEditor,
    FloatingActionButton,
  },
  data() {
    return {
      ready: false,
      clientConfig: { document: null, editorConfig: null },
      floatIn: false,
      source: "",
      path: "",
      debugMode: false,
    };
  },
  computed: {
    req() {
      return state.req;
    },
    onlyOfficeUrl() {
      return globalVars.onlyOfficeUrl;
    },
    globalVars() {
      return globalVars;
    },
  },
  async mounted() {
    this.source = state.req.source;
    this.path = state.req.path;

    // Show debug prompt if debug mode enabled
    if (state.user.debugOffice) {
      this.debugMode = true;
      this.showDebugPrompt();
    }

    // Perform the setup and fetch config from backend
    try {
      const configData = await officeApi.getConfig(state.req);
      configData.type = getters.isMobile() ? "mobile" : "desktop";
      this.clientConfig = configData;
      console.log("OnlyOffice client config received:", this.clientConfig);

      if (state.user?.locale !== "en") {
        this.clientConfig.editorConfig.lang = toStandardLocale(state.user.locale);
      }

      this.ready = true;

      setTimeout(() => {
        this.floatIn = true;
      }, 100);

    } catch (error) {
      console.error("Error during OnlyOffice setup:", error);
    }
  },
  beforeUnmount() {
    if (window.DocsAPI) delete window.DocsAPI;
    const iframes = document.querySelectorAll('iframe');
    for (const iframe of iframes) {
      if (iframe.src.includes('onlyoffice')) {
        iframe.remove();
      }
    }
  },
  methods: {
    close() {
      const { path, source } = state.req;
      mutations.replaceRequest({});
      if (path) {
        state.pendingHighlight = { path, source };
      }
      void router.push({ path: `${removeLastDir(state.route.path)}/` });
    },

    showDebugPrompt() {
      if (!this.debugMode) return;

      mutations.showPrompt({
        name: 'OfficeDebug',
        props: {
          onlyOfficeUrl: this.onlyOfficeUrl,
          source: this.source,
          path: this.path,
        },
      });
    },

    onLoadComponentError(errorInfo) {
      console.error("OnlyOffice component load error:", errorInfo);
    },
  },
};
</script>
