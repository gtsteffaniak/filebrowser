<template>
  <div class="media-player">
    <!-- Video with plyr -->
    <div
      v-if="previewType === 'video'"
      class="video-player-container"
      :class="{
        'no-captions': !hasSubtitles,
        'video-player-container--transcode-switching': transcodeSwitchInProgress,
      }"
    >
      <div class="media-video-container">
        <video
          ref="videoElement"
          :type="req.type"
          :src="nativeVideoSrc"
          :poster="artworkUrl"
          :autoplay="videoElementAutoplay"
          @play="handlePlay"
          @error="onVideoPlaybackError"
          @loadedmetadata="onVideoLoadedMetadata"
          playsinline
        >
          <track kind="captions" v-for="(sub, index) in subtitlesList" :key="index" :src="sub.src"
            :label="sub.name || subtitleTrackLabel(sub)" :srclang="sub.srclang || sub.language" />
        </video>
      </div>
      <div
        v-if="videoPlaybackLoading || transcodeSwitchInProgress"
        class="video-loading-overlay"
        :class="{ 'video-loading-overlay--blocking': transcodeSwitchInProgress }"
        :aria-label="transcodeSwitchInProgress ? $t('player.transcodeLoading') : undefined"
        aria-hidden="true"
        aria-busy="true"
      >
        <LoadingSpinner size="large" />
      </div>
      <div
        class="video-skip-feedback-layer"
        :class="{
          'video-skip-feedback-layer--visible': skipFeedbackVisible,
          'video-skip-feedback-layer--left': skipFeedbackSide === 'left',
          'video-skip-feedback-layer--right': skipFeedbackSide === 'right',
        }"
        aria-hidden="true"
      >
        <i :key="skipFeedbackKey" class="material-symbols video-skip-feedback-layer__icon">{{ skipFeedbackIcon }}</i>
      </div>
    </div>
    <!-- Queue button (hidden in fullscreen) -->
    <FloatingActionButton
      v-if="showQueueButton"
      icon="queue_music"
      :badge="queueCount"
      group="media-controls"
      @press="showQueuePrompt"
      :label="queueButtonLabel"
    />
    <!-- Toast when you change playback modes in the media player -->
    <div :class="['playback-toast', toastVisible ? 'visible' : '']">
      <!-- Loop icon for "single playback", "loop single file" and "loop all files" -->
      <i class="material-symbols">{{ toastIcon }}</i>
      <span>{{ playbackModeMessage }}</span>

      <!-- Status indicator for loop -->
      <span v-if="toastType === 'loop'" :class="[
          'status-indicator', loop !== 'off' ? 'status-on' : 'status-off',]"></span>
    </div>
    <!-- Speed toast (long-press) -->
    <div v-if="speedToastVisible" class="playback-toast visible">
      <i class="material-symbols">speed</i>
      <span>{{ speedToastMessage }}</span>
    </div>
  </div>
</template>

<script>
import { getCurrentInstance, shallowRef } from 'vue';
import { useDoubleTapSeek } from '@/composables/player/useDoubleTapSeek.ts';
import { useFullscreenState } from '@/composables/player/useFullscreenState.ts';
import { useGestureState } from '@/composables/player/useGestureState.ts';
import { useSwipeNavigationGestures } from '@/composables/player/useSwipeNavigationGestures.ts';
import { resolveMountedFileKey, useMediaPlayer } from '@/composables/player/useMediaPlayer.ts';
import { useMediaKeyboard } from '@/composables/player/useMediaKeyboard.ts';
import { usePlaybackQueue } from '@/composables/player/usePlaybackQueue.ts';
import { useMediaSession } from '@/composables/player/useMediaSession.ts';
import { useCaptionSize } from '@/composables/player/useCaptionSize.ts';
import {
  CAPTION_SIZE_IDS,
  getCaptionSizeLabel,
  getStoredCaptionSize,
  setStoredCaptionSize,
} from '@/composables/player/playerStorage.ts';
import { getters, mutations, state } from "@/store/index.ts";
import { globalVars } from "@/utils/constants.ts";
import { getSubtitleFormatExtension } from "@/utils/subtitles.js";
import { getPreviewURL, getPreviewURLPublic } from "@/api/resources.js";
import {
  blockSeekOnInput,
  enableSeekOnRelease,
} from "@/player/seekOnRelease.js";
import { enableScrubPreview } from "@/player/scrubPreview.js";
import { enableVideoLoadingIndicator } from "@/player/videoLoading.js";
import LoadingSpinner from '@/components/LoadingSpinner.vue';
import FloatingActionButton from '@/components/settings/FloatingActionButton.vue';
import {
  parseTranscodeModeFromQuery,
  buildPlaybackQueryPatch,
  playbackQueryChanged,
} from "@/utils/playbackQuery.js";
import {
  resolveTranscodeModeForPlayback,
  savePreferredTranscodeMode,
} from '@/utils/transcodePreference.js';
import { createTranscodeClientId } from '@/player/transcodeSession.js';
import { TranscodePlaybackController } from '@/player/transcodePlaybackController.js';
import {
  TRANSCODE_MODES as PLYR_TRANSCODE_MODES,
  TRANSCODE_HLS_MODES as PLYR_TRANSCODE_HLS_MODES,
  normalizeTranscodeMode,
  normalizeTranscodeSessionStatus,
  isTranscodeHlsMode,
  isTranscodeAvailable,
} from '@/player/transcodeMode.js';
import { transcodeLog, transcodeWarn } from '@/player/transcodeDebug.js';
import { appendMediaFragment, isPlayBlockedError, primePlay } from '@/player/playbackIntent.js';
import { needsTranscodeFirst } from '@/utils/canBrowserPlayNative.js';
import { getTranscodeSessions } from '@/api/transcode.js';
import { getCachedViewToken } from '@/api/viewToken.js';
import { notify } from "@/notify/index.ts";

