<template>
  <div class="plyr-viewer">
    <!-- Audio with plyr -->
    <div v-if="previewType == 'audio' && !useDefaultMediaPlayer" class="audio-player-container">
      <div class="audio-player-content">

        <!-- Album art with a generic icon if no image/metadata -->
        <div class="album-art-container"
             :class="{ 'no-artwork': !albumArtUrl }"
             :style="{
                 maxHeight: albumArtSize + 'em',
                 maxWidth: albumArtSize + 'em'
             }"
             @mouseenter="onAlbumArtHover"
             @mouseleave="onAlbumArtLeave"
             @wheel="onAlbumArtScroll">
          <img v-if="albumArtUrl" :src="albumArtUrl" :alt="metadata.album || 'Album art'"
            class="album-art" />
          <div v-else class="album-art-fallback">
            <i class="material-icons">music_note</i>
          </div>
        </div>

        <!-- Metadata info -->
        <div class="audio-metadata" v-if="metadata">
          <div class="audio-title">
            {{ metadata.title }}
          </div>
          <div class="audio-artist" v-if="metadata.artist">
            {{ metadata.artist }}
          </div>
          <div class="audio-album" v-if="metadata.album">
            {{ metadata.album }}
          </div>
          <div class="audio-year" v-if="metadata.album">
            {{ metadata.year }}
          </div>
        </div>
      </div>

      <div class="audio-controls-container" :class="{ 'dark-mode': darkMode, 'light-mode': !darkMode }">
        <div class="plyr-audio-container" ref="plyrAudioContainer">
          <audio :src="raw" :type="req.type" :autoplay="shouldAutoplay" @play="handlePlay" ref="audioElement"></audio>
        </div>
      </div>
    </div>

    <!-- Video with plyr -->
    <div v-else-if="previewType == 'video' && !useDefaultMediaPlayer" class="video-player-container" :class="{ 'no-captions': !hasSubtitles }">
      <div class="plyr-video-container" ref="plyrVideoContainer">
        <video :src="raw" :type="req.type" :autoplay="shouldAutoplay" @play="handlePlay" playsinline ref="videoElement">
          <track kind="captions" v-for="(sub, index) in subtitlesList" :key="index" :src="sub.src"
            :label="'Subtitle ' + sub.name" :default="false" />
        </video>
      </div>
    </div>

    <!-- Default HTML5 Audio -->
    <div v-else-if="previewType == 'audio' && useDefaultMediaPlayer" class="audio-player-container">
      <audio ref="defaultAudioPlayer" :src="raw"
        controls :autoplay="shouldAutoplay" @play="handlePlay">
      </audio>
    </div>

    <!-- Default HTML5 Video -->
    <div v-else-if="previewType == 'video' && useDefaultMediaPlayer" class="video-player-container">
      <video ref="defaultVideoPlayer" :src="raw"
        controls :autoplay="shouldAutoplay" @play="handlePlay" playsinline >
        <track kind="captions" v-for="(sub, index) in subtitlesList" :key="index" :src="sub.src"
          :label="'Subtitle ' + sub.name" :default="index === 0" />
      </video>
    </div>

    <!-- Mouse detection zone for top-left corner -->
    <div
      v-if="showQueueButton"
      class="queue-zone"
      @mousemove="toggleQueueButton"
      @mouseover="setHoverQueue(true)"
      @mouseleave="setHoverQueue(false)"
    ></div>

    <button
      v-if="showQueueButton"
      @click="showQueuePrompt"
      @mouseover="setHoverQueue(true)"
      @mouseleave="setHoverQueue(false)"
      class="queue-button floating"
      :class="{
          'dark-mode': darkMode,
          'hidden': !showQueueButtonVisible,
      }"
      :aria-label="$t('player.QueueButtonHint')"
      :title="$t('player.QueueButtonHint')"
    >
      <i class="material-icons">queue_music</i>
      <span v-if="queueCount > 0" class="queue-count">{{ queueCount }}</span>
    </button>

    <!-- Toast when you change playback modes in the media player -->
    <div :class="['playback-toast', toastVisible ? 'visible' : '']">
      <!-- Loop icon for "single playback", "loop single file" and "loop all files" -->
      <i v-if="playbackMode === 'single' || playbackMode === 'loop-single' || playbackMode === 'loop-all'" class="material-icons">
        {{ playbackMode === 'loop-single' ? 'repeat_one' : 'repeat' }} <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
      </i>
      <i v-else-if="playbackMode === 'shuffle'" class="material-icons">shuffle</i>
      <i v-else class="material-icons">playlist_play</i>

      <span>{{ playbackModeMessage }}</span>

      <!-- Status indicator for loop -->
      <span v-if="playbackMode === 'single' || playbackMode === 'loop-single'" :class="[
          'status-indicator', playbackMode === 'loop-single' ? 'status-on' : 'status-off',]"></span>
    </div>
  </div>
</template>

<script>
import { state, mutations, getters } from '@/store';
import { url } from '@/utils';
import { globalVars } from '@/utils/constants';
import Plyr from 'plyr';

