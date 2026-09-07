<template>
  <!-- Conditionally render the DocumentEditor component -->
  <DocumentEditor v-if="ready" id="docEditor" :documentServerUrl="globalVars.onlyOfficeUrl" :config="clientConfig"
    :onLoadComponentError="onLoadComponentError" />
  <div v-else>
    <p>{{ $t("general.loading", { suffix: "..." }) }}</p>
  </div>
  <FloatingActionButton
    icon="close"
    position="top-center"
    :slide-in="floatIn"
    :auto-hide="false"
    :label="$t('general.close', { suffix: '' })"
    @click="close"
  />
</template>

<script>
import { DocumentEditor } from "@onlyoffice/document-editor-vue";
import { globalVars } from "@/utils/constants";
import router from "@/router";
import { state, mutations } from "@/store";
import { removeLastDir } from "@/utils/url";
import { officeApi } from "@/api";
import { toStandardLocale } from "@/i18n";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";

export default {
  name: "onlyOfficeEditor",
  components: {
    DocumentEditor,
    FloatingActionButton,
  },
  data() {
    return {
      ready: false,
      clientConfig: {},
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
      configData.type = state.isMobile ? "mobile" : "desktop";
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
    iframes.forEach(iframe => {
      if (iframe.src.includes('onlyoffice')) {
        iframe.remove();
      }
    });
  },
  methods: {
    close() {
      mutations.replaceRequest({});
      const uri = `${removeLastDir(state.route.path)}/`;
      const filename = this.path.split('/').pop() || "";
      void router.push({ path: uri, hash: `#${filename}` });
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