export default {
  name: "plyrViewer",
  components: {
    LoadingSpinner,
    FloatingActionButton,
  },
  props: {
    previewType: {
      type: String,
      required: true,
    },
    raw: {
      type: String,
      required: true,
    },
    subtitlesList: {
      type: Array,
      default: () => [],
    },
    req: {
      type: Object,
      required: true,
    },
    autoPlayEnabled: {
      type: Boolean,
      default: false,
    },
    listing: {
      type: Array,
      default: () => [],
    },
    startTranscode: {
      type: Boolean,
      default: false,
    },
    sharedTranscodePlayback: {
      type: Object,
      default: null,
    },
  },
  emits: {
    'playback-start': () => true,
    'navigate-previous': () => true,
    'navigate-next': () => true,
    'close-preview': () => true,
    'needs-transcode': () => true,
  },
  setup(props, { emit }) {
    // The Plyr instance is held in a shallow ref: it is a third-party object and must not be deep-proxied.
    const player = shallowRef(null);
    const { applyCaptionSizeClass, syncCaptionSizeSettingsVisibility } = useCaptionSize({
      player,
      isVisible: () => props.previewType === 'video' && props.subtitlesList.length > 0,
    });
    const {
      setupMediaSession,
      updateMediaSessionNavHandlers,
      updateMediaSessionPlaybackState,
      clearMediaSession,
    } = useMediaSession(player);
    const gestures = useGestureState();
    // Bridge to the Options API instance for component methods the composables call lazily; goes away with phase 5c.
    const vm = getCurrentInstance()?.proxy;
    const doubleTap = useDoubleTapSeek({
      player,
      gestures,
      getPreviewType: () => props.previewType,
      getAudioGestureRoot: () => null,
      requestPlay: () => vm?.requestPlay(),
    });
    const swipe = useSwipeNavigationGestures({
      player,
      gestures,
      getPreviewType: () => props.previewType,
      isMobileLyricsActive: () => false,
      getGestureSurface: doubleTap.getGestureSurface,
      isControlOrMenuTarget: doubleTap.isControlOrMenuTarget,
      onNavigatePrevious: () => emit('navigate-previous'),
      onNavigateNext: () => emit('navigate-next'),
      onClosePreview: () => emit('close-preview'),
    });
    const fullscreen = useFullscreenState({ onEnter: swipe.resetEdgeGestureImmediate });
    const playbackQueue = usePlaybackQueue({
      getReq: () => props.req,
      getListing: () => props.listing,
    });
    const mediaPlayer = useMediaPlayer({
      player,
      getMediaElement: () => vm?.mediaElement,
      getRequestedFile: () => props.req,
      getMountedFile: () => vm?.mountedPreviewKey,
      isTornDown: () => vm?.plyrTeardownDone ?? false,
      destroy: () => vm?.destroyPlyr(),
      beforeStopMedia: () => vm?.clearAttachVideoStreamWait(),
    });
    const keyboard = useMediaKeyboard({
      player,
      getPreviewType: () => props.previewType,
      cyclePlaybackModes: playbackQueue.cyclePlaybackModes,
      toggleSingleLoop: playbackQueue.toggleSingleLoop,
      showQueuePrompt: playbackQueue.showQueuePrompt,
      flashSkipFeedback: doubleTap.flashSkipFeedback,
      toggleDesktopPanel: () => undefined, // the desktop panel exists only in AudioPlayer
    });
    return {
      player,
      ...keyboard,
      ...mediaPlayer,
      ...playbackQueue,
      ...gestures,
      ...doubleTap,
      ...swipe,
      ...fullscreen,
      applyCaptionSizeClass,
      syncCaptionSizeSettingsVisibility,
      setupMediaSession,
      updateMediaSessionNavHandlers,
      updateMediaSessionPlaybackState,
      clearMediaSession,
    };
  },
  data() {
    return {
      // Metadata & Art
      metadata: null, // Null by default, will be loaded from the file metadata.
      albumArtUrl: null,

      captionSizeMenuInitialized: false,
      /** Native video: defer stream URL until play so opening preview does not range-fetch the file. */
      videoStreamAttached: false,
      attachVideoStreamResume: null,
      /** Centered spinner while the video is loading or buffering. */
      videoPlaybackLoading: false,
      seekOnReleaseCleanup: null,
      scrubPreviewCleanup: null,
      videoLoadingCleanup: null,
      plyrTeardownDone: false,
      /** Playhead restore (transcode -> native); applied when the video stream attaches. */
      resumeSnapshot: null,
      /** Seek target passed via Media Fragments URI (#t=) when re-attaching the native stream. */
      resumeSeekSeconds: null,
      /** Source+path this instance mounted for; guards stale instances during route transitions. */
      mountedPreviewKey: null,
      /** Keeps :src stable after a resume attach so Vue does not rebind without #t=. */
      lockedNativeVideoSrc: null,
      transcodePlayback: null,
      transcodeClientId: createTranscodeClientId(),
      transcodeSwitchInProgress: false,
      transcodeFallbackAttempted: false,
      transcodeMode: 'native',
      transcodeMenuInitialized: false,
      transcodeModeButtons: null,
      transcodeModeValueSpan: null,
      captionSizeButtons: null,
      captionSizeValueSpan: null,
      transcodeOfferEmitted: false,
      mountPlyrInProgress: false,
      /** Armed when autoplay is blocked so the next user gesture can resume playback. */
      resumeOnGestureArmed: false,
      resumeOnGestureHandler: null,
    };
  },
  watch: {
    async playbackMode(newMode, oldMode) {
      if (newMode !== oldMode) {
        if (oldMode !== undefined) {
          // can only be "single" via clearPlaybackQueue
          this.toastType = newMode === 'single' ? 'cleared' : 'mode';
          this.showToast();
        }
        this.updateMediaSessionNavHandlers();
        await this.$nextTick();
        this.ensurePlaybackModeApplied();
      }
    },
    async loop(newVal, oldVal) {
      if (newVal !== oldVal) {
        this.toastType = 'loop';
        this.showToast();
        this.updateMediaSessionNavHandlers();
        await this.$nextTick();
        this.ensurePlaybackModeApplied();
      }
    },
    currentQueueIndex() {
      this.updateMediaSessionNavHandlers();
    },
    shouldTogglePlayPause(newVal, oldVal) {
      if (newVal !== oldVal) {
      this.togglePlayPause();
      }
    },
    isPlaying(isPlaying) {
      if (this.previewType === 'video') {
        this.syncOverlaidHintPlayback(isPlaying);
      }
    },
    'req.path'() {
      this.resetFilePlaybackState();
    },
    'req.source'(newSource, oldSource) {
      if (!newSource || newSource === oldSource) {
        return;
      }
      this.resetFilePlaybackState();
      this.destroyPlyr();
      this.plyrTeardownDone = false;
      void this.initializePlyr();
    },
    routePath() {
      this.stopIfRouteLeftThisFile();
    },
    subtitlesList: {
      async handler(newSubs, oldSubs) {
        const gained = newSubs && newSubs.length > 0 && (!oldSubs || oldSubs.length === 0);
        const lost = (!newSubs || newSubs.length === 0) && oldSubs && oldSubs.length > 0;
        if (gained || lost) {
          this.captionSizeMenuInitialized = false;
        }
        if (gained) {
          if (!this.player && this.previewType === 'video') {
            await this.$nextTick();
            void this.initializePlyr();
          } else if (this.player && this.previewType === 'video') {
            await this.$nextTick();
            this.applyCustomSettings(this.player);
            this.syncCaptionSizeSettingsVisibility();
            this.applyCaptionSizeClass();
          }
        } else if (this.player && this.previewType === 'video') {
          await this.$nextTick();
          this.syncCaptionSizeSettingsVisibility();
          this.captionSizeMenuInitialized = false;
          this.applyCustomSettings(this.player);
        }
      },
      deep: true,
    },
    '$route.query': {
      handler(newQuery, oldQuery) {
        const urlTranscode = parseTranscodeModeFromQuery(newQuery);
        const oldUrlTranscode = parseTranscodeModeFromQuery(oldQuery);
        if (urlTranscode !== oldUrlTranscode) {
          const requestedMode = urlTranscode || 'native';
          if (requestedMode !== this.transcodeMode) {
            void this.switchTranscodeMode(requestedMode);
          }
        }
      },
    },
    transcodeSwitchInProgress(val) {
      this.setTranscodeModeMenuDisabled(val);
    },
    transcodeMode() {
      this.updateTranscodeModeMenuSelection();
    },
    transcodeMenuAvailable() {
      if (this.player) {
        this.applyCustomSettings(this.player);
      }
    },
  },
  computed: {
    routePath() {
      return state.route.path;
    },
    queueButtonLabel() {
      return this.$t('player.QueueButtonHint');
    },
    showQueueButton() {
      return !this.isFullscreen;
    },
    shouldTogglePlayPause() {
      return state.playbackQueue.shouldTogglePlayPause || false;
    },
    isPlaying() {
      return state.playbackQueue.isPlaying || false;
    },
    hasSubtitles() {
      return this.subtitlesList && this.subtitlesList.length > 0;
    },
    mediaElement() {
      return this.$refs.videoElement;
    },
    shouldAutoplay() {
      return this.autoPlayEnabled || this.playbackQueue.length > 1;
    },
    fileName() {
      return this.req.name ? this.req.name.replace(/\.[^/.]+$/, "") : '';
    },
    isMobile() {
      return getters.isMobile();
    },
    scrubPreviewEnabled() {
      return (
        this.previewType === 'video'
        && Boolean(this.req?.hasPreview)
        && getters.previewPerms().video
      );
    },
    artworkUrl() {
      if (this.albumArtUrl) return this.albumArtUrl;
      if (!this.scrubPreviewEnabled) return undefined;
      return getters.isShare()
        ? getPreviewURLPublic(this.req.path, 'original')
        : `${getPreviewURL(this.req.source, this.req.path, this.req.modified)}&size=original`;
    },
    transcodeQueryMode() {
      return parseTranscodeModeFromQuery(this.$route?.query);
    },
    transcodeEnabled() {
      return isTranscodeAvailable();
    },
    transcodeActive() {
      return this.transcodePlayback?.active === true;
    },
    transcodeStartInProgress() {
      return this.transcodePlayback?.startInProgress === true;
    },
    transcodeMenuAvailable() {
      return (
        this.transcodeEnabled
        && this.previewType === 'video'
        && getters.isLoggedIn()
        && !getters.isShare()
      );
    },
    isTranscodePlaybackMode() {
      return (
        this.transcodeEnabled
        && PLYR_TRANSCODE_HLS_MODES.includes(this.transcodeMode)
      );
    },
    wantsTranscodePlayback() {
      return this.previewType === 'video' && this.isTranscodePlaybackMode;
    },
    nativeVideoTranscodeEligible() {
      return (
        this.transcodeEnabled
        && this.previewType === 'video'
        && this.transcodeMode === 'native'
        && !this.transcodeSwitchInProgress
        && !getters.isShare()
      );
    },
    formatNeedsTranscode() {
      if (this.previewType !== 'video') {
        return false;
      }
      return needsTranscodeFirst(this.transcodeProbeParams());
    },
    transcodeProfileOptions() {
      const live = state.mediaSettings?.profiles;
      const configured = Array.isArray(live) ? live : PLYR_TRANSCODE_HLS_MODES;
      return configured.filter((id) => PLYR_TRANSCODE_HLS_MODES.includes(id));
    },
    shouldProactivelyOfferTranscode() {
      if (this.previewType !== 'video' || this.startTranscode) {
        return false;
      }
      if (this.isTranscodePlaybackMode || getters.isShare()) {
        return false;
      }
      return this.formatNeedsTranscode;
    },
    plyrSettingsMenu() {
      const base = ['captions', 'captionSize', 'quality', 'speed', 'playback', 'loop'];
      // Register the slot whenever authenticated video can show it later.
      // Plyr only builds settings panels at mount; applyCustomSettings toggles
      // visibility when media/live settings settle (e.g. after syncMediaSettings).
      if (this.previewType === 'video' && getters.isLoggedIn() && !getters.isShare()) {
        return [...base, 'transcodeMode'];
      }
      return base;
    },
    nativeVideoSrc() {
      if (this.previewType !== 'video' || this.transcodeActive || this.transcodeStartInProgress) {
        return null;
      }
      if (isTranscodeHlsMode(this.transcodeMode) || this.formatNeedsTranscode) {
        return null;
      }
      if (this.lockedNativeVideoSrc) {
        return this.lockedNativeVideoSrc;
      }
      if (this.shouldAttachVideoStream) {
        const resumeTime = this.resumeSeekSeconds
          ?? this.resumeSnapshot?.currentTime;
        return appendMediaFragment(this.raw, resumeTime);
      }
      return null;
    },
    shouldAttachVideoStream() {
      if (this.formatNeedsTranscode) {
        return false;
      }
      return this.videoStreamAttached || this.shouldAutoplay;
    },
    resumeShouldAutoplay() {
      if (this.previewType !== 'video') {
        return false;
      }
      const snap = this.resumeSnapshot;
      if (!snap && !this.resumeSeekSeconds) {
        return false;
      }
      return snap?.wasPlaying !== false;
    },
    videoElementAutoplay() {
      if (this.previewType !== 'video') {
        return false;
      }
      if (isTranscodeHlsMode(this.transcodeMode) || this.transcodeStartInProgress) {
        return false;
      }
      if (this.formatNeedsTranscode) {
        return false;
      }
      if (this.resumeShouldAutoplay) {
        return true;
      }
      if (this.resumeSnapshot || this.resumeSeekSeconds) {
        return false;
      }
      return this.shouldAutoplay;
    },
    /** Rewind / fast-forward in the control bar only on non-mobile (gestures stay as elsewhere). */
    plyrOptions() {
      const controlsDesktop = [
        'play-large',
        'rewind',
        'play',
        'fast-forward',
        'progress',
        'current-time',
        'duration',
        'mute',
        'volume',
        'captions',
        'settings',
        'fullscreen',
      ];
      const controlsMobile = [
        'play-large',
        'play',
        'progress',
        'current-time',
        'duration',
        'mute',
        'volume',
        'captions',
        'settings',
        'fullscreen',
      ];
      return {
        controls: this.isMobile ? controlsMobile : controlsDesktop,
        settings: this.plyrSettingsMenu,
        i18n: {
          playback: this.$t('player.playbackMode'),
          captionSize: this.$t('player.captionSize'),
          loop: this.$t('player.loop'),
          transcodeMode: this.$t('general.transcode'),
        },
        speed: {
          selected: 1,
          options: [0.25, 0.5, 0.75, 1, 1.25, 1.5, 2],
        },
        disableContextMenu: true,
        seekTime: 10,
        hideControls: true,
        keyboard: { focused: true, global: true },
        tooltips: { controls: true, seek: !this.scrubPreviewEnabled },
        loop: { active: false },
        blankVideo: '',
        muted: false,
        autoplay: false,
        playsinline: true,
        clickToPlay: false, // we manage this ourselves with the gestures, plyr has a issue where this doesn't work in mobile.
        resetOnEnd: false,
        preload: 'none',
        fullscreen: {
          enabled: true,
          fallback: true,
          // iPhone Safari has no element Fullscreen API; only video.webkitEnterFullscreen() hides the browser UI.
          iosNative: true,
          container: '.media-player',
        },
        iconUrl: `${globalVars.baseURL}public/static/img/plyr.svg`,
        // Blob/async tracks need addtrack → captions.update; otherwise meta never fills and toggle CC throws (track undefined).
        // Do not call toggleCaptions() here — Plyr already applies `plyr` localStorage for captions on/off.
        captions: {
          active: false,
          language: 'auto',
          update: true,
        },
        listeners: {
          seek: blockSeekOnInput,
        },
      };
    },
  },
  async mounted() {
    // Record the mounted file before any async prepare step so route-leave
    // detection works while Plyr is still waiting on subtitles/metadata.
    this.mountedPreviewKey = resolveMountedFileKey(this.req?.source, this.req?.path);
    this.loadMediaMetadata();
    if (this.previewType === 'video') {
      await this.preparePlyrMount();
    }
    this.attachKeyboard();
    window.addEventListener('pagehide', this.onPageHide);
  },
  beforeUnmount() {
    // Cleanup timeouts
    this.clearDismissAnimTimers();
    // Cleanup Plyr. Always finish listener/media-session cleanup even if a
    // teardown step throws, so an orphaned element cannot keep playing.
    this.disarmResumeOnNextGesture();
    try {
      this.destroyPlyr();
    } finally {
      this.clearMediaSession();
      this.detachKeyboard();
      window.removeEventListener('pagehide', this.onPageHide);
    }
  },
  methods: {
    onPageHide(event) {
      if (event.persisted) return;
      this.teardownTranscodePlayback({ stopSession: true, keepalive: true });
      this.stopMediaElement();
    },
    /** Plyr captions menu: show format only (e.g. `.srt`, `.ass`), not the video basename. */
    subtitleTrackLabel(sub) {
      const ext = getSubtitleFormatExtension(sub?.name || '');
      return ext || sub?.name || '';
    },
    resetFilePlaybackState() {
      this.videoStreamAttached = false;
      this.resumeSeekSeconds = null;
      this.lockedNativeVideoSrc = null;
      this.hasStartedPlayback = false;
      this.transcodeOfferEmitted = false;
      this.transcodeFallbackAttempted = false;
      this.resetOverlaidHints();
    },
    destroyPlyr() {
      if (this.plyrTeardownDone) {
        return;
      }
      this.plyrTeardownDone = true;
      this.cleanupAlbumArt();

      if (this.scrubPreviewCleanup) {
        this.scrubPreviewCleanup();
        this.scrubPreviewCleanup = null;
      }
      if (this.seekOnReleaseCleanup) {
        this.seekOnReleaseCleanup();
        this.seekOnReleaseCleanup = null;
      }
      if (this.videoLoadingCleanup) {
        this.videoLoadingCleanup();
        this.videoLoadingCleanup = null;
      }
      this.videoPlaybackLoading = false;
      this.teardownOverlaidHintController();
      this.mountedPreviewKey = null;
      this.lockedNativeVideoSrc = null;
      const media = this.getActiveMediaElement();
      const isTransition = state.navigation.isTransitioning;

      // Stop the element before controller/player teardown. Those paths can
      // throw or await, and a failure there must never leave audio playing on
      // an element that is about to be detached from the DOM.
      if (media) {
        this.stopMediaElement(media);
      }

      // Any deliberate teardown of this viewer (route leave, source swap, close)
      // must stop the server session.
      this.teardownTranscodePlayback({ stopSession: true });
      if (!this.usesSharedTranscodePlayback()) {
        this.transcodePlayback = null;
      }
      this.disarmResumeOnNextGesture();

      if (this.player) {
        if (isTransition) {
          this.rememberFullscreenForTransition();
        }
        this.restoreNativePlay();
        this.teardownSwipeGestures();
        this.teardownDoubleTapSeek();
        this.clearRewindForwardFeedback();
        this.clearMediaSession();
      }
      this.destroyPlayer(media);

      this.resetPlaybackMenuState();
      this.captionSizeMenuInitialized = false;
      this.transcodeMenuInitialized = false;
      this.captionSizeButtons = null;
      this.captionSizeValueSpan = null;
      this.transcodeModeButtons = null;
      this.transcodeModeValueSpan = null;
    },
    /** Starts playback from the gesture surface or the overlaid play button. */
    requestPlay() {
      // Only offer transcoding when the source cannot play natively and no
      // transcode is already running. Otherwise a transcoded `.wmv` could never
      // be resumed from the player surface or the overlaid play button.
      if (this.formatNeedsTranscode && !this.isTranscodePlaybackMode && !this.transcodeActive) {
        void this.offerTranscodePlayback();
        return;
      }
      void this.player.play().catch((playErr) => {
        transcodeWarn('player.play failed', { message: playErr?.message });
        if (playErr?.name === 'NotSupportedError' && !this.transcodeActive) {
          void this.offerTranscodePlayback();
        }
      });
    },
    handlePlay() {
      if (this.plyrTeardownDone) {
        return;
      }
      this.$emit('playback-start');
      if (
        this.previewType === 'video'
        && this.isTranscodePlaybackMode
        && !this.transcodeActive
        && !this.transcodeStartInProgress
      ) {
        void this.startTranscodePlayback({ autoplay: true });
      }
    },
    ensurePlaybackModeApplied() {
      if (!this.player) return;
      try {
        const settingsMenu = this.player.elements.settings?.menu;
        const playbackBtn = this.player.elements.settings?.buttons?.playback;
        const captionSizeBtn = this.player.elements.settings?.buttons?.captionSize;
        const loopBtn = this.player.elements.settings?.buttons?.loop;
        const transcodeBtn = this.player.elements.settings?.buttons?.transcodeMode;
        const menuOpen =
          settingsMenu
          && settingsMenu.style.display !== 'none'
          && settingsMenu.getAttribute('hidden') === null;
        const needPlayback = playbackBtn && !this.playbackMenuInitialized;
        const needCaptionSize = captionSizeBtn && !this.captionSizeMenuInitialized;
        const needLoop = loopBtn && !this.loopMenuInitialized;
        const needTranscodeMode = transcodeBtn && !this.transcodeMenuInitialized;

        if (menuOpen || needPlayback || needCaptionSize || needLoop || needTranscodeMode) {
          this.applyCustomSettings(this.player);
        }
      } catch (error) {
        console.error('Error ensuring playback mode applied:', error);
      }
    },
    // Load metadata from the backend response
    loadMediaMetadata() {
      const meta = this.req.metadata;
      this.metadata = {
        title: meta?.title || this.fileName, // Fallback to filename
        artist: meta?.artist || null,
        album: meta?.album || null,
        year: meta?.year || null,
      };
      if (!meta?.albumArt) return;
      // Base64 encoded art
      try {
        const bytes = Uint8Array.from(atob(meta.albumArt), (c) => c.charCodeAt(0));
        this.albumArtUrl = URL.createObjectURL(new Blob([bytes], { type: 'image/jpeg' }));
      } catch (error) {
        console.error("Failed to decode album art:", error);
        this.albumArtUrl = null;
      }
    },
    cleanupAlbumArt() {
      if (typeof this.albumArtUrl === 'string' && this.albumArtUrl.startsWith('blob:')) {
        URL.revokeObjectURL(this.albumArtUrl);
      }
      this.albumArtUrl = null;
      this.metadata = null;
    },
    hookEvents() {
      void this.preparePlyrMount();
    },
    async preparePlyrMount() {
      // For videos with subtitle metadata, wait for subtitles to load before initializing Plyr
      // This prevents Plyr from trying to access tracks before they have valid blob URLs
      const hasSubtitleMetadata = this.req?.subtitles?.length > 0;
      const subtitlesNotLoaded = !this.subtitlesList || this.subtitlesList.length === 0;

      if (this.previewType === 'video' && hasSubtitleMetadata && subtitlesNotLoaded) {
        // Wait for subtitles to be loaded (watcher will call initializePlyr)
        return;
      }

      void this.initializePlyr();
    },
    async initializePlyr() {
      if (!this.mediaElement || this.player || this.plyrTeardownDone) {
        return;
      }
      await this.$nextTick();
      if (!this.mediaElement || this.player || this.plyrTeardownDone) {
        return;
      }
      void this.mountPlyrPlayer();
    },
    usesSharedTranscodePlayback() {
      return Boolean(
        this.sharedTranscodePlayback
        && this.transcodePlayback === this.sharedTranscodePlayback,
      );
    },
    bindTranscodePlaybackController() {
      if (this.sharedTranscodePlayback && !this.sharedTranscodePlayback.disposed) {
        this.transcodePlayback = this.sharedTranscodePlayback;
        return;
      }
      if (!this.transcodePlayback || this.transcodePlayback.disposed) {
        this.transcodePlayback = new TranscodePlaybackController();
      }
    },
    async mountPlyrPlayer() {
      if (!this.mediaElement || this.player || this.mountPlyrInProgress || this.plyrTeardownDone) {
        return;
      }
      this.mountPlyrInProgress = true;
      try {
        this.bindTranscodePlaybackController();
        this.mountedPreviewKey = resolveMountedFileKey(this.req?.source, this.req?.path);
        this.initTranscodeModeFromRoute();
        await this.stripPlaybackTimeFromUrl();
        if (!this.mediaElement || this.plyrTeardownDone || this.player) {
          return;
        }
        if (
          this.previewType === 'video'
          && !this.startTranscode
          && !this.wantsTranscodePlayback
          && this.shouldProactivelyOfferTranscode
        ) {
          await this.offerTranscodePlayback();
          if (!this.mediaElement || this.plyrTeardownDone || this.player) {
            return;
          }
        }
        this.createPlayer(this.plyrOptions);
        if (this.previewType === 'video' && !this.shouldAttachVideoStream) {
          this.overridePlay((nativePlay) => {
            if (!this.videoStreamAttached) {
              this.attachVideoStreamAndPlay();
              return Promise.resolve();
            }
            return nativePlay();
          });
        }
        this.setupMediaSession({
          title: this.metadata?.title || this.fileName,
          artist: this.metadata?.artist || globalVars.name || "Filebrowser Quantum",
          album: this.metadata?.album || "",
          // Embedded artwork (if any), otherwise the thumbnail/preview url.
          artworkSrc: this.artworkUrl,
        });
        this.setupPlyrEvents();
        this.seekOnReleaseCleanup = enableSeekOnRelease(this.player);
        this.setupScrubPreview();
        this.setupVideoLoadingIndicator();
        if (this.previewType === 'video' && (this.startTranscode || (this.wantsTranscodePlayback && !this.shouldProactivelyOfferTranscode))) {
          await this.startTranscodePlayback({
            autoplay: this.shouldAutoplay,
          });
        }
        this.resumeFullscreenAfterNavigation();
      } finally {
        this.mountPlyrInProgress = false;
      }
    },
    applyResumeSeek(el, snapshot, onDone) {
      if (!el || !snapshot) {
        onDone?.();
        return;
      }
      const targetTime = Number.isFinite(snapshot.currentTime) ? snapshot.currentTime : 0;
      let finished = false;
      let resumeCompleted = false;
      const completeResume = () => {
        if (resumeCompleted) {
          return;
        }
        resumeCompleted = true;
        if (snapshot.wasPlaying !== false) {
          this.attemptResumePlay(el, snapshot);
        }
        const boundSrc = el.currentSrc || el.getAttribute('src');
        if (boundSrc) {
          this.lockedNativeVideoSrc = boundSrc;
        }
        this.resumeSnapshot = null;
        this.resumeSeekSeconds = null;
        onDone?.();
      };
      const finish = () => {
        if (finished) {
          return;
        }
        finished = true;
        if (snapshot.wasPlaying !== false && el.readyState < HTMLMediaElement.HAVE_FUTURE_DATA) {
          el.addEventListener('canplay', () => completeResume(), { once: true });
          window.setTimeout(() => completeResume(), 8000);
        } else {
          completeResume();
        }
      };
      const readTime = () => this.player?.currentTime ?? el.currentTime;
      const writeTime = (time) => {
        if (this.player) {
          this.player.currentTime = time;
        } else {
          el.currentTime = time;
        }
      };
      const nearTarget = () => targetTime <= 0 || Math.abs(readTime() - targetTime) <= 1.5;
      const seek = () => {
        if (targetTime <= 0) {
          finish();
          return;
        }
        if (nearTarget()) {
          finish();
          return;
        }
        try {
          writeTime(targetTime);
        } catch (_) {
          finish();
          return;
        }
        const onSeeked = () => {
          if (nearTarget()) {
            finish();
            return;
          }
          el.addEventListener('canplay', () => {
            try {
              writeTime(targetTime);
            } catch (_) {
              finish();
              return;
            }
            el.addEventListener('seeked', () => {
              if (nearTarget()) {
                finish();
              }
            }, { once: true });
          }, { once: true });
        };
        el.addEventListener('seeked', onSeeked, { once: true });
        window.setTimeout(() => {
          if (!finished && nearTarget()) {
            finish();
          }
        }, 5000);
      };
      const whenSeekable = () => {
        if (nearTarget()) {
          finish();
          return;
        }
        if (el.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
          seek();
          return;
        }
        el.addEventListener('canplay', seek, { once: true });
      };
      if (el.readyState >= HTMLMediaElement.HAVE_METADATA) {
        whenSeekable();
      } else {
        el.addEventListener('loadedmetadata', whenSeekable, { once: true });
      }
    },
    async attachVideoStreamAndPlay() {
      if (this.plyrTeardownDone || this.videoStreamAttached || this.previewType !== 'video' || !this.raw) {
        return;
      }
      this.videoStreamAttached = true;
      this.videoLoadingCleanup?.expectPlayback?.();
      await this.$nextTick();
      await this.$nextTick();
      if (this.plyrTeardownDone || !this.mediaElement) {
        return;
      }
      const el = this.mediaElement;
      if (!el) {
        return;
      }
      const snap = this.resumeSnapshot;
      const resumeTime = snap?.currentTime ?? this.resumeSeekSeconds;
      const streamUrl = appendMediaFragment(this.raw, resumeTime);
      if (streamUrl && !el.getAttribute('src')) {
        el.setAttribute('src', streamUrl);
        this.lockedNativeVideoSrc = streamUrl;
        if (snap?.wasPlaying !== false) {
          el.autoplay = true;
        }
      }
      if (snap?.wasPlaying !== false) {
        const onCanPlay = () => this.attemptResumePlay(el, snap);
        if (el.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
          onCanPlay();
        } else {
          el.addEventListener('canplay', onCanPlay, { once: true });
        }
      }

      const startDefault = () => {
        this.clearAttachVideoStreamWait();
        if (this.plyrTeardownDone) {
          return;
        }
        void el.play().catch(() => {});
      };

      const startWithResume = () => {
        this.clearAttachVideoStreamWait();
        const onMeta = () => {
          this.applyResumeSeek(el, snap);
        };
        if (el.readyState >= HTMLMediaElement.HAVE_METADATA) {
          onMeta();
        } else {
          el.addEventListener('loadedmetadata', onMeta, { once: true });
        }
      };

      const start = snap ? startWithResume : startDefault;
      this.attachVideoStreamResume = start;
      if (snap) {
        if (el.readyState >= HTMLMediaElement.HAVE_METADATA) {
          start();
        } else {
          el.addEventListener('loadedmetadata', start, { once: true });
        }
      } else {
        start();
      }
    },
    clearAttachVideoStreamWait() {
      const el = this.mediaElement;
      if (el && this.attachVideoStreamResume) {
        el.removeEventListener('canplay', this.attachVideoStreamResume);
        el.removeEventListener('loadedmetadata', this.attachVideoStreamResume);
      }
      this.attachVideoStreamResume = null;
    },
    resolveViewTokenForTranscode() {
      return this.req?.viewToken || getCachedViewToken(this.req?.source) || '';
    },
    transcodeProbeParams() {
      const meta = this.req?.metadata || {};
      return {
        videoCodec: meta.videoCodec,
        audioCodec: meta.audioCodec,
        mimeType: this.req?.type,
        fileName: this.req?.name,
      };
    },
    ensureTranscodePlaybackController() {
      this.bindTranscodePlaybackController();
      return this.transcodePlayback;
    },
    async offerTranscodePlayback() {
      if (this.plyrTeardownDone || this.transcodeSwitchInProgress || this.isTranscodePlaybackMode) {
        return;
      }
      if (this.transcodeOfferEmitted) {
        return;
      }
      if (this.previewType !== 'video' || getters.isShare() || !this.formatNeedsTranscode) {
        return;
      }
      this.transcodeOfferEmitted = true;
      if (!this.transcodeEnabled) {
        // Transcoding is off on this server: show the unsupported notice without
        // asking for sessions, which would answer 503.
        this.$emit('needs-transcode', { enabled: false, canStart: false, sessions: [] });
        return;
      }
      try {
        const status = await getTranscodeSessions();
        this.$emit('needs-transcode', normalizeTranscodeSessionStatus(status, {
          source: this.req?.source,
          path: this.req?.path,
        }));
      } catch (err) {
        console.error('Failed to check transcode sessions:', err);
        this.$emit('needs-transcode', { enabled: false, canStart: false, sessions: [] });
      }
    },
    onVideoLoadedMetadata() {
      if (this.transcodeOfferEmitted || getters.isShare()) {
        return;
      }
      const video = this.mediaElement;
      if (!video || video.videoWidth > 0 || video.videoHeight > 0) {
        return;
      }
      if (!Number.isFinite(video.duration) || video.duration <= 0) {
        return;
      }
      void this.offerTranscodePlayback();
    },
    syncPlyrAfterTranscodeReady() {
      if (!this.transcodeActive || !this.player) {
        return;
      }
      const video = this.player.media || this.mediaElement;
      if (!video) {
        return;
      }
      const container = this.player.elements?.container;
      if (container && video.videoWidth > 0 && video.videoHeight > 0) {
        container.style.aspectRatio = `${video.videoWidth} / ${video.videoHeight}`;
      }
      this.ensurePlaybackModeApplied();
    },
    resumeTranscodePlayback() {
      const media = this.player?.media || this.mediaElement;
      if (this.plyrTeardownDone) {
        return Promise.resolve();
      }
      transcodeLog('resumeTranscodePlayback', {
        playerPaused: this.player?.paused,
        mediaPaused: media?.paused,
        readyState: media?.readyState,
      });
      const playResult = this.player?.play ? this.player.play() : media?.play();
      return Promise.resolve(playResult).then(() => {
        transcodeLog('resumeTranscodePlayback play resolved');
      }).catch((playErr) => {
        transcodeWarn('player.play failed during transcode attach', {
          name: playErr?.name,
          message: playErr?.message,
        });
        if (isPlayBlockedError(playErr)) {
          this.armResumeOnNextGesture();
        }
        throw playErr;
      });
    },
    /**
     * Autoplay was blocked (Safari requires the play() call to come directly from
     * a gesture handler). Arm a one-shot resume so a single tap or keypress
     * anywhere restarts playback instead of the user hunting for the play button.
     */
    armResumeOnNextGesture() {
      if (this.resumeOnGestureArmed) {
        return;
      }
      this.resumeOnGestureArmed = true;
      const handler = () => {
        this.disarmResumeOnNextGesture();
        const media = this.player?.media || this.mediaElement;
        if (!this.transcodeActive || !media?.paused) {
          return;
        }
        transcodeLog('resuming transcode playback after user gesture');
        void this.resumeTranscodePlayback().catch(() => {});
      };
      this.resumeOnGestureHandler = handler;
      window.addEventListener('pointerdown', handler, { once: true, capture: true });
      window.addEventListener('keydown', handler, { once: true, capture: true });
    },
    disarmResumeOnNextGesture() {
      const handler = this.resumeOnGestureHandler;
      this.resumeOnGestureArmed = false;
      this.resumeOnGestureHandler = null;
      if (!handler) {
        return;
      }
      window.removeEventListener('pointerdown', handler, { capture: true });
      window.removeEventListener('keydown', handler, { capture: true });
    },
    syncTranscodePlaybackState() {
      if (this.plyrTeardownDone) {
        return;
      }
      mutations.setPlaybackState(true);
      this.$emit('playback-start');
    },
    initTranscodeModeFromRoute() {
      if (this.previewType !== 'video' || !this.transcodeEnabled) {
        this.transcodeMode = 'native';
        return;
      }
      if (this.startTranscode) {
        this.transcodeMode = resolveTranscodeModeForPlayback(this.$route?.query);
        return;
      }
      if (needsTranscodeFirst(this.transcodeProbeParams())) {
        this.transcodeMode = 'native';
        return;
      }
      const urlTranscode = parseTranscodeModeFromQuery(this.$route?.query);
      if (urlTranscode) {
        this.transcodeMode = urlTranscode;
        return;
      }
      this.transcodeMode = 'native';
    },
    resolveActiveTranscodeMode() {
      const urlMode = parseTranscodeModeFromQuery(this.$route?.query);
      if (urlMode) {
        return urlMode;
      }
      if (PLYR_TRANSCODE_HLS_MODES.includes(this.transcodeMode)) {
        return this.transcodeMode;
      }
      return resolveTranscodeModeForPlayback(this.$route?.query);
    },
    async resolveReuseSessionId() {
      if (!this.req?.source || !this.req?.path) {
        return undefined;
      }
      try {
        const status = await getTranscodeSessions();
        const sameFile = (status.sessions || []).filter(
          (session) => session.source === this.req?.source && session.path === this.req?.path,
        );
        // Prefer a live session so a completed cache entry for another profile
        // is not passed as replaceSessionId while an older profile is still active.
        const active = sameFile.find(
          (session) => session.state === 'starting' || session.state === 'running',
        );
        return active?.id ?? sameFile[0]?.id;
      } catch {
        return undefined;
      }
    },
    async startTranscodePlayback({
      replaceSessionId,
      startSec,
      mode,
      autoplay,
      playPrime,
      manageSwitchState = true,
    } = {}) {
      if (this.plyrTeardownDone) {
        return;
      }
      transcodeLog('startTranscodePlayback called', {
        source: this.req?.source,
        path: this.req?.path,
        transcodeEnabled: this.transcodeEnabled,
        transcodeMode: this.transcodeMode,
        requestedMode: mode,
        hasPlayer: Boolean(this.player),
        replaceSessionId,
        startSec,
      });
      if (this.previewType !== 'video' || !this.req?.source || !this.req?.path || !this.transcodeEnabled) {
        transcodeWarn('startTranscodePlayback skipped — preconditions failed', {
          previewType: this.previewType,
          source: this.req?.source,
          path: this.req?.path,
          transcodeEnabled: this.transcodeEnabled,
        });
        return;
      }
      const activeMode = isTranscodeHlsMode(mode)
        ? normalizeTranscodeMode(mode)
        : this.resolveActiveTranscodeMode();
      if (!isTranscodeHlsMode(activeMode)) {
        transcodeWarn('startTranscodePlayback skipped — not an HLS mode', { activeMode });
        return;
      }
      const viewToken = this.resolveViewTokenForTranscode();
      if (!viewToken) {
        transcodeWarn('startTranscodePlayback skipped — missing view token');
        return;
      }
      const manageSwitch = manageSwitchState && !this.transcodeSwitchInProgress;
      if (manageSwitch) {
        this.transcodeSwitchInProgress = true;
      }
      const effectiveReplaceSessionId = replaceSessionId
        ?? this.transcodePlayback?.sessionController?.session?.id
        ?? await this.resolveReuseSessionId();
      const seekSec = startSec ?? this.player?.currentTime ?? 0;
      const shouldPlay = autoplay ?? this.shouldAutoplay;
      transcodeLog('startTranscodePlayback params', {
        activeMode,
        manageSwitch,
        shouldPlay,
        seekSec,
        hasPlayPrime: Boolean(playPrime),
        transcodeSwitchInProgress: this.transcodeSwitchInProgress,
      });

      try {
        transcodeLog('preparing video element for HLS', {
          replaceSessionId: effectiveReplaceSessionId,
          seekSec,
        });
        this.videoStreamAttached = true;
        this.transcodeMode = activeMode;
        this.lockedNativeVideoSrc = null;

        const startParams = {
          source: this.req.source,
          path: this.req.path,
          viewToken,
          mode: activeMode,
          clientId: this.transcodeClientId,
          replaceSessionId: effectiveReplaceSessionId,
          startSec: seekSec,
          playPrime,
          getVideoElement: () => this.player?.media || this.mediaElement,
          nextTick: () => this.$nextTick(),
          shouldAutoplay: shouldPlay,
          onFirstBuffered: () => {
            transcodeLog('first HLS fragment buffered — starting playback');
            this.syncPlyrAfterTranscodeReady();
          },
          onResumePlayback: () => this.resumeTranscodePlayback(),
          onPlaybackStarted: () => this.syncTranscodePlaybackState(),
          onFatal: () => {
            notify.showError(this.$t('player.transcodeFailed'));
          },
          onRecoverPlayback: async (ctx) => {
            transcodeLog('recovering transcode playback after HLS failure', {
              startSec: ctx?.startSec,
              sessionId: ctx?.sessionId,
            });
            await this.startTranscodePlayback({
              mode: ctx?.mode ?? this.resolveActiveTranscodeMode(),
              startSec: ctx?.startSec ?? this.player?.currentTime ?? 0,
              replaceSessionId: ctx?.sessionId,
              autoplay: ctx?.shouldAutoplay ?? this.shouldAutoplay,
              manageSwitchState: true,
              playPrime: ctx?.playPrime,
            });
            return true;
          },
          onCapacityStatus: (status) => {
            this.$emit('needs-transcode', normalizeTranscodeSessionStatus(status, {
              source: this.req?.source,
              path: this.req?.path,
            }));
          },
          onShowError: (err) => {
            notify.showError(err?.message || this.$t('player.transcodeFailed'));
          },
          onAttachNative: () => {
            if (!this.transcodeFallbackAttempted && !this.wantsTranscodePlayback) {
              this.attachVideoStreamAndPlay();
            }
          },
        };

        const result = await this.ensureTranscodePlaybackController().start(startParams);

        if (!result.ok && result.reason !== 'cancelled' && result.reason !== 'disposed' && result.reason !== 'recovered') {
          transcodeWarn('transcode playback did not start', { reason: result.reason });
          if (result.reason === 'error') {
            notify.showError(this.$t('player.transcodeFailed'));
          }
        }

        if (result.reason === 'capacity' && isTranscodeHlsMode(this.transcodeMode)) {
          // Fall back fully: leaving transcodeMode on an HLS profile would make
          // the play event fired by attachVideoStreamAndPlay re-enter
          // startTranscodePlayback and spin on the 409 limit.
          this.transcodeMode = 'native';
          this.updateTranscodeModeMenuSelection();
          this.syncPlaybackUrl();
          await this.switchToNativePlayback();
          return;
        }
        if (result.ok) {
          this.updateTranscodeModeMenuSelection();
          this.syncPlaybackUrl({ transcodeMode: activeMode });
        }
      } finally {
        if (manageSwitch) {
          this.transcodeSwitchInProgress = false;
        }
      }
    },
    teardownTranscodePlayback({ stopSession = false, keepAlive = false, keepalive = false } = {}) {
      if (!this.transcodePlayback || keepAlive) {
        return;
      }
      if (this.usesSharedTranscodePlayback()) {
        this.transcodePlayback.teardownLocal({ stopSession, keepalive });
        return;
      }
      this.transcodePlayback.dispose({ stopSession, keepalive });
    },
    onVideoPlaybackError(event) {
      if (
        this.transcodeSwitchInProgress
        || this.transcodeStartInProgress
        || this.isTranscodePlaybackMode
      ) {
        return;
      }
      if (this.transcodeOfferEmitted || getters.isShare()) {
        return;
      }
      const code = event?.target?.error?.code;
      if (
        code !== MediaError.MEDIA_ERR_DECODE
        && code !== MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED
      ) {
        return;
      }
      void this.offerTranscodePlayback();
    },
    async stripPlaybackTimeFromUrl() {
      const nextQuery = buildPlaybackQueryPatch(this.$route.query, { time: null });
      if (!playbackQueryChanged(this.$route.query, nextQuery)) {
        return;
      }
      await this.$router.replace({
        path: this.$route.path,
        query: nextQuery,
        hash: this.$route.hash,
      });
    },
    syncPlaybackUrl({ transcodeMode } = {}) {
      if (this.previewType !== 'video') {
        return;
      }
      if (this.transcodeSwitchInProgress) {
        return;
      }
      const explicitMode = transcodeMode !== undefined;
      const resolvedMode = normalizeTranscodeMode(
        explicitMode ? transcodeMode : this.transcodeMode,
      );
      let modeForQuery = null;
      if (PLYR_TRANSCODE_HLS_MODES.includes(resolvedMode)) {
        if (explicitMode || this.transcodeActive) {
          modeForQuery = resolvedMode;
        }
      }
      const nextQuery = buildPlaybackQueryPatch(this.$route.query, {
        time: null,
        transcodeMode: modeForQuery,
      });
      if (!playbackQueryChanged(this.$route.query, nextQuery)) {
        return;
      }
      this.$router.replace({
        path: this.$route.path,
        query: nextQuery,
        hash: this.$route.hash,
      });
    },
    getTranscodeModeLabel(mode) {
      switch (normalizeTranscodeMode(mode)) {
        case 'datasaver':
          return this.$t('general.dataSaver');
        case 'balanced':
          return this.$t('general.balanced');
        case 'quality':
          return this.$t('general.quality');
        default:
          return this.$t('general.off');
      }
    },
    updateTranscodeModeMenuSelection() {
      if (!this.transcodeModeButtons) {
        return;
      }
      const label = this.getTranscodeModeLabel(this.transcodeMode);
      for (const btn of this.transcodeModeButtons) {
        btn.setAttribute('aria-checked', btn.getAttribute('value') === this.transcodeMode);
      }
      if (this.transcodeModeValueSpan) {
        // Only the profile name is shown; the resolved encoder/accel is a
        // backend/ops detail and does not belong in the player's profile picker.
        this.transcodeModeValueSpan.textContent = label;
      }
    },
    setTranscodeModeMenuDisabled(disabled) {
      if (!this.transcodeModeButtons) {
        return;
      }
      for (const btn of this.transcodeModeButtons) {
        btn.disabled = disabled;
        if (disabled) {
          btn.setAttribute('aria-disabled', 'true');
        } else {
          btn.removeAttribute('aria-disabled');
        }
      }
    },    async switchToNativePlayback({ autoplay } = {}) {
      const currentTime = this.player?.currentTime ?? 0;
      const shouldPlay = autoplay ?? !!this.player?.playing;
      transcodeLog('switchToNativePlayback', { currentTime, shouldPlay });

      // Release the HLS source even when a shared controller owns it. hls.js
      // otherwise keeps its MediaSource attached to the element and the native
      // source never loads, leaving the player frozen on the last frame.
      const controller = this.transcodePlayback;
      if (controller) {
        if (this.usesSharedTranscodePlayback()) {
          controller.teardownLocal({ stopSession: true });
        } else {
          controller.dispose({ stopSession: true });
        }
      }
      this.lockedNativeVideoSrc = null;
      this.videoStreamAttached = false;
      // hls.js leaves its (now revoked) MediaSource blob URL on the element, and
      // attachVideoStreamAndPlay only binds when no src is present.
      const media = this.player?.media || this.mediaElement;
      if (media?.hasAttribute('src')) {
        media.removeAttribute('src');
      }
      await this.$nextTick();

      // Restore the playhead through the resume path, which waits for the
      // new source to report metadata before seeking. Setting currentTime any
      // earlier is discarded and playback restarts at 0:00.
      const canResume = Number.isFinite(currentTime) && currentTime > 0;
      if (canResume) {
        this.resumeSeekSeconds = currentTime;
        this.resumeSnapshot = {
          currentTime,
          wasPlaying: shouldPlay,
        };
      }

      await this.attachVideoStreamAndPlay();
      if (!canResume && shouldPlay && this.player) {
        await this.player.play().catch(() => {});
      }
    },
    async switchTranscodeMode(mode, { playPrime } = {}) {
      mode = normalizeTranscodeMode(mode);
      if (!PLYR_TRANSCODE_MODES.includes(mode)) {
        return;
      }
      if (mode === this.transcodeMode || this.transcodeSwitchInProgress) {
        return;
      }
      const previousMode = this.transcodeMode;
      const wasPlaying = !!this.player?.playing;
      transcodeLog('switchTranscodeMode start', {
        mode,
        previousMode,
        wasPlaying,
        playerPaused: this.player?.paused,
        mediaPaused: (this.player?.media || this.mediaElement)?.paused,
        shouldAutoplay: this.shouldAutoplay,
        currentTime: this.player?.currentTime,
      });
      this.transcodeSwitchInProgress = true;
      // Capture before mode/teardown changes so same-file profile switches always
      // send replaceSessionId even if the controller is mid-start.
      const replaceSessionId = this.transcodePlayback?.sessionController?.session?.id
        ?? await this.resolveReuseSessionId();
      this.transcodeMode = mode;
      if (PLYR_TRANSCODE_HLS_MODES.includes(mode)) {
        savePreferredTranscodeMode(mode);
      }
      try {
        if (PLYR_TRANSCODE_HLS_MODES.includes(mode)) {
          await this.startTranscodePlayback({
            replaceSessionId,
            startSec: this.player?.currentTime ?? 0,
            mode,
            autoplay: wasPlaying || this.shouldAutoplay,
            playPrime: wasPlaying ? playPrime : null,
            manageSwitchState: false,
          });
        } else if (needsTranscodeFirst(this.transcodeProbeParams())) {
          this.transcodeOfferEmitted = false;
          if (!this.usesSharedTranscodePlayback()) {
            this.teardownTranscodePlayback({ stopSession: true });
          }
          try {
            const status = await getTranscodeSessions();
            this.$emit('needs-transcode', normalizeTranscodeSessionStatus(status, {
              source: this.req?.source,
              path: this.req?.path,
            }));
          } catch (err) {
            console.error('Failed to check transcode sessions:', err);
          }
        } else {
          await this.switchToNativePlayback({ autoplay: wasPlaying || this.shouldAutoplay });
        }
        this.updateTranscodeModeMenuSelection();
      } catch (err) {
        console.error('Transcode mode switch failed:', err);
        this.transcodeMode = previousMode;
        this.updateTranscodeModeMenuSelection();
        if (previousMode === 'native') {
          await this.switchToNativePlayback();
        } else {
          await this.startTranscodePlayback({
            startSec: this.player?.currentTime ?? 0,
            manageSwitchState: false,
          });
        }
      } finally {
        this.transcodeSwitchInProgress = false;
        this.syncPlaybackUrl({ transcodeMode: this.transcodeMode });
        transcodeLog('switchTranscodeMode done', {
          mode: this.transcodeMode,
          transcodeActive: this.transcodeActive,
          playerPaused: this.player?.paused,
          mediaPaused: (this.player?.media || this.mediaElement)?.paused,
          videoPlaybackLoading: this.videoPlaybackLoading,
        });
      }
    },
    attemptResumePlay(el, snapshot) {
      if (this.plyrTeardownDone || !el || snapshot?.wasPlaying === false) {
        return;
      }
      const tryPlay = () => {
        if (this.plyrTeardownDone) {
          return;
        }
        const playCall = this.getNativePlay()
          ?? (() => this.player?.play?.() ?? el.play());
        void Promise.resolve(playCall()).then(() => {
          mutations.setPlaybackState(true);
          this.$emit('playback-start');
        }).catch(() => {});
      };
      if (el.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA && !el.paused) {
        return;
      }
      if (el.readyState >= HTMLMediaElement.HAVE_FUTURE_DATA) {
        tryPlay();
        return;
      }
      el.addEventListener('canplay', tryPlay, { once: true });
    },
    getPlaybackDuration() {
      const fromPlayer = this.player?.duration;
      if (Number.isFinite(fromPlayer) && fromPlayer > 0) {
        return fromPlayer;
      }
      const meta = this.req?.metadata?.duration;
      if (Number.isFinite(meta) && meta > 0) {
        return meta;
      }
      return 0;
    },
    setupVideoLoadingIndicator() {
      if (this.previewType !== 'video' || !this.player) {
        return;
      }
      this.videoLoadingCleanup?.();
      this.videoLoadingCleanup = enableVideoLoadingIndicator(
        this.player,
        (loading) => {
          this.videoPlaybackLoading = loading;
        }
      );
    },
    setupScrubPreview() {
      if (!this.scrubPreviewEnabled || !this.player) {
        return;
      }
      this.scrubPreviewCleanup?.();
      this.scrubPreviewCleanup = enableScrubPreview(this.player, {
        buildPreviewUrl: (atPercentage) => {
          const base = getters.isShare()
            ? getPreviewURLPublic(this.req.path, 'large')
            : `${getPreviewURL(this.req.source, this.req.path, this.req.modified)}&size=large`;
          return `${base}&atPercentage=${atPercentage}`;
        },
        getDuration: () => this.getPlaybackDuration(),
      });
    },
    setupPlyrEvents() {
      if (!this.player) return;
      mutations.setPlaybackState(this.player.playing);
      const eventMap = {
        ended: this.handleMediaEnd,
        play: () => {
          this.hasStartedPlayback = true;
          mutations.setPlaybackState(true);
          this.updateMediaSessionPlaybackState();
          this.transcodePlayback?.updatePlayback({
            currentTime: this.player?.currentTime,
            paused: false,
          });
        },
        pause: () => {
          mutations.setPlaybackState(false);
          this.updateMediaSessionPlaybackState();
          this.transcodePlayback?.updatePlayback({
            currentTime: this.player?.currentTime,
            paused: true,
          });
        },
        timeupdate: () => {
          this.updateMediaSessionPlaybackState();
          if (this.transcodePlayback.sessionController) {
            this.transcodePlayback.updatePlayback({
              currentTime: this.player?.currentTime,
              paused: !this.player?.playing,
            });
          }
        },
        seeked: this.updateMediaSessionPlaybackState,
        loadedmetadata: this.updateMediaSessionPlaybackState,
        ratechange: this.updateMediaSessionPlaybackState,
        canplay: this.updateMediaSessionPlaybackState,
      };
      for (const [evt, fn] of Object.entries(eventMap)) {
        this.onPlayerEvent(evt, fn);
      }
      this.onPlayerEvent('enterfullscreen', this.onFullscreenEnter);
      this.onPlayerEvent('exitfullscreen', this.onFullscreenExit);
      if (this.previewType === 'video') {
        this.onPlayerEvent('enterfullscreen', this.applyCaptionSizeClass);
        this.onPlayerEvent('exitfullscreen', this.applyCaptionSizeClass);
      }
      this.ensurePlaybackModeApplied();
      if (this.previewType === 'video') {
        this.applyCaptionSizeClass();
        this.syncCaptionSizeSettingsVisibility();
      }
      this.setupDoubleTapSeek();
      this.setupSwipeGestures();
      this.setupRewindForwardFeedback();
      if (this.previewType === 'video') {
        this.setupOverlaidHintController();
      }
    },
    // Builds playback and caption menus once, then updates state without rebuilding again.
    applyCustomSettings(player) {
      try {
        const captionSizeBtn = player.elements.settings?.buttons?.captionSize;
        const captionSizePanel = player.elements.settings?.panels?.captionSize;

        // --- Playback and loop menus (usePlaybackQueue) ---
        this.applyPlaybackMenu(player);
        this.applyLoopMenu(player);
        // --- Caption size menu ---
        if (captionSizeBtn && captionSizePanel) {
          const visible = this.previewType === 'video' && this.hasSubtitles;
          if (!visible) {
            captionSizeBtn.setAttribute('hidden', '');
            this.captionSizeMenuInitialized = true;
          } else {
          captionSizeBtn.removeAttribute('hidden');
          const title = player.config.i18n?.captionSize || 'Caption size';
          const currentSize = getStoredCaptionSize();
          const currentSizeLabel = getCaptionSizeLabel(currentSize);

          if (!this.captionSizeMenuInitialized) {
            const menu = captionSizePanel.querySelector('div[role="menu"]');
            menu.innerHTML = CAPTION_SIZE_IDS.map(
              (id) => `<button type="button" data-plyr="caption-size" role="menuitemradio" class="plyr__control" value="${id}">
                        <span>${getCaptionSizeLabel(id)}</span>
                      </button>`
            ).join('');

            this.captionSizeButtons = menu.querySelectorAll('button[data-plyr="caption-size"]');
            // Set initial checked state
            for (const btn of this.captionSizeButtons) {
              btn.setAttribute('aria-checked', btn.getAttribute('value') === currentSize);
            }
            // Add click listeners
            for (const btn of this.captionSizeButtons) {
              btn.addEventListener('click', (event) => {
                const value = event.currentTarget.getAttribute('value');
                if (!CAPTION_SIZE_IDS.includes(value)) return;
                setStoredCaptionSize(value);
                this.applyCaptionSizeClass();
                // Update checked states and label
                for (const b of this.captionSizeButtons) {
                  b.setAttribute('aria-checked', b.getAttribute('value') === value);
                }
                // Update label in button
                const label = getCaptionSizeLabel(value);
                if (this.captionSizeValueSpan) this.captionSizeValueSpan.textContent = label;
              });
            }
            const valueSpan = captionSizeBtn.querySelector('span .plyr__menu__value');
            if (valueSpan) {
              valueSpan.textContent = currentSizeLabel;
              this.captionSizeValueSpan = valueSpan;
            } else {
              captionSizeBtn.querySelector('span').innerHTML = `${title}: <span class="plyr__menu__value">${currentSizeLabel}</span>`;
              this.captionSizeValueSpan = captionSizeBtn.querySelector('span .plyr__menu__value');
            }
            this.captionSizeMenuInitialized = true;
          } else {
            // Update checked states and label
            if (this.captionSizeButtons) {
              for (const btn of this.captionSizeButtons) {
                btn.setAttribute('aria-checked', btn.getAttribute('value') === currentSize);
              }
            }
            if (this.captionSizeValueSpan) {
              this.captionSizeValueSpan.textContent = currentSizeLabel;
            }
          }
          this.applyCaptionSizeClass();
          }
        }

        // --- Transcode mode menu (video + transcode enabled) ---
        const transcodeBtn = player.elements.settings?.buttons?.transcodeMode;
        const transcodePanel = player.elements.settings?.panels?.transcodeMode;
        if (transcodeBtn && transcodePanel) {
          // Stable hook for tests — Plyr stamps data-plyr="settings" on every menu item.
          transcodeBtn.setAttribute('data-plyr-menu', 'transcodeMode');
          if (!this.transcodeMenuAvailable) {
            transcodeBtn.setAttribute('hidden', '');
            // Availability flipped off after init (logout, live settings
            // sync): the built buttons are stale, so a later availability flip
            // must rebuild them.
            this.transcodeMenuInitialized = false;
            this.transcodeModeButtons = null;
            this.transcodeModeValueSpan = null;
          } else {
            transcodeBtn.removeAttribute('hidden');
            const currentLabel = this.getTranscodeModeLabel(this.transcodeMode);

            if (!this.transcodeMenuInitialized) {
              const menu = transcodePanel.querySelector('div[role="menu"]');
              const modeButtons = [{ value: 'native', label: this.$t('general.off') }]
                .concat(this.transcodeProfileOptions.map((id) => ({
                  value: id,
                  label: this.getTranscodeModeLabel(id),
                })));
              menu.innerHTML = modeButtons.map((opt) => `
                <button data-plyr="transcode-mode" type="button" role="menuitemradio" class="plyr__control" value="${opt.value}">
                  <span>${opt.label}</span>
                </button>`).join('');
              this.transcodeModeButtons = menu.querySelectorAll('button[data-plyr="transcode-mode"]');
              for (const btn of this.transcodeModeButtons) {
                btn.setAttribute('aria-checked', btn.getAttribute('value') === this.transcodeMode);
              }
              for (const btn of this.transcodeModeButtons) {
                btn.addEventListener('click', (event) => {
                  if (this.transcodeSwitchInProgress) {
                    return;
                  }
                  const value = event.currentTarget.getAttribute('value');
                  if (!PLYR_TRANSCODE_MODES.includes(value)) {
                    return;
                  }
                  const wasPlaying = !!this.player?.playing;
                  const el = this.player?.media || this.mediaElement;
                  transcodeLog('transcode menu selection', {
                    value,
                    wasPlaying,
                    playerPaused: this.player?.paused,
                    mediaPaused: el?.paused,
                    currentTime: this.player?.currentTime,
                    shouldAutoplay: this.shouldAutoplay,
                  });
                  const playPrime = wasPlaying ? primePlay(el) : null;
                  void this.switchTranscodeMode(value, { playPrime });
                });
              }
              const valueSpan = transcodeBtn.querySelector('span .plyr__menu__value');
              if (valueSpan) {
                valueSpan.textContent = currentLabel;
                this.transcodeModeValueSpan = valueSpan;
              } else {
                transcodeBtn.querySelector('span').innerHTML = `<span class="plyr__menu__value">${currentLabel}</span>`;
                this.transcodeModeValueSpan = transcodeBtn.querySelector('span .plyr__menu__value');
              }
              this.transcodeMenuInitialized = true;
            } else if (this.transcodeModeButtons) {
              for (const btn of this.transcodeModeButtons) {
                btn.setAttribute('aria-checked', btn.getAttribute('value') === this.transcodeMode);
              }
              if (this.transcodeModeValueSpan) {
                this.transcodeModeValueSpan.textContent = currentLabel;
              }
            }
          }
        }
      } catch (error) {
        console.error('Error applying custom settings:', error);
      }
    },
  },
};
</script>

