<template>
  <div class="lyrics-view" :class="{ compact }">
    <FloatingActionButton
      v-if="syncedLyrics"
      v-bind="lockButtonProps"
      icon="lock"
      :icon-outlined="scrollLocked"
      size="small"
      :auto-hide="false"
      :label="lockLabel"
      @press="scrollLocked = !scrollLocked"
    />
    <div ref="lyricsScrollable" class="lyrics-scrollable">
      <div v-if="lyrics.length" class="lyrics-list">
        <p v-if="meta" class="lyrics-meta-header" aria-hidden="true">{{ meta }}</p>
        <p
          v-for="(line, index) in lyrics"
          :key="index"
          :class="{ active: syncedLyrics && line.timestamp === lyrics[activeLyricIndex]?.timestamp, 'no-seek': !syncedLyrics }"
          class="lyric-line"
          @click.stop="syncedLyrics && seek(line.timestamp)"
          @keydown.enter.stop.prevent="syncedLyrics && seek(line.timestamp)"
          @keydown.space.stop.prevent="syncedLyrics && seek(line.timestamp)"
          :role="syncedLyrics ? 'button' : undefined"
          :tabindex="syncedLyrics ? 0 : undefined"
          :aria-label="syncedLyrics ? `Seek to ${line.text}` : undefined"
        >
          <template v-if="index === activeLyricIndex && hasFullWordCoverage(line)">
            <span
              v-for="(word, w) in line.words"
              :key="w"
              class="lyric-word"
              :class="{ sung: w <= activeWordIndex, current: w === activeWordIndex }"
            >{{ word.text }}&nbsp;</span>
          </template>
          <template v-else>{{ line.text }}</template>
        </p>
      </div>
      <div v-else class="no-lyrics">
        <i class="material-symbols">lyrics</i>
        <p>{{ t('player.noLyrics') }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useTemplateRef, watch } from "vue";
import type Plyr from "plyr";
import FloatingActionButton from "@/components/settings/FloatingActionButton.vue";
import { useI18n } from "@/i18n/index.ts";
import { isSyncedLyrics } from "@/utils/lyrics.ts";
import type { LyricLines } from "@/utils/lyrics.ts";

const scrollLocked = defineModel<boolean>("scrollLocked");

const props = withDefaults(
  defineProps<{
    lyrics?: LyricLines;
    player?: Plyr | null;
    /** tighter spacing for the mobile view */
    compact?: boolean;
  }>(),
  {
    lyrics: () => [],
    player: null,
    compact: false,
  }
);

const { t } = useI18n();

const lyricsScrollable = useTemplateRef<HTMLElement>("lyricsScrollable");

const activeLyricIndex = ref(-1);
const activeWordIndex = ref(-1);

const syncedLyrics = computed(() => isSyncedLyrics(props.lyrics));
// "Title - Artist" shown above the lyrics from the [ti:] [ar:] tags on lyrics (if present)
const meta = computed(() => [props.lyrics.lrcMeta?.title, props.lyrics.lrcMeta?.artist].filter(Boolean).join(" - "));
const lockLabel = computed(() => (scrollLocked.value ? t("player.unlockLyrics") : t("player.lockLyrics")));
const lockButtonProps = computed(() =>
  props.compact
    ? { position: "bottom-right", offset: { bottom: "calc(env(safe-area-inset-bottom, 0px) + 6rem)" } }
    : { positionMode: "absolute", edgeOffset: { top: "0.5em", right: "0.5em" } }
);

function hasFullWordCoverage(line: LyricLines[number]) {
  const words = line.words;
  if (!words?.length) return false;
  return words.map((word) => word.text).join(" ") === line.text;
}

// Seek the player to the given timestamp (in milliseconds)
function seek(timestampMs: number) {
  const player = props.player;
  if (player) player.currentTime = timestampMs / 1000;
}

function findActiveIndex(items: readonly { timestamp: number }[], currentMs: number) {
  let idx = -1;
  let best = Number.NEGATIVE_INFINITY;
  for (let i = 0; i < items.length; i++) {
    const timestamp = items.at(i)?.timestamp ?? Number.POSITIVE_INFINITY;
    if (timestamp <= currentMs && timestamp > best) {
      best = timestamp;
      idx = i;
    }
  }
  return idx;
}

