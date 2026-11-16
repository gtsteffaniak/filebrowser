<template>
  <div class="card-content no-buttons prompt-panel info-content">
    <div class="info-tabs">
      <SlidingTabs :model-value="activeTab" :tabs="tabs" @update:model-value="activeTab = $event as InfoTab" />
    </div>
    <div class="info-grid" :class="{ 'info-grid--fill': activeTab === 'access' }">
      <div class="info-section" v-if="activeTab === 'basic'">
        <div class="info-item">
          <strong>{{ $t("prompts.displayName") }}</strong>
          <span role="group" aria-label="info display name">{{ item.name }}</span>
        </div>
        <div class="info-item">
          <strong>{{ $t("general.size") }}</strong>
          <span role="group" aria-label="info size">{{ humanSize }}</span>
        </div>
        <div class="info-item">
          <strong>{{ $t("general.type") }}</strong>
          <span role="group" aria-label="info type">{{ item.type }}</span>
        </div>
        <div class="info-item" v-if="humanTime">
          <strong>{{ $t("files.lastModified") }}</strong>
          <span role="group" aria-label="info last modified" :title="modTime">{{ humanTime }}</span>
        </div>
        <div class="info-item" v-if="humanCreatedTime">
          <strong>{{ $t("files.creationTime") }}</strong>
          <span aria-label="info date added" :title="createdTime">{{ humanCreatedTime }}</span>
        </div>
        <div class="info-item" v-if="source">
          <strong>{{ $t("general.source") }}</strong>
          <span role="group" aria-label="info source">{{ source }}</span>
        </div>
        <div class="info-item" v-if="filePath">
          <strong>{{ $t("general.path") }}</strong>
          <span role="group" aria-label="info path" class="break-word">{{ filePath }}</span>
        </div>
        <div class="info-item" v-if="item.hidden !== undefined">
          <strong>{{ $t("prompts.hidden") }}</strong>
          <span role="group" aria-label="info hidden">{{ item.hidden ? "✓" : "✗" }}</span><!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </div>
        <div class="info-item" v-if="item.hasPreview !== undefined">
          <strong>{{ $t("prompts.hasPreview") }}</strong>
          <span role="group" aria-label="info has preview">{{ item.hasPreview ? "✓" : "✗" }}</span><!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
        </div>
        <div class="settings-items" v-if="source && !isShare">
          <ActivityViewerButton :href="activityViewerHref" />
        </div>
      </div>
      <!-- Media metadata -->
      <div class="info-section" v-else-if="activeTab === 'media'">
        <div class="info-item" v-for="info in mediaInfo" :key="info.key">
          <strong>{{ info.label }}</strong>
          <span>{{ info.value }}</span>
        </div>
        <!-- Lyrics -->
        <div class="info-item" v-if="hasLyrics">
          <strong>{{ $t("player.lyrics") }}</strong>
          <span>
            <a class="info-toggle" role="button" tabindex="0" @click="toggleLyrics" @keydown.enter="toggleLyrics">
              {{ showLyrics ? $t("general.hide") : $t("general.show") }}
            </a>
            <span v-if="showLyrics" class="info-multiline">{{ lyricsText }}</span>
          </span>
        </div>
        <!-- Subtitles -->
        <div class="info-item" v-for="(track, idx) in subtitleTracks" :key="track.name">
          <strong>{{ $t("prompts.subtitle", { n: idx + 1 }) }}</strong>
          <span>
            {{ subtitleLabel(track) }}
            <a class="info-toggle" role="button" tabindex="0" @click="toggleSubtitle(track)" @keydown.enter="toggleSubtitle(track)">
              {{ subtitleContent[track.name] !== undefined ? $t("general.hide") : $t("general.show") }}
            </a>
            <span v-if="subtitleContent[track.name] !== undefined" class="info-multiline">{{ subtitleContent[track.name] }}</span>
          </span>
        </div>
        <div class="info-item" v-if="!showMediaContent">
          <span>{{ $t("general.none") }}</span>
        </div>
      </div>
      <div v-else-if="activeTab === 'quota'" class="info-section">
        <div class="settings-items">
          <SettingsButton
            class="info-manage-link"
            :name="t('quotas.title')"
            :description="$t('quotas.openDescription')"
            @activate="openQuotaPrompt"
          />
        </div>
        <div class="info-item">
          <strong>{{ $t("general.enabled") }}</strong>
          <span>{{ quotaEnabled ? $t("general.yes") : $t("general.no") }}</span>
        </div>
        <div v-if="quotaEnabled" class="info-quota-usage">
          <QuotaFolderBar
            class="info-quota-bar"
            :source="source"
            :path="filePath"
            :enabled="true"
            :limit-bytes="quotaLimitBytes"
            :snapshot="quotaSnapshot"
          />
        </div>
      </div>
      <div class="info-section" v-else-if="activeTab === 'additional'">
        <div class="info-item" v-for="info in additionalInfo" :key="info.key">
          <strong>{{ info.label }}</strong>
          <span>{{ info.value }}</span>
        </div>
      </div>
      <div class="info-section" v-else-if="activeTab === 'checksums'">
        <div class="hash-generator">
          <div class="hash-select">
            <label for="hash-algo">{{ $t("prompts.hashAlgorithm") }}</label>
            <div class="form-flex-group">
              <ExpandDropdown
                input-id="hash-algo"
                v-model="selectedHashAlgo"
                class="form-form flat-right"
                :options="hashAlgoOptions"
                :aria-label="$t('prompts.hashAlgorithm')"
              />
              <button type="button" class="button form-button flat-left" @click="generateHash"
                :title="$t('prompts.generate')" :aria-label="$t('prompts.generate')">
                {{ $t("prompts.generate") }}
              </button>
            </div>
          </div>
          <div class="hash-result">
            <label for="hash-result">{{ $t("prompts.hashValue") }}</label>
            <div class="form-flex-group">
              <input id="hash-result" class="input form-form flat-right" type="text" :value="hashResult" readonly
                :placeholder="$t('prompts.selectHashAlgorithm')" />
              <button
                type="button"
                class="button form-button flat-left"
                @click="copyHash"
                :disabled="!hashResult"
                :title="$t('buttons.copyToClipboard')"
                :aria-label="$t('buttons.copyToClipboard')"
              >
                <i class="material-symbols-outlined" style="font-size: 16px;">content_copy</i>
              </button>
            </div>
          </div>
        </div>
      </div>
      <div v-if="accessVisited" v-show="activeTab === 'access'" class="info-section">
        <Access embedded :source-name="source" :path="filePath" />
      </div>
    </div>
  </div>