<style >

/**********************************
*** STYLES FOR THE MEDIA PLAYER ***
**********************************/

.media-video-container {
  width: 100%;
  height: 100%;
}



/* this is just a workaround, the own plyr poster was hiding the video poster and showing black
   when it didn't loaded (somehow happens); but summarizing, this a bug of plyr, it also loads the poster image 2 times
   cuz it sets the URL on the html video tags and in their own */
.plyr__poster {
  display: none !important;
}

/* Scrub preview popup (mounted on Plyr container / fullscreen root while scrubbing) */
.fb-scrub-preview {
  position: fixed;
  z-index: 2147483646;
  pointer-events: none;
  transform: translate(-50%, calc(-100% - 14px));
  opacity: 0;
  visibility: hidden;
  transition: opacity 0.12s ease;
}

.fb-scrub-preview--visible {
  opacity: 1;
  visibility: visible;
}

.fb-scrub-preview__frame {
  overflow: hidden;
  position: relative;
  max-height: 600px;
  max-width: 600px;
  border-radius: 14px;
  background: #000;
  border: 2px solid var(--primaryColor);
  box-shadow:
    0 0 0 1px rgb(0 0 0 / 35%),
    0 6px 20px rgb(0 0 0 / 55%),
    0 0 12px color-mix(in srgb, var(--primaryColor) 35%, transparent);
}

