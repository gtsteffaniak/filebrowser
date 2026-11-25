<template>
  <div
    class="video-skip-feedback-layer"
    :class="{
      'video-skip-feedback-layer--visible': skipVisible,
      'video-skip-feedback-layer--left': skipSide === 'left',
      'video-skip-feedback-layer--right': skipSide === 'right',
    }"
    aria-hidden="true"
  >
    <i :key="skipKey" class="material-symbols video-skip-feedback-layer__icon">{{ skipIcon }}</i>
  </div>
  <div :class="['playback-toast', toastVisible ? 'visible' : '']">
    <i class="material-symbols">{{ toastIcon }}</i>
    <span>{{ toastMessage }}</span>
    <span v-if="toastLoopState" :class="['status-indicator', toastLoopState === 'on' ? 'status-on' : 'status-off']"></span>
  </div>
  <div v-if="speedToastVisible" class="playback-toast visible">
    <i class="material-symbols">speed</i>
    <span>{{ speedToastMessage }}</span>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  skipVisible: boolean;
  skipSide: "left" | "right";
  skipIcon: string;
  skipKey: number;
  toastVisible: boolean;
  toastIcon: string;
  toastMessage: string;
  toastLoopState?: "on" | "off";
  speedToastVisible: boolean;
  speedToastMessage: string;
}>();
</script>

<style scoped>
.video-skip-feedback-layer {
  position: absolute;
  inset: 0;
  z-index: 8;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.video-skip-feedback-layer--left {
  justify-content: flex-start;
  padding-left: min(22%, 7rem);
}

.video-skip-feedback-layer--right {
  justify-content: flex-end;
  padding-right: min(22%, 7rem);
}

.video-skip-feedback-layer i.material-symbols.video-skip-feedback-layer__icon {
  flex-shrink: 0;
  font-size: clamp(2.5rem, 7vmin, 6rem);
  line-height: 1;
  color: rgb(255 255 255 / 96%);
  filter: drop-shadow(0 2px 16px rgb(0 0 0 / 85%));
  opacity: 0;
  transform: scale(0.55);
  font-variation-settings:
    "FILL" 1,
    "wght" 500,
    "GRAD" 0,
    "opsz" 40;
}

.video-skip-feedback-layer--visible
  i.material-symbols.video-skip-feedback-layer__icon {
  animation: video-skip-feedback-pop 0.7s cubic-bezier(0.34, 1.56, 0.64, 1)
    forwards;
}

@keyframes video-skip-feedback-pop {
  0% {
    opacity: 0;
    transform: scale(0.55);
  }
  28% {
    opacity: 1;
    transform: scale(1.12);
  }
  100% {
    opacity: 0;
    transform: scale(1);
  }
}

.playback-toast {
  position: fixed;
  bottom: 50px;
  left: 50%;
  transform: translateX(-50%);
  background: rgb(0 0 0 / 80%);
  color: white;
  padding: 15px 25px;
  border-radius: 8px;
  font-size: 1.1rem;
  display: flex;
  align-items: center;
  gap: 10px;
  z-index: 10000;
  pointer-events: none;
  user-select: none;
  opacity: 0;
  transition: opacity 0.3s ease;
  box-shadow: 0 4px 12px rgb(0 0 0 / 30%);
}

.playback-toast.visible {
  opacity: 1;
}

.playback-toast .material-symbols {
  font-size: 24px;
  color: white;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.status-indicator {
  display: inline-block;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  margin-left: 10px;
}

.status-on {
  background: #4caf50;
}

.status-off {
  background: #f44336;
}
</style>