</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { getHumanReadableFilesize } from "@/utils/filesizes.js";
import { formatTimestamp, fromNow } from "@/utils/moment.ts";
import { copyToClipboard } from "@/utils/clipboard.js";
import { formatDuration } from "@/utils/files.js";
import { resourcesApi, quotasApi, mediaApi } from "@/api/index.js";
import { getters, mutations, state } from "@/store/index.ts";
import type { FileListItem, SubtitleTrack } from "@/store/types.ts";
import { notify } from "@/notify/index.ts";
import { activityViewerPresets } from "@/utils/activityViewerLink.js";
import SlidingTabs from "@/components/settings/SlidingTabs.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ActivityViewerButton from "@/components/settings/ActivityViewerButton.vue";
import SettingsButton from "@/components/settings/SettingsButton.vue";
import QuotaFolderBar from "@/components/prompts/QuotaFolderBar.vue";
import Access from "@/components/prompts/Access.vue";

type InfoTab = "basic" | "media" | "quota" | "access" | "additional" | "checksums";
type HashAlgo = "md5" | "sha1" | "sha256" | "sha512";

interface InfoItem extends FileListItem {
  subtitles?: SubtitleTrack[];
  token?: string;
  hash?: string;
  onlyOfficeId?: string;
}

interface QuotaSnapshot {
  usedBytes?: number;
  reservedBytes?: number;
  measurementStatus?: string;
  limitBytes?: number;
}

interface InfoRow {
  key: string;
  label: string;
  value: string | number;
}

const props = defineProps<{
  item: InfoItem;
  promptId?: string | number | null;
}>();

const { t } = useI18n();

const hashAlgoOptions: { value: HashAlgo; label: string }[] = [
  { value: "md5", label: "MD5" },
  { value: "sha1", label: "SHA1" },
  { value: "sha256", label: "SHA256" },
  { value: "sha512", label: "SHA512" },
];

const activeTab = ref<InfoTab>("basic");
const accessVisited = ref(false);
const selectedHashAlgo = ref<HashAlgo>("md5");
const hashResult = ref("");
const generatingHash = ref(false);
const quotaSnapshot = ref<QuotaSnapshot>({});
const quotaExists = ref(false);
const lyricsText = ref("");
const lyricsLoading = ref(false);
const showLyrics = ref(false);
// Subtitle content by track name
const subtitleContent = ref<Record<string, string>>({});
// Listing items carry no subtitles, they're fetched when the tab opens
const mediaDetails = ref<InfoItem | null>(null);
let mediaRequested = false;