export default {
  name: "plyrViewer",
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
    useDefaultMediaPlayer: {
      type: Boolean,
      default: false,
    },
    autoPlayEnabled: {
      type: Boolean,
      default: false,
    },
    listing: {
      type: Array,
      default: () => [],
    },
  },
  emits: ['play'],
  data() {
    return {
      toastVisible: false,
      toastTimeout: null,
      metadata: null, // Null by default, will be loaded from the audio file.
      albumArtUrl: null,
      albumArtSize: 25, // Default size in em
      isHovering: false, // Track hover state
      // Playback settings
      playbackMenuInitialized: false,
      lastAppliedMode: null,
      // Queue button visibility state
      queueButtonVisible: false,
      hoverQueue: false,
      queueTimeout: null,
      // Plyr instance
      player: null,
      // Plyr options
      plyrOptions: {
        controls: [
          "play-large",
          "rewind",
          "play",
          "fast-forward",
          "progress",
          "current-time",
          "duration",
          "mute",
          "volume",
          "captions",
          "pip",
          "settings",
          "fullscreen",
        ],
        settings: ["quality", "speed", "playback"],
        speed: {
          selected: 1,
          options: [0.25, 0.5, 0.75, 1, 1.25, 1.5, 2],
        },
        disableContextMenu: true,
        seekTime: 10,
        hideControls: true,
        keyboard: { focused: true, global: true },
        tooltips: { controls: true, seek: true },
        loop: { active: false },
        blankVideo: "",
        muted: false, // Disable muting automatically
        autoplay: false, // The users will manage this from their profile settings
        playsinline: true,
        clickToPlay: true,
        resetOnEnd: true,
        preload: 'metadata',
        iconUrl: '/static/img/plyr.svg',
      },
    };
  },
  watch: {
    playbackMode: {
      handler(newMode, oldMode) {
        if (newMode !== oldMode) {
          const forceReshuffle = newMode === 'shuffle';
          this.setupPlaybackQueue(forceReshuffle);
          this.$nextTick(() => {
            this.ensurePlaybackModeApplied();
          });
        }
      },
    },
    shouldTogglePlayPause(newVal, oldVal) {
      if (newVal !== oldVal) {
      this.togglePlayPause();
      }
    },
    listing(newListing) {
      // update queue if the listing changes
      if (newListing && newListing.length > 0) {
        this.setupPlaybackQueue(true);
      }
    },
  },
  computed: {
    darkMode() {
      return state.user.darkMode;
    },
    showQueueButtonVisible() {
      return this.queueButtonVisible || this.hoverQueue;
    },
    showQueueButton() {
      return state.req && (state.req.type?.startsWith('audio/') || state.req.type?.startsWith('video/')) &&
      state.navigation.enabled;
    },
    queueCount() {
      return state.playbackQueue?.queue?.length || 0;
    },
    shouldTogglePlayPause() {
      return state.playbackQueue?.shouldTogglePlayPause || false;
    },
    playbackQueue() {
      return state.playbackQueue?.queue || [];
    },
    currentQueueIndex() {
      return state.playbackQueue?.currentIndex ?? -1;
    },
    playbackMode() {
      return state.playbackQueue?.mode || 'single';
    },
    playbackModeMessage() {
      const mode = {
      'sequential': this.$t('player.PlayAllOncePlayback'),
      'shuffle': this.$t('player.ShuffleAllPlayback'),
      'loop-all': this.$t('player.PlayAllLoopedPlayback'),
      'loop-single': this.$t('player.LoopEnabled'),
      'single': this.$t('player.LoopDisabled')
      };
      return mode[this.playbackMode] || mode.single;
    },
    isPlaying() {
      return state.playbackQueue?.isPlaying || false;
    },
    hasSubtitles() {
      return this.subtitlesList && this.subtitlesList.length > 0;
    },
    mediaElement() {
      if (this.useDefaultMediaPlayer) {
        return this.previewType === 'video'
          ? this.$refs.defaultVideoPlayer 
          : this.$refs.defaultAudioPlayer;
      }
      return this.previewType === 'video'
        ? this.$refs.videoElement 
        : this.$refs.audioElement;
    },
    shouldAutoplay() {
      return this.autoPlayEnabled || this.playbackQueue.length > 1;
    },
    fileName() {
      return this.req.name ? this.req.name.replace(/\.[^/.]+$/, "") : '';
    },
  },
  mounted() {
    this.updateMedia();
    this.$nextTick(() => {
      // Show queue button initially if it should be shown
      if (this.showQueueButton) {
        this.showQueueButtonMethod();
      }
    });
    document.addEventListener('keydown', this.handleKeydown);
  },
  beforeUnmount() {
    // Cleanup timeouts
    [this.toastTimeout, this.queueTimeout].forEach(timeout => {
      if (timeout) clearTimeout(timeout);
    });
    // Cleanup Plyr
    this.destroyPlyr();
    this.mediaElement.pause();
    this.clearMediaSession();
    document.removeEventListener('keydown', this.handleKeydown);
  },
  methods: {
    showQueuePrompt() {
      mutations.showHover({
        name: "PlaybackQueue",
      });
    },
    toggleQueueButton() {
      if (!this.showQueueButton) {
        return;
      }
      this.showQueueButtonMethod();
    },
    showQueueButtonMethod() {
      this.queueButtonVisible = true;
      this.clearQueueTimeout();
      this.queueTimeout = setTimeout(() => {
        if (!this.hoverQueue) {
          this.queueButtonVisible = false;
        }
        this.queueTimeout = null;
      }, 3000); // Show for 3 seconds
    },
    setHoverQueue(value) {
      this.hoverQueue = value;
    },
    clearQueueTimeout() {
      if (this.queueTimeout) {
        clearTimeout(this.queueTimeout);
        this.queueTimeout = null;
      }
    },
    setupMediaSession() {
      if (!('mediaSession' in navigator) || !this.player) return;
      // Create a fresh fallback URL with timestamp to prevent caching issues
      const fallbackIcon = globalVars.loginIcon;
      const timestamp = Date.now();
      const fallbackUrl = fallbackIcon.includes('?') 
        ? `${fallbackIcon}&t=${timestamp}`
        : `${fallbackIcon}?t=${timestamp}`;
      const metadata = {
        title: this.metadata?.title || this.fileName,
        artist: this.metadata?.artist || globalVars.name || "Filebrowser Quantum",
        album: this.metadata?.album || "",
        // In current versions of Firefox the artwork will not work, seems that doesn't like blob URLs.
        // But testing in 149.0a1 (nightly builds), it seems to work, so this something that will solve over time :)
        artwork: [ { src: this.albumArtUrl || fallbackUrl } ]
      };
      navigator.mediaSession.metadata = new MediaMetadata(metadata);
      // Setup handlers for the media session
      const actionHandlers = [
        ['play', () => this.player?.play()],
        ['pause', () => this.player?.pause()],
        ['previoustrack', () => {
          if (this.playbackQueue.length > 1) {
            this.playPrevious();
          }
        }],
        ['nexttrack', () => {
          if (this.playbackQueue.length > 1) {
            this.playNext();
          }
        }],
        ['seekbackward', (details) => this.player?.rewind(details.seekOffset || 10)],
        ['seekforward', (details) => this.player?.forward(details.seekOffset || 10)],
        ['seekto', (details) => {
          if (details.fastSeek && details.fastSeek === 'optional') return;
          this.player.currentTime = details.seekTime;
        }]
      ];
      for (const [action, handler] of actionHandlers) {
        try {
          navigator.mediaSession.setActionHandler(action, handler);
        } catch (error) {
          console.warn(`The media session action "${action}" is not supported`);
        }
      }
      this.updateMediaSessionPlaybackState();
    },
    updateMediaSessionPlaybackState() {
      if (!('mediaSession' in navigator)) return;
      if (this.player) {
        navigator.mediaSession.playbackState = this.player.playing ? 'playing' : 'paused';
        // Update position state
        if (navigator.mediaSession.setPositionState) {
          navigator.mediaSession.setPositionState({
            duration: this.player.duration,
            playbackRate: this.player.speed || 1,
            position: this.player.currentTime,
          });
        }
      }
    },
    clearMediaSession() {
      if (!('mediaSession' in navigator)) return;
      // Clear metadata
      navigator.mediaSession.metadata = null;
      // Clear all action handlers
      const actions = [ 'play', 'pause', 'previoustrack', 'nexttrack', 'seekbackward', 'seekforward', 'seekto', 'stop' ];
      actions.forEach(action => {
        navigator.mediaSession.setActionHandler(action, null);
      });
      // Clear position state
      if (navigator.mediaSession.setPositionState) {
        navigator.mediaSession.setPositionState(null);
      }
      // Reset playback state
      navigator.mediaSession.playbackState = 'none';
    },
    destroyPlyr() {
      if (this.player) {
        this.clearMediaSession();
        this.cleanupAlbumArt();
        this.player.off();
        this.player.destroy();
        this.player = null;
        this.playbackMenuInitialized = false;
        this.lastAppliedMode = null;
        // This should fix (most of) the "Invalid URI" warns, meanwhile we still destroying plyr.
        // Somehow firefox will still trying to "load" the empty source which causes the warn.
        this.mediaElement.src = this.raw;
      }
    },
    togglePlayPause() {
      if (!this.mediaElement) return;
      if (this.useDefaultMediaPlayer) {
        if (this.mediaElement.paused) {
          this.mediaElement.play();
        } else {
          this.mediaElement.pause();
        }
      } else if (this.player) {
        if (this.player.playing) {
          this.player.pause();
        } else {
          this.player.play();
        }
      }
    },
    handlePlay() {
      this.$emit('play');
    },
    ensurePlaybackModeApplied() {
      if (this.useDefaultMediaPlayer || !this.player) return;
      try {
        const settingsMenu = this.player.elements.settings?.menu;
        const playbackBtn = this.player.elements.settings?.buttons?.playback;

        if (settingsMenu && settingsMenu.style.display !== 'none' && settingsMenu.getAttribute('hidden') === null) {
          this.applyCustomPlaybackSettings(this.player);
        } else if (playbackBtn && !this.playbackMenuInitialized) {
          // Initial setup -- if menu hasn't been initialized yet
          console.log('Initializing custom playback menu');
          this.applyCustomPlaybackSettings(this.player);
        }
        // Otherwise, skip to avoid unnecessary recreation
      } catch (error) {
        console.error('Error ensuring playback mode applied:', error);
      }
    },
    toggleLoop() {
      const newMode = this.playbackMode === 'loop-single' ? 'single' : 'loop-single';
      // Update the state directly via mutations
      mutations.setPlaybackQueue({
        queue: this.playbackQueue,
        currentIndex: this.currentQueueIndex,
        mode: newMode
      });
      this.showToast();
    },
    handleKeydown(event) {
      // Handle 'P' and 'L' keys for loop and change playback
      const key = event.key.toLowerCase();

      if (key === 'p' || key === 'l') {
        event.stopPropagation();
        event.preventDefault();

        if (key === 'p') this.cyclePlaybackModes();
        if (key === 'l') this.toggleLoop();
      }
      // "Q" key for open the queue prompt
      if (key === 'q' && state.prompts.length === 0) { // Only open if no other prompts are open
        event.stopPropagation();
        event.preventDefault();
        this.showQueuePrompt();
      }
    },
    cyclePlaybackModes() {
      // cycle order (excluding single and loop-single cuz they are handled by the "L" key)
      const modeCycle = ['loop-all', 'shuffle', 'sequential'];
      const currentIndex = modeCycle.indexOf(this.playbackMode);
      const nextIndex = (currentIndex + 1) % modeCycle.length;
      const newMode = modeCycle[nextIndex];
      // Directly update state
      mutations.setPlaybackQueue({
        queue: this.playbackQueue,
        currentIndex: this.currentQueueIndex,
        mode: newMode
      });
      this.showToast();
    },
    showToast() {
      if (this.toastTimeout) {
        clearTimeout(this.toastTimeout);
      }
      this.toastVisible = true;
      this.toastTimeout = setTimeout(() => {
        this.toastVisible = false;
      }, 1500);
    },
    // Album art hover and scroll handlers
    onAlbumArtHover() {
      this.isHovering = true;
    },
    onAlbumArtLeave() {
      this.isHovering = false;
    },
    onAlbumArtScroll(event) {
      if (!this.isHovering) return;
      event.preventDefault();
      const scrollDelta = event.deltaY > 0 ? -5 : 5; // Scroll down decreases, scroll up increases
      const newSize = this.albumArtSize + scrollDelta;
      // Apply size constraints (minimum 10em, maximum 50em)
      this.albumArtSize = Math.max(10, Math.min(50, newSize));
    },
    // Load metadata from the backend response
    loadAudioMetadata() {
      if (this.previewType !== "audio") return;
      // Check if metadata is already provided by the backend
      if (this.req.metadata) {
        this.metadata = {
          title: this.req.metadata.title || this.fileName, // Fallback to filename
          artist: this.req.metadata.artist || null,
          album: this.req.metadata.album || null,
          year: this.req.metadata.year || null
        };
        // Handle base64 encoded album art
        if (this.req.metadata.albumArt) {
          try {
            const byteCharacters = atob(this.req.metadata.albumArt);
            const byteArray = new Uint8Array(byteCharacters.length);
            for (let i = 0; i < byteCharacters.length; i++) {
              byteArray[i] = byteCharacters.charCodeAt(i);
            }
            const blob = new Blob([byteArray], { type: 'image/jpeg' });
            this.albumArtUrl = URL.createObjectURL(blob);
          } catch (error) {
            console.error("Failed to decode album art:", error);
            this.albumArtUrl = null;
          }
        }
      } else {
        this.metadata = {
          title: this.fileName,
          artist: null,
          album: null,
          year: null,
        };
      }
    },
    cleanupAlbumArt() {
      if (this.albumArtUrl && this.albumArtUrl.startsWith('blob:')) {
        URL.revokeObjectURL(this.albumArtUrl);
      }
      this.albumArtUrl = null;
      this.metadata = null;
    },
    updateMedia() {
      this.hookEvents();
      if (this.previewType === "audio") {
        this.loadAudioMetadata();
      }
    },
    hookEvents() {
      if (this.useDefaultMediaPlayer) {
        this.setupDefaultPlayerEvents(this.mediaElement);
        return;
      }
      this.initializePlyr();
    },
    initializePlyr() {
      if (!this.mediaElement) return;
      // Small delay to ensure DOM is ready
      this.$nextTick(() => {
        // Initialize Plyr
        this.player = new Plyr(this.mediaElement, this.plyrOptions);
        // Set up Media Session API
        this.setupMediaSession();
        // Set up event listeners
        this.setupPlyrEvents();
      });
    },
    setupPlyrEvents() {
      if (!this.player) return;
      this.player.on('ended', this.handleMediaEnd);
      this.player.on('play', () => {
        mutations.setPlaybackState(true);
        this.updateMediaSessionPlaybackState();
      });
      this.player.on('pause', () => {
        mutations.setPlaybackState(false);
        this.updateMediaSessionPlaybackState();
      });
      this.player.on('timeupdate', () => {
        this.updateMediaSessionPlaybackState();
      });
      this.player.on('seeked', () => {
        this.updateMediaSessionPlaybackState();
      });
      this.player.on('loadedmetadata', () => {
        this.updateMediaSessionPlaybackState();
      });
      this.player.on('ratechange', () => {
        this.updateMediaSessionPlaybackState();
      });
      this.player.on('canplay', () => {
        this.updateMediaSessionPlaybackState();
      });
      if (this.previewType === 'video') {
        if (screen.orientation) {
          this.player.on('enterfullscreen', this.onFullscreenEnter);
          this.player.on('exitfullscreen', this.onFullscreenExit);
        }
      }
      this.ensurePlaybackModeApplied();
    },
    setupDefaultPlayerEvents(element) {
      if (!element) return;
      element.addEventListener('ended', this.handleMediaEnd);
      element.addEventListener('play', () => {
        mutations.setPlaybackState(true);
      });
      element.addEventListener('pause', () => {
        mutations.setPlaybackState(false);
      });
      element.addEventListener('timeupdate', () => {
        this.updateMediaSessionPlaybackState();
      });
      element.addEventListener('loadedmetadata', () => {
        this.updateMediaSessionPlaybackState();
      });
    },
    async onFullscreenEnter() {
      // Allow free rotation when video enters full screen mode. This works even if the device's orientation is currently locked.
      try {
        await screen.orientation.lock('any');
      } catch (error) {
        // The NotSupportedError is thrown for non-mobile browsers and there seems to be no way to pre-check if it is supported.
        // -> Swallow NotSupportedError but let other errors be thrown.
        if (error.name !== 'NotSupportedError')
          throw error;
      }
    },
    onFullscreenExit() {
      screen.orientation.unlock();
    },
    // Playback methods
    async setupPlaybackQueue(forceReshuffle = false) {

      let listing = this.listing;
      const isShare = getters.isShare();

      // Filter only audio/video files
      const mediaFiles = listing.filter(item => {
        const isAudio = item.type && item.type.startsWith('audio/');
        const isVideo = item.type && item.type.startsWith('video/');
        return isAudio || isVideo;
      });

      if (mediaFiles.length === 0) {
        console.log('No media files found in current directory');
        mutations.setPlaybackQueue({
          queue: [],
          currentIndex: -1,
          mode: this.playbackMode,
        });
        return;
      }

      let currentIndex = -1;
      if (isShare) {
        // Compare by name for shares
        currentIndex = mediaFiles.findIndex(item => item.name === this.req.name);
      } else {
        currentIndex = mediaFiles.findIndex(item => item.path === this.req.path);
      }

      let finalQueue = [];
      let finalIndex = 0;

      switch (this.playbackMode) {
        case 'single':
        case 'loop-single':
          // When playing the same file (single modes), the queue only contains only the current file
          finalQueue = currentIndex !== -1 ? [mediaFiles[currentIndex]] : [];
          finalIndex = 0;
          break;

        case 'sequential':
        case 'loop-all': {
          // We'll use the listing order from the parent directory for this two modes.
          // On sequential mode will start playing from the file opened and find its place on the queue by the current index (you can see this on UI queue)
          // Loop-all will do the same, but if the queue ends, will restart from the first file of the current folder.
          const sortedFiles = [...mediaFiles];
          finalQueue = sortedFiles;
          // Find the current file position in the queue
          if (currentIndex !== -1) {
            const currentFile = mediaFiles[currentIndex];
            finalIndex = sortedFiles.findIndex(item => item.path === currentFile.path);
          } else {
            finalIndex = 0;
          }
          break;
        }
        case 'shuffle': {
          // For shuffle, all on random order and only reshuffle if we cycle modes again
          // This is for preserve the current queue and don't lose it (since the component is re-created each time)
          // It has one drawback: if you change of directories, you'll need to cycle modes again to see a new queue.
          if (forceReshuffle || this.playbackQueue.length === 0) {
            const shuffledFiles = this.shuffleArray([...mediaFiles]);
            finalQueue = shuffledFiles;
            } else {
              // Use the existing queue when not forcing reshuffle
              finalQueue = this.playbackQueue;
            }
          // Find the current file position in the queue
          if (currentIndex !== -1) {
            const currentFile = mediaFiles[currentIndex];
            finalIndex = finalQueue.findIndex(item => item.path === currentFile.path);
          } else {
            finalIndex = 0;
          }
          break;
        }
      }
      // console.log('Current place on the queue:', finalIndex + 1, 'of', finalQueue.length);

      // After the queue is set up, update the store
      mutations.setPlaybackQueue({
        queue: finalQueue,
        currentIndex: finalIndex,
        mode: this.playbackMode
      });
    },
    shuffleArray(array) {
      const shuffled = [...array];
      for (let i = shuffled.length - 1; i > 0; i--) {
        const j = Math.floor(Math.random() * (i + 1));
        [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
      }
      return shuffled;
    },
    updateCurrentQueueIndex() {
      if (this.playbackQueue.length === 0) {
        this.setupPlaybackQueue();
        return;
      }
      // Find current file in the existing queue
      const currentIndex = this.playbackQueue.findIndex(item => item.path === this.req.path);
      if (currentIndex !== -1) {
        mutations.setPlaybackQueue({
          queue: this.playbackQueue,
          currentIndex: currentIndex,
          mode: this.playbackMode
        });
      } else {
        this.setupPlaybackQueue(true);
      }
    },
    playPrevious() {
      if (this.playbackQueue.length === 0) return;
      // Calculate previous index
      let prevIndex = this.currentQueueIndex - 1;
      // Handle wrapping based on mode
      if (prevIndex < 0) {
        if (this.playbackMode === 'loop-all' || this.playbackMode === 'shuffle') {
          prevIndex = this.playbackQueue.length - 1;
        } else {
          return;
        }
      }
      const prevItem = this.playbackQueue[prevIndex];
      // Update current index
      mutations.setPlaybackQueue({
        queue: this.playbackQueue,
        currentIndex: prevIndex,
        mode: this.playbackMode
      });
      mutations.setNavigationTransitioning(true);
      url.goToItem(prevItem.source || this.req.source, prevItem.path, undefined);
    },
    playNext() {
      if (this.playbackQueue.length === 0) return;

      // Calculate next index
      let nextIndex = this.currentQueueIndex + 1;

      // Handle end of queue based on mode
      if (nextIndex >= this.playbackQueue.length) {
        if (this.playbackMode === 'loop-all' || this.playbackMode === 'shuffle') {
          nextIndex = 0; // Loop back to beginning
        } else {
          return; // Stop at end for sequential mode
        }
      }

      const nextItem = this.playbackQueue[nextIndex];

      try {
        // Update current index
        mutations.setPlaybackQueue({
          queue: this.playbackQueue,
          currentIndex: nextIndex,
          mode: this.playbackMode
        });

        mutations.setNavigationTransitioning(true);
        url.goToItem( nextItem.source || this.req.source, nextItem.path, undefined );

      } catch (error) {
        console.error('Failed to navigate to next file:', error);
      }
    },
    restartCurrentFile() {
      console.log('Restarting current file');
      if (this.useDefaultMediaPlayer) {
        // HTML5 player
        this.mediaElement.currentTime = 0;
        this.mediaElement.play();
      } else if (this.player) {
        // Plyr player
        this.player.currentTime = 0;
        this.player.play();
      }
    },
    handleMediaEnd() {
      const handleShortQueue = () => {
        if (this.playbackQueue.length > 1) {
          this.playNext();
        } else {
          this.restartCurrentFile();
        }
      };
      const modeActions = {
        'single': () => {}, // Do nothing
        'loop-single': () => this.restartCurrentFile(),
        'sequential': () => this.playNext(),
        'shuffle': handleShortQueue,
        'loop-all': handleShortQueue,
      };
      const action = modeActions[this.playbackMode];
      if (action) {
        console.log(`Media ended - ${this.playbackMode} mode`);
        action();
      }
    },
    applyCustomPlaybackSettings(player) {
      // This is the actual logic to set up the settings menu
      // Separated so it can be called after source changes

      // Only recreate menu if mode changed or menu not initialized, this for avoid unnecesary recreations
      const modeChanged = this.lastAppliedMode !== this.playbackMode;

      if (this.playbackMenuInitialized && !modeChanged) {
        return;
      }

      try {
        // Access the playback button and panel
        const playbackBtn = player.elements.settings.buttons.playback;
        const playbackPanel = player.elements.settings.panels.playback;

        if (playbackBtn && playbackPanel) {
          // Make the button visible
          playbackBtn.removeAttribute('hidden');

          // Set up the button text
          const modeLabels = {
            'single': 'Play Once',
            'sequential': 'Play All',
            'shuffle': 'Shuffle All',
            'loop-single': 'Loop current',
            'loop-all': 'Play All Looped'
          };
          const currentMode = modeLabels[this.playbackMode] || 'Play Once';
          playbackBtn.querySelector('span').innerHTML = `Playback: <span class="plyr__menu__value">${currentMode}</span>`;

          // Set up the back button text
          playbackPanel.querySelector('.plyr__control--back span[aria-hidden="true"]').innerHTML = 'Playback';

          // Only recreate menu if needed, will rebuild the UI if the source changes.
          const menu = playbackPanel.querySelector('div[role="menu"]');

          if (!this.playbackMenuInitialized || modeChanged) {

            // Create the menu options
            menu.innerHTML = `
              <button data-plyr="playback" type="button" role="menuitemradio" class="plyr__control" aria-checked="${this.playbackMode === 'single'}" value="single">
                <span>Play Once</span>
              </button>
              <button data-plyr="playback" type="button" role="menuitemradio" class="plyr__control" aria-checked="${this.playbackMode === 'sequential'}" value="sequential">
                <span>Play All</span>
              </button>
              <button data-plyr="playback" type="button" role="menuitemradio" class="plyr__control" aria-checked="${this.playbackMode === 'shuffle'}" value="shuffle">
                <span>Shuffle All</span>
              </button>
              <button data-plyr="playback" type="button" role="menuitemradio" class="plyr__control" aria-checked="${this.playbackMode === 'loop-single'}" value="loop-single">
                <span>Loop Current</span>
              </button>
              <button data-plyr="playback" type="button" role="menuitemradio" class="plyr__control" aria-checked="${this.playbackMode === 'loop-all'}" value="loop-all">
                <span>Play All Looped</span>
              </button>
            `;
            // Add event listeners to the buttons
            const buttons = menu.querySelectorAll('button[data-plyr="playback"]');
            buttons.forEach(button => {
              button.addEventListener('click', (event) => {
                const value = event.currentTarget.getAttribute('value');
                console.log('Playback mode changed to:', value);

                // Update visual state
                buttons.forEach(btn => btn.setAttribute('aria-checked', 'false'));
                event.currentTarget.setAttribute('aria-checked', 'true');

                // Update button text
                const currentMode = modeLabels[value] || 'Play Once';
                playbackBtn.querySelector('span').innerHTML = `Playback: <span class="plyr__menu__value">${currentMode}</span>`;

                // Update the global state with the new mode
                mutations.setPlaybackQueue({
                  queue: this.playbackQueue,
                  currentIndex: this.currentQueueIndex,
                  mode: value
                });
                // Show toast
                this.showToast();
              });
            });
            this.playbackMenuInitialized = true;
            this.lastAppliedMode = this.playbackMode;
          } else {
            // Just update the checked states without recreating the menu again
            const buttons = menu.querySelectorAll('button[data-plyr="playback"]');
            buttons.forEach(button => {
              const value = button.getAttribute('value');
              button.setAttribute('aria-checked', this.playbackMode === value);
            });
          }
        } else {
          console.error('Could not find playback button or panel');
        }
      } catch (error) {
        console.error('Error applying custom playback settings:', error);
      }
    },
  },
};
</script>

