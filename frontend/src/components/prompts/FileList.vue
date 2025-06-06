<template>
  <div>
    <div>Source: {{ sourcePath.source }} </div> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
    <div aria-label="filelist-path" class="searchContext">{{$t('search.path')}} {{ sourcePath.path }}</div>
    <ul class="file-list">
      <li
        @click="itemClick"
        @touchstart="touchstart"
        @dblclick="next"
        role="button"
        tabindex="0"
        :aria-label="item.name"
        :aria-selected="selected == item.url"
        :key="item.name"
        v-for="item in items"
        :data-url="item.url"
      >
        {{ item.name }}
      </li>
    </ul>
  </div>
</template>

<script>
import { state, mutations } from "@/store";
import url from "@/utils/url.js";
import { extractSourceFromPath } from "@/utils/url.js";
import { filesApi } from "@/api";

export default {
  name: "file-list",
  data: function () {
    return {
      items: [],
      touches: {
        id: "",
        count: 0,
      },
      selected: null,
      current: window.location.pathname,
    };
  },
  computed: {
    sourcePath() {
      return extractSourceFromPath(decodeURIComponent(this.current), "files");
    },
  },
  mounted() {
    this.fillOptions(state.req);
  },
  methods: {
    fillOptions(req) {
      // Sets the current path and resets
      // the current items.
      this.current = req.url;
      this.items = [];

      this.$emit("update:selected", this.current);

      // If the path isn't the root path,
      // show a button to navigate to the previous
      // directory.
      if (req.url !== "/files/") {
        this.items.push({
          name: "..",
          url: url.removeLastDir(req.url) + "/",
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
          url: item.url,
        });
      }
    },
    next: function (event) {
      // Retrieves the URL of the directory the user
      // just clicked in and fill the options with its
      // content.
      let uri = event.currentTarget.dataset.url;
      filesApi.fetchFiles(uri).then(this.fillOptions);
    },
    touchstart(event) {
      let url = event.currentTarget.dataset.url;

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
      // If the element is already selected, unselect it.
      if (this.selected === event.currentTarget.dataset.url) {
        this.selected = null;
        this.$emit("update:selected", this.current);
        return;
      }

      // Otherwise select the element.
      this.selected = event.currentTarget.dataset.url;
      this.$emit("update:selected", this.selected);
    },
    createDir: async function () {
      mutations.showHover({
        name: "newDir",
        action: null,
        confirm: null,
        props: {
          redirect: false,
          base: this.current === state.route.path ? null : this.current,
        },
      });
    },
  },
};
</script>
