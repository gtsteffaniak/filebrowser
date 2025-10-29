<template>
  <div class="error-message">
    <h2 class="message">
      <i class="material-icons">{{ info.icon }}</i>
      <span>{{ $t(info.message) }}</span>
    </h2>
  </div>
</template>

<script>
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
  "share404": {
    icon: "gps_off",
    message: "errors.shareNotFound",
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
};
</script>