<style >
@import url("plyr/dist/plyr.css");

/* Background styles for the audio player */
.plyr-background-dark {
  background: radial-gradient(#3b3b3b, black);
}

.plyr-background-light {
  background: radial-gradient(#262626, #e2e2e2);
}

/**********************************
*** STYLES FOR THE MEDIA PLAYER ***
**********************************/

.plyr-video-container {
  width: 100%;
  height: 100%;
}

.plyr-viewer {
  height: 100%;
  width: 100%;
}

.plyr {
  --plyr-color-main: var(--primaryColor);
  --plyr-video-background: rgba(0, 0, 0, 1);
  --plyr-focus-visible-color: var(--primaryColor);
  --plyr-audio-control-color: #ffffff;
  --plyr-menu-background: rgba(0, 0, 0, 0.7);
  --plyr-menu-color: #ffffff;
  --plyr-menu-border-shadow-color: rgba(0, 0, 0, 0.5);
  --plyr-menu-radius: 12px;
  --plyr-menu-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
  --plyr-control-radius: 12px;
  --plyr-control-icon-size: 16px;
  --plyr-control-spacing: 8px;
  --plyr-control-padding: 6px;
  --plyr-tooltip-background: rgba(0, 0, 0, 0.8);
  --plyr-tooltip-color: #ffffff;
  --plyr-video-controls-background: linear-gradient(transparent,
          rgba(0, 0, 0, 0.7));
  overflow: visible;
  background-color: rgb(216 216 216);
  box-shadow: 0 2px 6px rgba(88, 88, 88, 0.45);
}

.plyr__controls {
  color: black;
}

.audio-controls-container.dark-mode .plyr {
  background-color: rgb(37 49 55 / 33%);
  color: white;
}

/* Backdrop-filter support for plyr */
@supports (backdrop-filter: none) {
  .plyr {
    backdrop-filter: blur(16px) invert(0.1);
  }
}

/* Position/space of the buttons */
.plyr .plyr__controls {
  display: flex;
  flex-direction: row;
  gap: 8px;
  background-color: transparent;
}

.audio-controls-container.dark-mode .plyr .plyr__controls {
  color: white;
}

.audio-controls-container.light-mode .plyr .plyr__controls {
  color: black;
}

.plyr .plyr__controls__items {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: nowrap;
}

/* Transitions (e.g. how much time take to hide the player UI) */
.plyr .plyr__control {
  transition: all 0.2s ease;
  flex-shrink: 0;
  display: flex;
  min-width: 2em;
  justify-content: center;
  align-items: center;
}

/* Progress bar with full width (audio and video) */
.plyr .plyr__progress__container {
  flex: 100%;
  margin: 0;
}

/* Big play button when pause/start the video */
.plyr--full-ui.plyr--video .plyr__control--overlaid {
  display: flex;
  justify-content: center;
  align-items: center;
}

.plyr__control--overlaid {
  background: var(--plyr-video-control-background-hover, var(--primaryColor));
  border: 0;
  display: none;
  position: absolute;
  transition: 0.3s;
  z-index: 2;
  height: 4em;
  top: 50%;
  left: 50%;
  right: auto;
  transform: translate(-50%, -50%) !important;
  bottom: auto;
  width: 4em !important;
  margin: 0 !important;
  border-radius: 5em !important;
  transition: transform 0.2s ease !important;
}

.plyr--fullscreen-active .plyr__control--overlaid {
  top: 50% !important;
  left: 50% !important;
  transform: translate(-50%, -50%) !important;
}

.plyr__control--overlaid:hover {
  transform: translate(-50%, -50%) scale(1.05) !important;
}

/************
*** VIDEO ***
************/

/* Video container size */
.video-player-container {
  width: 100%;
  height: 100%;
}

/* Video size in the container */
.plyr.plyr--video {
  width: 100%;
  height: 100%;
}

/* Hide captions button when there are no subtitle tracks */
.video-player-container.no-captions .plyr__control[data-plyr="captions"] {
  display: none !important;
}

/* Subtitles style */
.plyr__captions {
  font-size: max(24px, 4.5vmin) !important;
  line-height: 150% !important; /* Line height needs to be in both, if not we'll not override the default of plyr */
  text-shadow:  /* The multiples shadows are for better readability since we are using a transparent background */
    0 0 6px #000,
    0 0 6px #000,
    0 0 6px #000,
    0 0 6px #000,
    0 0 6px #000 !important;
  font-weight: 700 !important;
  -webkit-font-smoothing: antialiased;
}

.plyr__caption {
  background: transparent !important;
  line-height: 150% !important;
}

/************
*** AUDIO ***
************/

.plyr.plyr--audio {
  border-radius: 12px;
}

/* Hide some unnesary buttons on the audio player */
.plyr--audio .plyr__control--overlaid,
.plyr--audio .plyr__control[data-plyr="captions"],
.plyr--audio .plyr__control[data-plyr="fullscreen"],
.plyr--audio .plyr__control[data-plyr="pip"] {
  display: none !important;
}

/* Style for audio player on mobile */
@media (max-width: 768px) {
  /* Buttons container more "big" for easy touch */
  .plyr--audio .plyr__control {
    min-width: 44px;
    min-height: 44px;
  }

  .plyr--audio .plyr__progress__container {
    margin: 10px 0;
  }

  .plyr--audio .plyr__controls__items {
    justify-content: center;
    gap: 12px;
  }

  /* Play button a bit more big */
  .plyr--audio .plyr__control--play {
    transform: scale(1.25);
  }
}

/*****************************
*** ALBUM ART AND METADATA ***
*****************************/

.audio-player-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1em;
  margin: 0 auto;
  padding: 1em;
  width: 100%;
  box-sizing: border-box;
  height: 100%;
  justify-content: center;
}

.audio-player-content {
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  flex-grow: 1;
  margin: 0 auto;
  gap: 1em;
  justify-content: center;
}

.album-art-container {
  height: 100%;
  width: 100%;
  border-radius: 1em;
  overflow: hidden;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.2);
  transition: max-height 0.3s ease, max-width 0.3s ease;
  will-change: transform;
}