const isShare = getters.isShare();
const source = computed(() => props.item.source || "");
const filePath = computed(() => props.item.path || "");
const isDir = computed(() => props.item.type === "directory");
const isAudio = computed(() => !!props.item.type?.startsWith("audio"));
const isVideo = computed(() => !!props.item.type?.startsWith("video"));
const isAdmin = computed(() => getters.isAdmin());

const humanSize = computed(() => getHumanReadableFilesize(props.item.size || 0));
const humanTime = computed(() => formatDate(props.item.modified));
const modTime = computed(() => relativeDate(props.item.modified));
const humanCreatedTime = computed(() => formatDate(props.item.created));
const createdTime = computed(() => relativeDate(props.item.created));
const activityViewerHref = computed(() => activityViewerPresets.files(source.value, filePath.value));

const additionalInfo = computed<InfoRow[]>(() => {
  const { token, hash, onlyOfficeId } = props.item;
  const rows: [string, string, string | undefined][] = [
    ["token", t("prompts.token"), token],
    ["hash", t("general.hash"), hash],
    ["onlyOfficeId", t("prompts.onlyOfficeId"), onlyOfficeId],
  ];
  return rows.flatMap(([key, label, value]) => (value ? [{ key, label, value }] : []));
});

// the fresh response wins over the (older) listing metadata
const mediaMetadata = computed(() => mediaDetails.value?.metadata || props.item.metadata);
const hasLyrics = computed(() => isAudio.value && !!mediaMetadata.value?.hasLyrics);
const subtitleTracks = computed<SubtitleTrack[]>(() =>
  isVideo.value ? props.item.subtitles || mediaDetails.value?.subtitles || [] : []
);

const mediaInfo = computed<InfoRow[]>(() => {
  const meta = mediaMetadata.value;
  if (!meta || !(isAudio.value || isVideo.value)) return [];
  const rows: [string, string, string | number | undefined][] = [
    ["title", t("prompts.mediaTitle"), meta.title],
    ["artist", t("prompts.artist"), meta.artist],
    ["album", t("prompts.album"), meta.album],
    ["year", t("prompts.year"), meta.year],
    ["genre", t("prompts.genre"), meta.genre],
    ["track", t("prompts.track"), meta.track],
    ["duration", t("files.duration"), meta.duration ? formatDuration(meta.duration) : ""],
    ["container", t("prompts.container"), meta.container],
    ["videoCodec", t("prompts.videoCodec"), meta.videoCodec],
    ["audioCodec", t("prompts.audioCodec"), meta.audioCodec],
  ];
  return rows.flatMap(([key, label, value]) => (value ? [{ key, label, value }] : []));
});
const showMediaContent = computed(
  () => mediaInfo.value.length > 0 || hasLyrics.value || subtitleTracks.value.length > 0
);

const showQuotaSection = computed(
  () => isAdmin.value && !isShare && isDir.value && !!source.value && !!filePath.value
);
const showAccessSection = computed(() => isAdmin.value && !isShare && !!source.value && !!filePath.value);
const quotaLimitBytes = computed(() => quotaSnapshot.value.limitBytes || 0);
const quotaEnabled = computed(() => quotaExists.value && quotaLimitBytes.value > 0);

const tabs = computed(() => {
  const list: { id: InfoTab; label: string }[] = [{ id: "basic", label: t("prompts.basicInfo") }];
  if (isAudio.value || isVideo.value) list.push({ id: "media", label: t("prompts.mediaInfo") });
  if (showQuotaSection.value) list.push({ id: "quota", label: t("quotas.title") });
  if (showAccessSection.value) list.push({ id: "access", label: t("access.rules") });
  if (additionalInfo.value.length > 0) list.push({ id: "additional", label: t("prompts.additionalInfo") });
  if (!isDir.value) list.push({ id: "checksums", label: t("prompts.checksums") });
  return list;
});

watch(activeTab, (tab) => {
  if (tab === "access") accessVisited.value = true;
  if (tab === "media" && !mediaRequested) {
    mediaRequested = true;
    void loadMediaDetails();
  }
});

onMounted(async () => {
  if (showQuotaSection.value) await loadQuota();
});

function formatDate(date?: string) {
  return date ? formatTimestamp(date, state.user.locale) : "";
}

function relativeDate(date?: string) {
  return date ? fromNow(date, state.user.locale) : "";
}

async function loadMediaDetails() {
  const { item } = props;
  if (!(isAudio.value || isVideo.value)) return;
  if (item.metadata && (!isVideo.value || item.subtitles)) return;
  try {
    const data = (await mediaApi.getItemMediaMetadata(filePath.value, { source: source.value })) as InfoItem;
    if (data && data.type !== "directory") {
      mediaDetails.value = data;
    }
  } catch (err) {
    console.warn("Failed to fetch media details:", err);
  }
}