.fb-scrub-preview__frame::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 44%;
  background: linear-gradient(to top, rgb(0 0 0 / 60%), rgb(0 0 0 / 0%));
  pointer-events: none;
}

.fb-scrub-preview__frame img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.fb-scrub-preview__loading {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(0 0 0 / 45%);
}

.fb-scrub-preview__loading[hidden] {
  display: none;
}

/* Spinner sits above the previous frame while the next preview loads. */
.fb-scrub-preview__frame--loading:not(.fb-scrub-preview__frame--empty) .fb-scrub-preview__loading {
  background: rgb(0 0 0 / 50%);
}

.fb-scrub-preview__frame--loading:not(.fb-scrub-preview__frame--empty) img {
  opacity: 1;
}

.fb-scrub-preview__loading .loader {
  position: relative;
  z-index: 1;
}

.fb-scrub-preview__time {
  position: absolute;
  left: 50%;
  bottom: 8px;
  transform: translateX(-50%);
  z-index: 1;
  font-size: 13px;
  font-weight: 600;
  line-height: 1;
  color: #fff;
  white-space: nowrap;
  text-shadow:
    0 1px 3px rgb(0 0 0 / 90%),
    0 0 8px rgb(0 0 0 / 60%);
  pointer-events: none;
}

.fb-scrub-preview__arrow {
  position: absolute;
  top: calc(100% - 2px);
  transform: translateX(-50%);
  width: 14px;
  height: 8px;
  pointer-events: none;
}

