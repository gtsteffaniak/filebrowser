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

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef } from "vue";
import {
  createLightningStrike,
  prefersReducedMotion,
  type LightningStrike,
} from "@/utils/lightning-bolt";

type Bolt = LightningStrike & { id: number };

type Timer = ReturnType<typeof setTimeout>;

const STRIKE_VISIBLE_MS = 480;
const FIRST_STRIKE_DELAY_MS = 2000;
const MIN_INTERVAL_MS = 3500;
const MAX_INTERVAL_MS = 11000;

const flashActive = ref(false);
const visibleBolts = shallowRef<Bolt[]>([]);

let nextBoltId = 0;
let scheduleTimer: Timer | null = null;
let flashTimers: Timer[] = [];

function clearSchedule() {
  if (scheduleTimer != null) {
    clearTimeout(scheduleTimer);
    scheduleTimer = null;
  }
}

function clearFlashTimers() {
  for (const id of flashTimers) clearTimeout(id);
  flashTimers = [];
}

function scheduleStrike(delayMs: number) {
  clearSchedule();
  scheduleTimer = setTimeout(() => {
    triggerStrike();
    scheduleNextStrike();
  }, delayMs);
}

function scheduleNextStrike() {
  const delay =
    MIN_INTERVAL_MS + Math.random() * (MAX_INTERVAL_MS - MIN_INTERVAL_MS);
  scheduleStrike(delay);
}

function triggerStrike() {
  const strike = createLightningStrike();
  const id = nextBoltId++;
  visibleBolts.value = [{ id, ...strike }];

  runFlashSequence();

  setTimeout(() => {
    visibleBolts.value = visibleBolts.value.filter((b) => b.id !== id);
  }, STRIKE_VISIBLE_MS);
}

function runFlashSequence() {
  clearFlashTimers();
  const setFlash = (on: boolean) => {
    flashActive.value = on;
  };
  const schedule = (ms: number, fn: () => void) => {
    flashTimers.push(setTimeout(fn, ms));
  };

  setFlash(true);
  schedule(70, () => setFlash(false));
  schedule(130, () => setFlash(true));
  schedule(210, () => setFlash(false));
  schedule(280, () => setFlash(true));
  schedule(360, () => setFlash(false));
}

onMounted(() => {
  if (prefersReducedMotion()) return;
  scheduleStrike(FIRST_STRIKE_DELAY_MS);
});

onBeforeUnmount(() => {
  clearSchedule();
  clearFlashTimers();
  visibleBolts.value = [];
});
</script>
