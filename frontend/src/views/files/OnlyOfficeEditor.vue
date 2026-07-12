<template>
  <!-- Conditionally render the DocumentEditor component -->
  <DocumentEditor v-if="ready" id="docEditor" :documentServerUrl="globalVars.onlyOfficeUrl" :config="clientConfig"
    :onLoadComponentError="onLoadComponentError" />
  <div v-else>
    <p>{{ $t("general.loading", { suffix: "..." }) }}</p>
  </div>
  <div @click="close" class="floating-close button" :class="{ 'float-in': floatIn }">
    <i class="material-symbols">close</i>
  </div>
</template>

<script>
import { DocumentEditor } from "@onlyoffice/document-editor-vue";
import { globalVars } from "@/utils/constants";
import router from "@/router";
import { state, mutations } from "@/store";
import { removeLastDir } from "@/utils/url";
import { officeApi } from "@/api";
import { toStandardLocale } from "@/i18n";

// Edge-to-edge (viewport-fit=cover) leaks the notch inset into OnlyOffice's iframe, which paints it as a black
// band; drop cover while the editor is open, restore on close. The runtime flip desyncs Safari/WebKit touch
// hit-testing, so resync via a reflow + scroll.
function resyncViewportHitTesting() {
  void document.documentElement.offsetHeight; // force a synchronous reflow
  window.scrollTo(0, window.scrollY);         // nudge safari to re-map touch coordinates
}

function setViewportFit(fit) {
  const meta = document.querySelector('meta[name="viewport"]');
  const content = meta?.getAttribute("content");
  if (!content?.includes("viewport-fit=")) return;
  meta.setAttribute("content", content.replace(/viewport-fit=\w+/, `viewport-fit=${fit}`));
  requestAnimationFrame(resyncViewportHitTesting); // resync on the next frame
  setTimeout(resyncViewportHitTesting, 250);       // and again once the inset change settles
}

// pending viewport-fit=cover restore, shared across instances so a fast remount can cancel a predecessor's stale restore
let restoreTimer = null;

export default {
  name: "onlyOfficeEditor",
  components: {
    DocumentEditor,
  },
  data() {
    return {
      ready: false,
      clientConfig: {},
      floatIn: false,
      source: "",
      path: "",
      debugMode: false,
      droppedCover: false, // whether we swapped viewport-fit=cover -> auto on open (to restore on close)
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
    // cancel a predecessor's pending restore so it can't flip us back to cover mid-session
    const inheritedDrop = restoreTimer !== null;
    clearTimeout(restoreTimer);
    restoreTimer = null;
    // drop edge-to-edge while the editor is open (see setViewportFit) to avoid the mobile-viewer black band
    const stillCover = document.querySelector('meta[name="viewport"]')
      ?.getAttribute("content")?.includes("viewport-fit=cover") ?? false;
    this.droppedCover = inheritedDrop || stillCover;
    if (stillCover) setViewportFit("auto");

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
    // remove the editor's own iframe first so the viewport flip below happens on a clean layout
    document.getElementById("docEditor")?.querySelectorAll("iframe").forEach((iframe) => iframe.remove());
    // restore edge-to-edge, but only after the iframe teardown settles or safari leaves taps offset
    if (this.droppedCover) {
      clearTimeout(restoreTimer);
      restoreTimer = setTimeout(() => {
        setViewportFit("cover");
        restoreTimer = null;
      }, 250);
    }
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

<style >
.floating-close {
  position: fixed;
  left: 50%;
  transform: translate(-50%, -5em);
  transition: transform 0.4s ease;
  background: var(--surfaceSecondary);
  font-size: .5em;
  top: 0;
}

.float-in {
  transform: translate(-50%, 2.75em);
}

.floating-close i {
  font-size: 2em;
  padding-right: 1em;
  padding-left: 1em;
}
</style>