.fb-scrub-preview__arrow::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  width: 0;
  height: 0;
  border-left: 7px solid transparent;
  border-right: 7px solid transparent;
  border-top: 8px solid var(--primaryColor);
}

.plyr__control--overlaid {
  background: var(--plyr-video-control-background-hover, var(--primaryColor));
  border: 0;
  display: none;
  position: absolute;
  transition:
    opacity 0.2s ease-out,
    visibility 0.2s ease-out,
    transform 0.3s ease,
    box-shadow 0.3s ease !important;
  z-index: 5;
  height: 4em;
  inset: 50% auto auto 50%;
  transform: translate(-50%, -50%) !important;
  width: 4em !important;
  margin: 0 !important;
  border-radius: 5em !important;
  pointer-events: auto;
  cursor: pointer;
  outline: none;
  box-shadow: 0 2px 10px rgb(0 0 0 / 30%);
}

.plyr--fullscreen-active .plyr__control--overlaid {
  top: 50% !important;
  left: 50% !important;
  transform: translate(-50%, -50%) !important;
}


/* Big play button when pause/start the video */
.plyr--full-ui.plyr--video .plyr__control--overlaid {
  display: flex;
  justify-content: center;
  align-items: center;
}

.plyr--video .plyr__control--overlaid:hover,
.plyr--video .plyr__control--overlaid:focus-visible,
.plyr--fullscreen-active .plyr__control--overlaid:hover,
.plyr--fullscreen-active .plyr__control--overlaid:focus-visible {
  transform: translate(-50%, -50%) scale(1.05) !important;
  opacity: 1 !important;
  outline: none;
  box-shadow:
    0 0 0 2px rgb(255 255 255 / 90%),
    0 8px 25px color-mix(in srgb, var(--primaryColor) 30%, transparent),
    0 4px 12px rgb(0 0 0 / 20%);
}

