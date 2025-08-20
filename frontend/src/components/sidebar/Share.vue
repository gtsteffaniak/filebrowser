<template>
  <div class="share-sidebar">
    <div class="share-sidebar__content">
      <ShareInfo
        :hash="hash"
        :token="token"
        :sub-path="subPath"
      />
    </div>
  </div>
</template>

<script>
import ShareInfo from "@/components/files/ShareInfo.vue";

export default {
  name: "SidebarShare",
  components: {
    ShareInfo,
  },
  props: {
    hash: {
      type: String,
      required: true,
    },
    token: {
      type: String,
      required: true,
    },
    subPath: {
      type: String,
      default: "/",
    },
  },
};
</script>

<style scoped>
.share-sidebar {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: auto;
}

.share-sidebar__content {
  flex: 1;
  overflow: auto;
}

</style>
