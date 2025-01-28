<template>
  <div class="card" id="popup-notification">
    <i v-on:click="closePopUp" class="material-icons">close</i>
    <canvas class="notification-spinner" width="100" height="100"></canvas>
    <div id="popup-notification-content">no info</div>
  </div>
</template>

<script>
import { notify } from "@/notify";

export default {
  data: function () {
    return {};
  },
  onMounted() {
    notify.startLoading(0, 360);
  },
  methods: {
    closePopUp() {
      return notify.closePopUp;
    },
    showLoading() {
      notify.startLoading(0, 360);
    },
  },
};
</script>

<style>
meter {
  max-width: 10em;
  width: 90%;
}

.centered {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.padded {
  margin: 1em;
}

.container {
  --uib-size: 5em;
  --uib-color: #1976ed;
  --uib-speed: 1.65s;
  --uib-cube-size: calc(var(--uib-size) / 5.5);
  --uib-arc-1: -80deg;
  --uib-arc-2: 80deg;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  width: var(--uib-size);
  height: calc(var(--uib-size) * 0.51);
}

.animated {
  animation: metronome var(--uib-speed) linear infinite;

  &::after {
    animation: morph var(--uib-speed) linear infinite;
    transition: background-color 0.3s ease;
  }
}

.cube {
  width: var(--uib-cube-size);
  height: calc(var(--uib-size) * 0.5);
  transform-origin: center bottom;

  &::after {
    content: "";
    display: block;
    width: var(--uib-cube-size);
    height: var(--uib-cube-size);
    background-color: var(--uib-color);
    border-radius: 25%;
  }
}

@keyframes metronome {
  0% {
    transform: rotate(var(--uib-arc-1));
  }

  5% {
    transform: rotate(var(--uib-arc-1));
    animation-timing-function: ease-out;
  }

  50% {
    transform: rotate(var(--uib-arc-2));
  }

  55% {
    transform: rotate(var(--uib-arc-2));
    animation-timing-function: ease-out;
  }

  100% {
    transform: rotate(var(--uib-arc-1));
  }
}

@keyframes morph {
  0%,
  5% {
    transform: scaleX(0.75) scaleY(1.25);
    transform-origin: center left;
  }

  12.5% {
    transform: scaleX(1.5);
    transform-origin: center left;
  }

  27.5% {
    transform: scaleX(1);
    transform-origin: center left;
  }

  27.5001%,
  42.5% {
    transform: scaleX(1);
    transform-origin: center right;
  }

  50%,
  52.5% {
    transform: scaleX(0.75) scaleY(1.25);
    transform-origin: center right;
    animation-timing-function: ease-in;
  }

  65% {
    transform: scaleX(1.5);
    transform-origin: center right;
  }

  77.5% {
    transform: scaleX(1);
    transform-origin: center right;
  }

  77.5001%,
  95% {
    transform: scaleX(1);
    transform-origin: center left;
  }

  100% {
    transform: scaleX(0.75) scaleY(1.25);
    transform-origin: center left;
  }
}
</style>