.plyr.fb-overlaid--fade-in .plyr__control--overlaid {
  transition:
    opacity 0.4s ease-in-out,
    transform 0.3s ease,
    box-shadow 0.3s ease !important;
}

.plyr.fb-overlaid--fade-out .plyr__control--overlaid {
  opacity: 0 !important;
  visibility: visible !important;
  pointer-events: none !important;
  transition:
    opacity 0.4s ease-in-out,
    transform 0.3s ease,
    box-shadow 0.3s ease !important;
}

.plyr--playing:not(.plyr--hide-controls) .plyr__control--overlaid,
.plyr--playing.fb-overlaid--shown:not(.fb-overlaid--fade-out) .plyr__control--overlaid {
  opacity: 1 !important;
  visibility: visible !important;
  pointer-events: auto !important;
  transition:
    opacity 0.4s ease-in-out,
    transform 0.3s ease,
    visibility 0.2s ease-out,
    box-shadow 0.3s ease !important;
}

/* Hide center button while playing unless shown or fading out */
.plyr--playing.plyr--hide-controls:not(.fb-overlaid--shown, .fb-overlaid--fade-out)
  .plyr__control--overlaid {
  opacity: 0 !important;
  visibility: hidden !important;
  pointer-events: none !important;
}

/************
*** VIDEO ***
************/

