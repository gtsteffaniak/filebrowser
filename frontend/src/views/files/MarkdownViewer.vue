<template>
  <div id="markedown-viewer" v-html="renderedContent"></div>
</template>

<script lang="ts">
import { marked } from "marked";
import DOMPurify from 'dompurify';
import { state } from "@/store";

export default {
  name: "markdownViewer",
  data() {
    return {
      content: "",
    };
  },
  computed: {
    renderedContent() {
      return DOMPurify.sanitize(marked(this.content));
    },
  },
  mounted() {
    const fileContent =
    state.req.content == "empty-file-x6OlSil" ? "" : state.req.content || "";
    this.content = fileContent
  },
};
</script>

<style>
#markedown-viewer {
  margin: 1em;
  padding: 1em;
  background-color: var(--alt-background);
  border-radius: 1em;
}
</style>
c