.album-art {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.album-art-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 100%;
  border-radius: 18px;
  background: linear-gradient(115deg, var(--primaryColor), rgba(2, 0, 36, 0.9));
  filter: brightness(0.85);
}

.album-art-fallback i.material-icons {
  font-size: 5rem;
  color: white;
  opacity: 0.8;
  user-select: none;
}

.album-art-container.no-artwork {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.audio-title {
  font-size: max(1.4rem, 3.1vmin);
  font-weight: bold;
  margin-bottom: 8px;
  word-break: break-word;
}

.audio-metadata {
   text-align: center;
   color: whitesmoke;
   box-sizing: border-box;
   padding: 10px 15px;
   word-wrap: break-word;
}

.audio-artist,
.audio-album,
.audio-year {
  font-size: max(1.2rem, 2.5vmin);
  opacity: 0.8;
  margin-bottom: 5px;
  word-break: break-word;
}

.audio-controls-container {
  width: 100%;
  border-radius: 1em;
  margin: -2px;
}

/* For small tablets and phones with big screen */
@media (max-width: 740px) {
  .audio-player-container {
    padding: 0;
    padding-top: 1em;
  }

  .plyr.plyr--audio {
    padding: 1em;
    border-radius: 0;
  }

  .plyr--audio .plyr__controls {
    padding: 0;
    gap: 5px;
  }

  .audio-metadata {
    padding: 12px 15px;
  }

  .album-art-container {
    width: min(71vw);
    height: min(71vw);
    margin-top: 12px;
  }
}

@media (max-width: 550px) {
  /* Hide volume buttons to made more space */
  .plyr__volume {
    display: none;
  }

  /* Time playing */
  .plyr--audio .plyr__time {
    font-size: 14px;
    margin: 0 5px;
  }
}


/* For small screens in landscape orientation (Like a phone) */
@media (max-height: 600px) and (orientation: landscape) {

  .audio-player-container {
    justify-content: center;
    align-items: center;
    padding: 1em;
  }

  .audio-player-content {
    flex-direction: row;
    align-items: center;
    gap: 1.5em;
    width: auto;
    max-width: 90vw;
    margin: 0 auto;
  }

  .audio-metadata {
    text-align: left;
    margin: 0;
    padding: 15px;
    flex: 0 1 auto;
    align-self: center;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }

  .album-art-container {
    width: min(150px, 30vh);
    height: min(150px, 30vh);
    margin: 0;
    flex-shrink: 0;
  }
}

/*******************
*** QUEUE BUTTON ***
*******************/

/* Queue detection zone for top-right corner */
.queue-zone {
  position: fixed;
  top: 4em; /* Account for header bar */
  right: 0;
  width: 5em;
  height: 5em;
  pointer-events: auto;
  z-index: 1000;
  background: transparent;
}

.queue-button {
  position: fixed;
  top: 80px;
  right: 20px;
  width: 50px;
  height: 50px;
  border: none;
  border-radius: 50%;
  background: var(--background);
  color: var(--textPrimary);
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
  outline: none;
  z-index: 9998; /* Make sure it's below prompts but above other content */
}

.queue-button.dark-mode {
  background: var(--surfacePrimary);
}

.queue-button:hover {
  background: var(--primaryColor);
  transform: translateY(-2px) scale(1.05);
  box-shadow: 0 8px 25px rgba(var(--primaryColor-rgb), 0.3), 0 4px 12px rgba(0, 0, 0, 0.2);
  color: white;
}

.queue-button i.material-icons {
  font-size: 24px;
  transition: transform 0.2s ease;
}

.queue-button:hover i.material-icons {
  transform: scale(1.1);
}

.queue-count {
  position: absolute;
  top: -5px;
  right: -5px;
  background: var(--accentColor);
  color: white;
  border-radius: 50%;
  width: 20px;
  height: 20px;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: bold;
  text-shadow: 
    0 0 3px rgba(0, 0, 0, 0.9),
    0 0 5px rgba(0, 0, 0, 0.7),
    0 0 8px rgba(0, 0, 0, 0.5),
    0 0 8px rgba(0, 0, 0, 0.3);
}

.queue-button.hidden {
  opacity: 0;
  transform: translateY(-2px) scale(0.9);
  pointer-events: none !important;
  z-index: -1;
}

/* Smooth show animation for better UX */
.queue-button:not(.hidden) {
  animation: queue-button-show 0.4s ease-out;
}

@keyframes queue-button-show {
  0% {
    opacity: 0;
    transform: translateY(-2px) scale(0.8);
  }
  100% {
    opacity: 1;
    transform: translateY(-2px) scale(1);
  }
}

/*********************
*** PLAYBACK TOAST ***
**********************/

.playback-toast {
  position: fixed;
  bottom: 50px;
  left: 50%;
  transform: translateX(-50%);
  background: rgba(0, 0, 0, 0.8);
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
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
}

.playback-toast.visible {
  opacity: 1;
}

.playback-toast .material-icons {
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