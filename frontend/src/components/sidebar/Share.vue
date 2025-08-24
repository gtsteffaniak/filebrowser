<template>
  <div class="share-sidebar">
    <div class="share-sidebar__content">
      <ShareInfoCard
        v-if="!disableShareCard"
        :hash="hash"
        :token="token"
        :sub-path="subPath"
      />
    </div>
  </div>
</template>

<script>
import ShareInfoCard from "@/components/files/ShareInfoCard.vue";
import { shareInfo } from "@/utils/constants";

export default {
  name: "SidebarShare",
  components: {
    ShareInfoCard,
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
  computed: {
    disableShareCard() {
      return shareInfo.disableShareCard;
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
