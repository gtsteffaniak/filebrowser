<template>
  <div class="card" id="popup-notification">
    <i v-on:click="closePopUp" class="material-icons">close</i>
    <canvas class="notification-spinner hidden" width="100" height="100"></canvas>
    <div id="popup-notification-content"></div>
  </div>
</template>

<script>
import { notify } from "@/notify";
export default {
  name: "notifications",
  data: function () {
    return {};
  },
  methods: {
    closePopUp() {
      return notify.closePopUp();
    },
  },
};
</script>

<style>
canvas.notification-spinner {
  color: #fff;
}
#popup-notification {
  z-index: 6;
}
</style>
