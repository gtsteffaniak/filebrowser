<template>
  <div id="editor-container">
    <header-bar>
      <action icon="close" :label="$t('buttons.close')" @action="close()" />
      <title>{{ state.req?.name ?? "" }}</title>
    </header-bar>
    <breadcrumbs base="/files" noLink />
    <errors v-if="error" :errorCode="error.status" />
    <div id="editor" v-if="clientConfig">
      <DocumentEditor
        v-if="clientConfig"
        id="onlyoffice-editor"
        :documentServerUrl="onlyOfficeUrl"
        :config="clientConfig"
      />
    </div>
  </div>
</template>

<script>
import { state } from "@/store";
import { filesApi } from "@/api";
import { notify } from "@/notify";
export default {
  name: "onlyOfficeEditor",
  data() {
    return {
      clientConfig: { value: null },
    };
  },
  async mounted() {
    try {
      const isMobile = window.innerWidth <= 736;
      this.clientConfig.value = await filesApi.OnlyOfficeConfig(state.isMobile);
    } catch (err) {
      notify.showError(err);
    }
  },
};
</script>
