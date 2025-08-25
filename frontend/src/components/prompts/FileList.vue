<template>
  <div>
    <!-- Source Selection Dropdown (only show if multiple sources available) -->
    <div v-if="availableSources.length > 1" class="source-selector" style="margin-bottom: 1rem;">
      <label for="destinationSource" style="display: block; margin-bottom: 0.5rem; font-weight: bold;">
        {{ $t("prompts.destinationSource") }}
      </label>
      <select id="destinationSource" v-model="currentSource" @change="onSourceChange"
          class="input">
        <option v-for="source in availableSources" :key="source" :value="source">
          {{ source }}
        </option>
      </select>
    </div>

    <div>Source: {{ sourcePath.source }} </div> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
    <div aria-label="filelist-path" class="searchContext button clickable">{{$t('search.path')}} {{ sourcePath.path }}</div>
    <ul class="file-list">
      <li
        @click="itemClick"
        @touchstart="touchstart"
        @dblclick="next"
        role="button"
        tabindex="0"
        :aria-label="item.name"
        :aria-selected="selected == item.path"
        :key="item.name"
        v-for="item in items"
        :data-path="item.path"
      >
        {{ item.name }}
      </li>
    </ul>
  </div>
</template>

<script>
import { state, mutations } from "@/store";
import { url } from "@/utils";
import { filesApi } from "@/api";

export default {
  name: "file-list",
  props: {
    browseSource: {
      type: String,
      default: null,
    },
  },
  data: function () {
    const initialSource = this.browseSource || state.req.source;
    // Use current path if browsing the same source as current, otherwise start at root
    const initialPath = this.browseSource && this.browseSource !== state.req.source ? "/" : state.req.path;
    return {
      items: [],
      path: initialPath,
      source: initialSource,
      touches: {
        id: "",
        count: 0,
      },
      selected: null,
      selectedSource: null,
      current: window.location.pathname,
      currentSource: initialSource,
    };
  },
  computed: {
    sourcePath() {
      return { source: this.source, path: this.path };
    },
    effectiveSource() {
      return this.browseSource || state.req.source;
    },
    availableSources() {
      // Get all available sources from state.sources.info
      return state.sources && state.sources.info ? Object.keys(state.sources.info) : [state.req.source];
    },
  },
  watch: {
    browseSource(newSource) {
      if (newSource && newSource !== this.source) {
        this.currentSource = newSource;
        this.resetToSource(newSource);
      }
    },
    currentSource(newSource) {
      if (newSource && newSource !== this.source) {
        this.resetToSource(newSource);
      }
    },
  },
  mounted() {
    // Use currentSource if provided, otherwise use state.req
    const sourceToUse = this.currentSource;
    const pathToUse = this.currentSource !== state.req.source ? "/" : state.req.path;
    const initialReq = {
      ...state.req,
      source: sourceToUse,
      path: pathToUse,
    };
    // Fetch the initial data for the source
    if (this.currentSource !== state.req.source) {
      filesApi.fetchFiles(sourceToUse, pathToUse).then(this.fillOptions);
    } else {
      this.fillOptions(initialReq);
    }
  },
  methods: {
    resetToSource(newSource) {
      // Use current path if browsing the same source as current, otherwise start at root
      const newPath = newSource === state.req.source ? state.req.path : "/";
      // Reset to the appropriate path for the new source
      this.path = newPath;
      this.source = newSource;
      this.selected = null;
      this.selectedSource = null;
      // Fetch files for the new source
      filesApi.fetchFiles(newSource, newPath).then(this.fillOptions);
    },
    fillOptions(req) {
      // Sets the current path and resets
      // the current items.
      this.current = req.path;
      this.source = req.source;
      this.items = [];

      // Emit both path and source
      this.$emit("update:selected", {
        path: this.current,
        source: this.source
      });

      // If the path isn't the root path,
      // show a button to navigate to the previous
      // directory.
      if (req.path !== "/") {
        this.items.push({
          name: "..",
          path: url.removeLastDir(req.path) + "/",
          source: req.source,
        });
      }

      // If this folder is empty, finish here.
      if (req.items === null) return;

      // Otherwise we add every directory to the
      // move options.
      for (let item of req.items) {
        if (item.type != "directory") continue;
        this.items.push({
          name: item.name,
          path: item.path,
          source: item.source || req.source,
        });
      }
    },
    next: function (event) {
      // Retrieves the URL of the directory the user
      // just clicked in and fill the options with its
      // content.
      let path = event.currentTarget.dataset.path;
      let clickedItem = this.items.find(item => item.path === path);
      let sourceToUse = clickedItem ? clickedItem.source : this.source;
      this.path = path;
      this.source = sourceToUse;
      filesApi.fetchFiles(sourceToUse, path).then(this.fillOptions);
    },
    touchstart(event) {
      let url = event.currentTarget.dataset.path;

      // In 300 milliseconds, we shall reset the count.
      setTimeout(() => {
        this.touches.count = 0;
      }, 300);

      // If the element the user is touching
      // is different from the last one he touched,
      // reset the count.
      if (this.touches.id !== url) {
        this.touches.id = url;
        this.touches.count = 1;
        return;
      }

      this.touches.count++;

      // If there is more than one touch already,
      // open the next screen.
      if (this.touches.count > 1) {
        this.next(event);
      }
    },
    itemClick: function (event) {
      if (state.user.singleClick) this.next(event);
      else this.select(event);
    },
    select: function (event) {
      let path = event.currentTarget.dataset.path;
      // If the element is already selected, unselect it.
      if (this.selected === path) {
        this.selected = null;
        this.selectedSource = null;
        this.$emit("update:selected", {
          path: this.current,
          source: this.source
        });
        return;
      }

      // Otherwise select the element.
      this.selected = path;
      let clickedItem = this.items.find(item => item.path === path);
      this.selectedSource = clickedItem ? clickedItem.source : this.source;
      this.$emit("update:selected", {
        path: this.selected,
        source: this.selectedSource
      });
    },
    createDir: async function () {
      mutations.showHover({
        name: "newDir",
        action: null,
        confirm: null,
        props: {
          redirect: false,
          base: this.current === this.path ? null : this.current,
        },
      });
    },
    onSourceChange() {
      this.resetToSource(this.currentSource);
    },
  },
};
</script>
