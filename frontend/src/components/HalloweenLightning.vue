<template>
  <div class="halloween-lightning" aria-hidden="true">
    <div class="lightning-flash" :class="{ 'lightning-flash--active': flashActive }" />
    <svg
      v-for="bolt in visibleBolts"
      :key="bolt.id"
      class="lightning-bolt lightning-bolt--strike"
      :style="bolt.style"
      :viewBox="bolt.viewBox"
      xmlns="http://www.w3.org/2000/svg"
    >
      <path class="lightning-bolt-glow" :d="bolt.main" />
      <path
        v-for="(branch, index) in bolt.branches"
        :key="`${bolt.id}-b-${index}`"
        class="lightning-bolt-branch"
        :d="branch"
      />
      <path class="lightning-bolt-core" :d="bolt.main" />
    </svg>
  </div>
</template>

<script>
import {
  createLightningStrike,
  prefersReducedMotion,
} from "@/utils/lightning-bolt.js";

const STRIKE_VISIBLE_MS = 480;
const FIRST_STRIKE_DELAY_MS = 2000;
const MIN_INTERVAL_MS = 3500;
const MAX_INTERVAL_MS = 11000;

export default {
  name: "HalloweenLightning",
  data: () => ({
    flashActive: false,
    visibleBolts: [],
    nextBoltId: 0,
    scheduleTimer: null,
    flashTimers: [],
  }),
  mounted() {
    if (prefersReducedMotion()) return;
    this.scheduleStrike(FIRST_STRIKE_DELAY_MS);
  },
  beforeUnmount() {
    this.clearSchedule();
    this.clearFlashTimers();
    this.visibleBolts = [];
  },
  methods: {
    clearSchedule() {
      if (this.scheduleTimer != null) {
        clearTimeout(this.scheduleTimer);
        this.scheduleTimer = null;
      }
    },
    clearFlashTimers() {
      for (const id of this.flashTimers) clearTimeout(id);
      this.flashTimers = [];
    },
    scheduleStrike(delayMs) {
      this.clearSchedule();
      this.scheduleTimer = setTimeout(() => {
        this.triggerStrike();
        this.scheduleNextStrike();
      }, delayMs);
    },
    scheduleNextStrike() {
      const delay =
        MIN_INTERVAL_MS +
        Math.random() * (MAX_INTERVAL_MS - MIN_INTERVAL_MS);
      this.scheduleStrike(delay);
    },
    triggerStrike() {
      const strike = createLightningStrike();
      const id = this.nextBoltId++;
      this.visibleBolts = [{ id, ...strike }];

      this.runFlashSequence();

      setTimeout(() => {
        this.visibleBolts = this.visibleBolts.filter((b) => b.id !== id);
      }, STRIKE_VISIBLE_MS);
    },
    runFlashSequence() {
      this.clearFlashTimers();
      const setFlash = (on) => {
        this.flashActive = on;
      };
      const schedule = (ms, fn) => {
        this.flashTimers.push(setTimeout(fn, ms));
      };

      setFlash(true);
      schedule(70, () => setFlash(false));
      schedule(130, () => setFlash(true));
      schedule(210, () => setFlash(false));
      schedule(280, () => setFlash(true));
      schedule(360, () => setFlash(false));
    },
  },
};
</script>