/* Video container size */
.video-player-container {
  position: relative;
  width: 100%;
  height: 100%;
  background-color: #000;
}

.video-loading-overlay {
  position: absolute;
  inset: 0;
  z-index: 6;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  background: rgb(0 0 0 / 35%);
}

/* Letterboxing and Plyr chrome: match cinema-style black */
.video-player-container .plyr {
  background-color: #000;
  box-shadow: none;
}

.video-player-container .plyr .plyr__controls {
  color: #fff;
}

/* Video size in the container */
.plyr.plyr--video {
  width: 100%;
  height: 100%;
}

.plyr .plyr__video-wrapper {
  touch-action: manipulation;
  cursor: pointer;
}



/* Hide captions button when there are no subtitle tracks */
.video-player-container.no-captions .plyr__control[data-plyr="captions"] {
  display: none !important;
}

/*
 * Caption size: --fb-captions-font-size on .plyr (caption-size--* from JS; inherits to .plyr__captions).
 * Video: cqmin replaces vmin when supported so small players don’t use the full viewport scale.
 */
.plyr__captions {
  pointer-events: none;
  font-size: var(--fb-captions-font-size, max(20px, 4vmin));
  line-height: 150%;
  font-weight: 700;
  -webkit-font-smoothing: antialiased;
  color: #fff;
  -webkit-text-stroke: 0.1em #000;
  paint-order: stroke fill;
  text-shadow:
    0.0625em 0.0625em 0 #000,
    -0.0625em 0.0625em 0 #000,
    -0.0625em -0.0625em 0 #000,
    0.0625em -0.0625em 0 #000,
    0 0.08em 0.2em rgb(0 0 0 / 55%);
}

.plyr.caption-size--small {
  --fb-captions-font-size: max(1em, 2.5vmin);
}

.plyr.caption-size--medium {
  --fb-captions-font-size: max(1.5em, 4vmin);
}

.plyr.caption-size--large {
  --fb-captions-font-size: max(2em, 5vmin);
}

.plyr.caption-size--xlarge {
  --fb-captions-font-size: max(2.5em, 5.5vmin);
}

.video-player-container .plyr--fullscreen-fallback .plyr__captions,
.video-player-container .plyr:fullscreen .plyr__captions {
  font-size: var(--fb-captions-font-size);
}

.plyr__caption {
  background: transparent;
  line-height: 150%;
}

</style>
