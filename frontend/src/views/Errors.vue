<template>
  <div>
    <h2 class="message">
      <i class="material-icons">{{ info.icon }}</i>
      <span>{{ $t(info.message) }}</span>
    </h2>
  </div>
</template>

<script>
import { state } from "@/store";
import { router } from "@/router";
const errors = {
  0: {
    icon: "cloud_off",
    message: "errors.connection",
  },
  403: {
    icon: "error",
    message: "errors.forbidden",
  },
  404: {
    icon: "gps_off",
    message: "errors.notFound",
  },
  500: {
    icon: "error_outline",
    message: "errors.internal",
  },
};

export default {
  name: "errors",
  components: {},
  props: ["errorCode", "showHeader"],
  computed: {
    info() {
      return errors[this.errorCode] ? errors[this.errorCode] : errors[500];
    },
  },
  mounted() {
    window.addEventListener("keydown", this.keyEvent);
  },
  methods: {
    keyEvent(event) {
      const { key, ctrlKey, metaKey, which } = event;
      if (key == "Backspace") {
        // go back
        let currentPath = state.route.path.replace(/\/+$/, "");
        let newPath = currentPath.substring(0, currentPath.lastIndexOf("/"));
        router.push({ path: newPath });
      }
    },
  },
};
</script>