// Update the active lyric line (and the active word within it, for elrc) from the player time.
function syncLyrics() {
  const player = props.player;
  if (!player || !syncedLyrics.value) return;
  const lyrics = props.lyrics;
  const currentMs = player.currentTime * 1000;
  const first = findActiveIndex(lyrics, currentMs);
  if (first !== activeLyricIndex.value) {
    activeLyricIndex.value = first;
    activeWordIndex.value = -1;
  }
  const words = first >= 0 ? lyrics.at(first)?.words : null;
  activeWordIndex.value = words?.length ? findActiveIndex(words, currentMs) : -1;
}

function scrollToActiveLine() {
  if (scrollLocked.value) return;
  const active = lyricsScrollable.value?.querySelector(".lyric-line.active");
  if (active) {
    active.scrollIntoView({ behavior: "smooth", block: "center" });
  }
}

async function scrollAfterRender() {
  await nextTick();
  scrollToActiveLine();
}

// a destroyed player throws on both, and there is nothing left to listen to
function listen(player: Plyr | null | undefined, subscribe: boolean) {
  try {
    if (subscribe) player?.on("timeupdate", syncLyrics);
    else player?.off("timeupdate", syncLyrics);
  } catch {
    /* ignore */
  }
}

watch(
  () => props.player,
  (player, oldPlayer) => {
    listen(oldPlayer, false);
    listen(player, true);
    syncLyrics();
  },
  { immediate: true }
);

watch(
  () => props.lyrics,
  () => {
    activeLyricIndex.value = -1;
    activeWordIndex.value = -1;
    syncLyrics();
    scrollAfterRender();
  }
);

watch(activeLyricIndex, scrollAfterRender);
watch(scrollLocked, (locked) => {
  if (!locked) scrollAfterRender();
});

onMounted(scrollToActiveLine);
onBeforeUnmount(() => listen(props.player, false));
</script>

<style scoped>
.lyrics-view {
  position: relative;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.lyrics-scrollable {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
  text-align: center;
  color: var(--textPrimary);
  scrollbar-width: none;
}

.lyrics-scrollable::-webkit-scrollbar {
  display: none;
}

.lyrics-list {
  padding-top: 1em;
  padding-bottom: 1em;
}

.lyrics-meta-header {
  padding: 0 0 0.6em;
  margin: 0;
  opacity: 0.55;
  font-size: 0.8rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
  cursor: default;
  user-select: none;
}

.lyric-line {
  padding: 0.5em 0;
  opacity: 0.6;
  cursor: pointer;
  transition: opacity 0.25s ease, color 0.25s ease, font-size 0.25s ease, transform 0.25s ease;
  transform: scale(1);
  transform-origin: center;
  overflow-wrap: break-word;
  font-size: 1.15rem;
}

.lyric-line:hover {
  opacity: 1;
}

.lyric-line.active {
  opacity: 1;
  font-weight: 700;
  color: var(--primaryColor);
  font-size: 1.33rem;
  animation: lyric-line-in 0.3s ease;
}

.lyric-line.no-seek {
  cursor: default;
}

@keyframes lyric-line-in {
  0% { transform: scale(0.98); }
  100% { transform: scale(1); }
}

.lyric-word {
  display: inline-block;
  opacity: 0.4;
  transform: scale(1);
  transition: opacity 0.35s ease, transform 0.35s cubic-bezier(0.25, 0.8, 0.25, 1);
}

.lyric-word.sung {
  opacity: 1;
}

.lyric-word.current {
  transform: scale(1.05);
}

.no-lyrics {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--textSecondary);
  gap: 0.5em;
}

.no-lyrics i {
  font-size: 3rem;
  opacity: 0.5;
}

/* Mobile */
.compact .lyrics-scrollable {
  padding: 0 1em;
}

.compact .lyrics-list {
  padding: 0;
}

.compact .lyrics-meta-header {
  padding-top: 0.2em;
}

.compact .lyric-line {
  padding: 0.2em 0;
  opacity: 0.5;
}

.compact .lyric-line:hover {
  opacity: 0.85;
}

.compact .lyric-line.active {
  opacity: 1;
}

.compact .lyric-line:first-child {
  padding-top: 0;
}
</style>