// Raw lyrics (sidecar or embedded) are fetched on first show
async function toggleLyrics() {
  if (lyricsLoading.value) return;
  if (showLyrics.value) {
    showLyrics.value = false;
    return;
  }
  if (!lyricsText.value) {
    lyricsLoading.value = true;
    try {
      const { lyrics } = await mediaApi.getLyrics(source.value, filePath.value);
      lyricsText.value = lyrics;
    } catch (err) {
      console.warn("Failed to fetch lyrics:", err);
    } finally {
      lyricsLoading.value = false;
    }
  }
  showLyrics.value = !!lyricsText.value;
}

function subtitleLabel(track: SubtitleTrack) {
  const extras = [track.language, track.codec].filter(Boolean).join(", ");
  return track.embedded ? extras : [track.name, extras && `(${extras})`].filter(Boolean).join(" ");
}

// Raw subtitle content are fetched on first show
async function toggleSubtitle(track: SubtitleTrack) {
  if (track.name in subtitleContent.value) {
    Reflect.deleteProperty(subtitleContent.value, track.name);
    return;
  }
  try {
    const content: string = await mediaApi.getSubtitleContent(source.value, filePath.value, track.name, track.embedded);
    if (content) {
      subtitleContent.value[track.name] = content;
    }
  } catch (err) {
    console.warn("Failed to fetch subtitle:", err);
  }
}

async function loadQuota() {
  try {
    const raw = await quotasApi.get(source.value, filePath.value);
    const data = Array.isArray(raw) ? raw[0] : raw;
    if (!data) return;
    const { usedBytes, reservedBytes, measurementStatus, limitBytes } = data;
    quotaSnapshot.value = { usedBytes, reservedBytes, measurementStatus, limitBytes };
    quotaExists.value = (limitBytes || 0) > 0;
  } catch {
    // no quota or preview unavailable
  }
}

function openQuotaPrompt() {
  mutations.showPrompt({
    name: "Quota",
    props: { item: props.item, source: source.value, path: filePath.value },
  });
}

async function generateHash() {
  if (generatingHash.value) return;
  hashResult.value = "";
  generatingHash.value = true;
  try {
    hashResult.value = (await resourcesApi.checksum(source.value, filePath.value, selectedHashAlgo.value)) as string;
  } catch (err) {
    hashResult.value = t("prompts.errorGeneratingHash");
    notify.showError(err instanceof Error ? err.message : "Error generating hash");
  } finally {
    generatingHash.value = false;
  }
}

async function copyHash() {
  if (hashResult.value) await copyToClipboard(hashResult.value);
}
</script>

<style scoped>
.info-content {
  gap: 0.5em;
}

.info-tabs {
  flex-shrink: 0;
  padding: 0 0.5em 0.5em;
  border-bottom: 1px solid var(--divider);
}

.info-toggle {
  color: var(--primaryColor);
  font-size: 0.85em;
  font-weight: 600;
  text-transform: uppercase;
  cursor: pointer;
}

.info-toggle:hover {
  text-decoration: underline;
}

.info-multiline {
  display: block;
  margin-top: 0.5em;
  max-height: 16em;
  overflow: auto;
  white-space: pre-wrap;
  user-select: text;
}

.info-manage-link {
  padding: 0.25em 0.5em;
}

.info-quota-usage {
  padding: 0.5em;
}

.info-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  align-content: start;
  gap: 1.5em;
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
}

.info-grid--fill {
  grid-template-rows: minmax(0, 1fr);
}

.info-section {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
}

.info-grid--fill > .info-section {
  min-height: 0;
}

.info-item {
  display: flex;
  align-items: flex-start;
  gap: 0.75em;
  padding: 0.5em;
  border-radius: 4px;
  transition: background-color 0.2s;
}

.info-item:hover {
  background-color: var(--hoverOverlay);
}

.info-item strong {
  min-width: 120px;
  font-weight: 600;
  color: var(--textPrimary);
}

.info-item span {
  flex: 1;
  color: var(--textSecondary);
  overflow-wrap: break-word;
}

.break-word {
  overflow-wrap: break-word;
}

.hash-generator {
  display: flex;
  flex-direction: column;
  gap: 1em;
}

.hash-select,
.hash-result {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
}

.hash-select label,
.hash-result label {
  font-weight: 600;
  font-size: 0.9em;
  color: var(--textPrimary);
}

#hash-result {
  font-family: monospace;
}

@media (width <= 768px) {
  .info-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .info-item strong {
    min-width: 100px;
  }
}
</style>
