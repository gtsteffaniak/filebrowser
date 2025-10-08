<template>
    <div class="plyr-viewer" :key="`${previewType}-${raw}`">
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
                    <img v-if="albumArtUrl" :src="albumArtUrl" :alt="audioMetadata.album || 'Album art'"
                        class="album-art" />
                    <div v-else class="album-art-fallback">
                        <i class="material-icons">music_note</i>
                    </div>
                </div>

                <!-- Metadata info -->
                <div class="metadata-info" v-if="audioMetadata">
                    <div class="audio-title">
                        {{ audioMetadata.title || req.name }}
                    </div>
                    <div class="audio-artist" v-if="audioMetadata.artist">
                        {{ audioMetadata.artist }}
                    </div>
                    <div class="audio-album" v-if="audioMetadata.album">
                        {{ audioMetadata.album }}
                    </div>
                    <div class="audio-year" v-if="audioMetadata.album">
                        {{ audioMetadata.year }}
                    </div>
                </div>

            </div>

            <div class="audio-controls-container" :class="{ 'dark-mode': darkMode, 'light-mode': !darkMode }">
                <vue-plyr ref="audioPlayer" :options="plyrOptions">
                    <audio :src="raw" :autoplay="shouldAutoPlay" @play="handlePlay"></audio>
                </vue-plyr>
            </div>
        </div>

        <!-- Video with plyr -->
        <vue-plyr v-else-if="previewType == 'video' && !useDefaultMediaPlayer" ref="videoPlayer"
            :options="plyrOptions">
            <video :src="raw" :autoplay="shouldAutoPlay" @play="handlePlay">
                <track kind="captions" v-for="(sub, index) in subtitlesList" :key="index" :src="sub.src"
                    :label="'Subtitle ' + sub.name" :default="index === 0" />
            </video>
        </vue-plyr>

        <!-- Default HTML5 Audio -->
        <audio v-else-if="previewType == 'audio' && useDefaultMediaPlayer" ref="defaultAudioPlayer" :src="raw"
            controls :autoplay="shouldAutoPlay" @play="handlePlay"></audio>

        <!-- Default HTML5 Video -->
        <video v-else-if="previewType == 'video' && useDefaultMediaPlayer" ref="defaultVideoPlayer" :src="raw"
            controls :autoplay="shouldAutoPlay" @play="handlePlay">
            <track kind="captions" v-for="(sub, index) in subtitlesList" :key="index" :src="sub.src"
                :label="'Subtitle ' + sub.name" :default="index === 0" />
        </video>

        <button
            v-if="showQueueButton"
            @click="showQueuePrompt"
            class="queue-button floating"
            :class="{
                'dark-mode': darkMode,
            }"
            :aria-label="$t('player.QueueButtonHint')"
            :title="$t('player.QueueButtonHint')"
        >
            <i class="material-icons">queue_music</i>
            <span v-if="queueCount > 0" class="queue-count">{{ queueCount }}</span>
        </button>

        <!-- Toast that shows when you press "P" or "L" on the media player -->
        <div :class="['loop-toast', toastVisible ? 'visible' : '']">
            <!-- Loop icon for "single playback", "loop single file" and "loop all files" -->
            <svg v-if="playbackMode === 'single' || playbackMode === 'loop-single' || playbackMode === 'loop-all'" class="loop-icon" viewBox="0 0 24 24">
                <path d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6 0 1.01-.25 1.97-.7 2.8l1.46 1.46A7.93 7.93 0 0020 12c0-4.42-3.58-8-8-8zm0 14c-3.31 0-6-2.69-6-6 0-1.01.25-1.97.7-2.8L5.24 7.74A7.93 7.93 0 004 12c0 4.42 3.58 8 8 8v3l4-4-4-4v3z" />
            </svg>

            <!-- Shuffle icon for "shuffle playback" -->
            <svg v-else-if="playbackMode === 'shuffle'" class="shuffle-icon" viewBox="0 0 24 24">
                <path d="M10.59 9.17L5.41 4 4 5.41l5.17 5.17 1.42-1.41zM14.5 4l2.04 2.04L4 18.59 5.41 20 17.96 7.46 20 9.5V4h-5.5zm.33 9.41l-1.41 1.41 3.13 3.13L14.5 20H20v-5.5l-2.04 2.04-3.13-3.13z"/>
            </svg>

            <!-- List icon for "sequential playback" -->
            <svg v-else class="sequential-icon" viewBox="0 0 24 24">
                <path d="M4 6h16v2H4zm0 5h16v2H4zm0 5h16v2H4z"/>
            </svg>

            <span>{{
                playbackMode === 'sequential' ? $t('player.PlayAllOncePlayback') :
                playbackMode === 'shuffle' ? $t('player.ShuffleAllPlayback') :
                playbackMode === 'loop-all' ? $t('player.PlayAllLoopedPlayback') :
                playbackMode === 'loop-single' ? $t('player.LoopEnabled') :
                $t('player.LoopDisabled') }}</span>

            <!-- Status indicator for loop -->
            <span v-if="playbackMode === 'single' || playbackMode === 'loop-single'" :class="[
                'status-indicator', playbackMode === 'loop-single' ? 'status-on' : 'status-off',]"></span>
        </div>
    </div>
</template>

<script>
import { state, mutations } from '@/store';
import { url } from '@/utils';
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
    },
    emits: ['play'],
    data() {
        return {
            loopEnabled: false, // The toast on the media player
            toastVisible: false,
            toastTimeout: null,
            audioMetadata: null, // Null by default, will be loaded from the audio file.
            albumArtUrl: null,
            albumArt: null,
            metadataId: 0,
            albumArtSize: 25, // Default size in em
            isHovering: false, // Track hover state
            // Playback settings
            playbackMenuInitialized: false,
            lastAppliedMode: null,
            isNavigating: false,
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
                clickToPlay: true,
                resetOnEnd: true,
                toggleInvert: false,
                preload: 'none',
            },
        };
    },
    watch: {
        req() {
            console.log('req changed, updating media');
            this.cleanupAlbumArt();
            this.updateMedia();
            this.playbackMenuInitialized = false;
            this.lastAppliedMode = null;

            // Re-hook event listeners after media source changes
            // This is critical - without this, the 'ended' event won't fire for the new video!
            this.$nextTick(() => {
                console.log('Re-hooking events after source change');
                this.hookEvents();
                
                // Update queue index to match current file
                this.updateCurrentQueueIndex();

                // Re-setup custom settings for the Plyr player
                if (!this.useDefaultMediaPlayer) {
                    console.log('Re-setting up custom playback settings after source change');
                    // Wait for Plyr to re-initialize its UI after source change
                    // Try multiple times with increasing delays to ensure Plyr is ready
                    const trySetupSettings = (attempt = 0) => {
                        const playerRef = this.getCurrentPlayer();
                        console.log('Attempt', attempt, 'to re-setup settings, playerRef:', playerRef);

                        if (playerRef && playerRef.player) {
                            const player = playerRef.player;

                            // Check if Plyr's settings are ready
                            if (player.elements && player.elements.settings && player.elements.settings.buttons) {
                                console.log('Plyr settings elements are ready, applying custom settings');
                                // Call applyCustomPlaybackSettings directly (not setupCustomPlaybackSettings)
                                // because the 'ready' event won't fire again after source change
                                this.applyCustomPlaybackSettings(player);
                            } else if (attempt < 5) {
                                // Retry if settings not ready yet
                                console.log('Plyr settings not ready yet, retrying...');
                                setTimeout(() => trySetupSettings(attempt + 1), 200);
                            } else {
                                console.error('Failed to set up custom settings after', attempt, 'attempts');
                            }
                        } else if (attempt < 5) {
                            // Retry if player not ready yet
                            console.log('Player not ready yet, retrying...');
                            setTimeout(() => trySetupSettings(attempt + 1), 200);
                        }
                    };

                    // Start trying after a small initial delay
                    setTimeout(() => trySetupSettings(0), 300);
                }
            });
        },
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
            immediate: true
        },
        shouldTogglePlayPause(newVal, oldVal) {
            if (newVal !== oldVal) {
            this.togglePlayPause();
            }
        },
    },
    computed: {
        darkMode() {
            return state.user.darkMode;
        },
        shouldAutoPlay() {
            // Use the autoPlayEnabled prop from parent
            return this.autoPlayEnabled;
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
        isPlaying() {
            return state.playbackQueue?.isPlaying || false;
        },
    },
    mounted() {
        this.updateMedia();
        this.hookEvents();
        this.syncWithStore();
        this.$nextTick(() => {
            this.setupPlaybackQueue();
        });
        document.addEventListener('keydown', this.handleKeydown);
    },
    beforeUnmount() {
        if (this.toastTimeout) {
            clearTimeout(this.toastTimeout);
        }
        this.cleanupAlbumArt();
        // Clean up media players
        if (this.$refs.videoPlayer && this.$refs.videoPlayer.player) {
            this.$refs.videoPlayer.player.destroy();
        }
        if (this.$refs.audioPlayer && this.$refs.audioPlayer.player) {
            this.$refs.audioPlayer.player.destroy();
        }
        document.removeEventListener('keydown', this.handleKeydown);
    },
    methods: {
        showQueuePrompt() {
            mutations.showHover({
                name: "PlaybackQueue",
            });
        },
        syncWithStore() {
            // Watch for store changes and update the player
            this.$watch(
                () => state.playbackQueue?.mode,
                (newMode) => {
                    if (newMode && newMode !== this.playbackMode) {
                        // ALWAYS force reshuffle when mode changes to shuffle
                        const forceReshuffle = newMode === 'shuffle';
                        this.setupPlaybackQueue(forceReshuffle);
                        this.showToast();
                        this.$nextTick(() => {
                        this.ensurePlaybackModeApplied();
                        });
                    }
                }
            );
        },
        togglePlayPause() {
            const player = this.getCurrentPlayer();
            if (!player) return;
            if (this.useDefaultMediaPlayer) {
                if (player.paused) {
                    player.play();
                } else {
                    player.pause();
                }
            } else {
                const plyrInstance = player.player;
                if (plyrInstance.playing) {
                    plyrInstance.pause();
                } else {
                    plyrInstance.play();
                }
            }
        },
        handlePlay() {
            this.$emit('play');
        },
        ensurePlaybackModeApplied() {
            if (this.useDefaultMediaPlayer) return;

            const playerRef = this.previewType === 'video'
                ? this.$refs.videoPlayer
                : this.$refs.audioPlayer;

            if (!playerRef || !playerRef.player) return;

            const player = playerRef.player;

            try {
                const settingsMenu = player.elements.settings?.menu;
                const playbackBtn = player.elements.settings?.buttons?.playback;

                if (settingsMenu && settingsMenu.style.display !== 'none' && settingsMenu.getAttribute('hidden') === null) {
                    this.applyCustomPlaybackSettings(player);
                } else if (playbackBtn && !this.playbackMenuInitialized) {
                    // Initial setup -- if menu hasn't been initialized yet
                    console.log('Initializing custom playback menu');
                    this.applyCustomPlaybackSettings(player);
                }
                // Otherwise, skip to avoid unnecessary recreation
            } catch (error) {
                console.error('Error ensuring playback mode applied:', error);
            }
        },
        focusPlayer() {
            this.$nextTick(() => {
                if (this.useDefaultMediaPlayer) {
                    // Focus default HTML5 players
                    const playerElement = this.previewType === 'video'
                        ? this.$refs.defaultVideoPlayer
                        : this.$refs.defaultAudioPlayer;

                    if (playerElement) {
                        playerElement.focus();
                        console.log('Focused default media player');
                    }
                } else {
                    // Focus Plyr players
                    const playerRef = this.previewType === 'video'
                        ? this.$refs.videoPlayer
                        : this.$refs.audioPlayer;

                    if (playerRef && playerRef.player && playerRef.player.elements.container) {
                        const container = playerRef.player.elements.container;
                        container.focus();

                        // Also try to focus the progress bar specifically
                        const progressContainer = container.querySelector('.plyr__progress__container');
                        if (progressContainer) {
                            progressContainer.focus();
                        }
                    }
                }
            });
        },
        toggleLoop() {
            // Always use our custom loop instead of Plyr's default
            this.loopEnabled = !this.loopEnabled;
            const newMode = this.loopEnabled ? 'loop-single' : 'single';

            // Update playback mode based on custom loop state
            mutations.setPlaybackQueue({
                queue: this.playbackQueue,
                currentIndex: this.currentQueueIndex,
                mode: newMode
            });

            // Update playback queue to reflect the new mode
            this.setupPlaybackQueue();
            this.showToast();

            // Sync the actual media element's loop state
            this.syncMediaLoopState();

            // Ensure player is focused and UI is updated
            this.focusPlayer();
            this.$nextTick(() => {
                this.ensurePlaybackModeApplied();
            });
        },
        handleKeydown(event) {
            // Handle 'P' and 'L' keys for loop and change playback
            if (event.key.toLowerCase() === 'p' || event.key.toLowerCase() === 'l') {
                event.stopPropagation();

                // Use requestAnimationFrame to ensure UI updates
                requestAnimationFrame(() => {
                    if (event.key.toLowerCase() === 'p') {
                        this.cyclePlaybackModes();
                    } else if (event.key.toLowerCase() === 'l') {
                        this.toggleLoop();
                    }
                });
            }
            // "Q" key for open the queue prompt
            if (event.key.toLowerCase() === 'q' && 
                state.prompts.length === 0) { // Only open if no other prompts are open
                event.stopPropagation();
                event.preventDefault();
                this.showQueuePrompt();
            }
        },
        cyclePlaybackModes() {
            // cycle order (excluding single and loop-single cuz they are handled by the "L" key)
            const modeCycle = ['loop-all', 'shuffle', 'sequential'];

            // Find current mode index in the cycle
            const currentIndex = modeCycle.indexOf(this.playbackMode);

            // Next mode index
            let nextIndex;
            if (currentIndex === -1) {
                // If current mode is not in cycle (single or loop-single), start from beginning
                nextIndex = 0;
            } else {
                nextIndex = (currentIndex + 1) % modeCycle.length;
            }

            // Set the new playback mode
            const newMode = modeCycle[nextIndex];
            mutations.setPlaybackQueue({
                queue: this.playbackQueue,
                currentIndex: this.currentQueueIndex,
                mode: newMode
            });

            console.log(`Playback mode changed to: ${newMode}`);

            // Update playback queue and force reshuffle
            const forceReshuffle = newMode === 'shuffle';
            this.setupPlaybackQueue(forceReshuffle);

            // Sync the actual media element's loop state
            this.syncMediaLoopState();

            // Show toast
            this.showToast();
            this.focusPlayer();
            this.$nextTick(() => {
                this.ensurePlaybackModeApplied();
            });
        },
        showToast() {
            if (this.toastTimeout) {
                clearTimeout(this.toastTimeout);
            }
            this.toastVisible = true;
            this.toastTimeout = setTimeout(() => {
                this.toastVisible = false;
            }, 2000);
        },
        async updateMedia() {
            this.cleanupAlbumArt();
            // Try to autoplay media, handle browser restrictions
            if (
                this.autoPlayEnabled &&
                (this.previewType === "video" || this.previewType === "audio")
            ) {
                this.$nextTick(() => {
                    if (this.useDefaultMediaPlayer) {
                        // Handle default HTML5 players
                        let playerRef =
                            this.previewType === "video"
                                ? this.$refs.defaultVideoPlayer
                                : this.$refs.defaultAudioPlayer;

                        if (playerRef) {
                            // Ensure player is not muted before attempting autoplay
                            playerRef.muted = false;
                            const playPromise = playerRef.play();
                            if (playPromise !== undefined) {
                                playPromise.catch((error) => {
                                    console.log("autoplay failed", error);
                                    // Don't force muted playback - let user manually start
                                });
                            }
                        }
                    } else {
                        // Handle vue-plyr players
                        let playerRef =
                            this.previewType === "video"
                                ? this.$refs.videoPlayer
                                : this.$refs.audioPlayer;

                        if (playerRef && playerRef.player) {
                            // Ensure player is not muted before attempting autoplay
                            playerRef.player.muted = false;
                            const playPromise = playerRef.player.play();
                            if (playPromise !== undefined) {
                                playPromise.catch((error) => {
                                    console.log("autoplay failed", error);
                                    // Don't force muted playback - let user manually start
                                });
                            }
                        }
                    }
                });
            }
            if (this.albumArt) {
                try {
                    URL.revokeObjectURL(this.albumArt);
                } catch (e) {Error;}
                this.albumArt = null;
            }
            this.albumArtUrl = null;
            this.audioMetadata = null;
            this.metadataId = (this.metadataId || 0) + 1;

            if (this.previewType === "audio") {
                this.loadAudioMetadata();
            }
            this.$nextTick(() => {
                setTimeout(() => {
                    this.focusPlayer();
                }, 300);
            });
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
        async loadAudioMetadata() {
            if (this.previewType !== "audio") {
                this.audioMetadata = null;
                this.cleanupAlbumArt();
                return;
            }

            try {
                // Clean up previous album art
                this.cleanupAlbumArt();

                // Check if metadata is already provided by the backend
                if (this.req.audioMeta) {
                    this.audioMetadata = {
                        title: this.req.audioMeta.title || null,
                        artist: this.req.audioMeta.artist || null,
                        album: this.req.audioMeta.album || null,
                        year: this.req.audioMeta.year || null,
                    };

                    // Handle base64 encoded album art from backend
                    if (this.req.audioMeta.albumArt) {
                        try {
                            // Decode base64 album art
                            const byteCharacters = atob(this.req.audioMeta.albumArt);
                            const byteNumbers = new Array(byteCharacters.length);
                            for (let i = 0; i < byteCharacters.length; i++) {
                                byteNumbers[i] = byteCharacters.charCodeAt(i);
                            }
                            const byteArray = new Uint8Array(byteNumbers);
                            const blob = new Blob([byteArray], { type: 'image/jpeg' });
                            this.albumArt = URL.createObjectURL(blob);
                            this.albumArtUrl = this.albumArt;
                        } catch (error) {
                            console.error("Failed to decode album art:", error);
                            this.cleanupAlbumArt();
                        }
                    } else {
                        this.cleanupAlbumArt();
                    }
                } else {
                    this.audioMetadata = null;
                    this.cleanupAlbumArt();
                }
            } catch (error) {
                this.audioMetadata = null;
                this.cleanupAlbumArt();
            }
        },
        cleanupAlbumArt() {
            if (this.albumArtUrl && this.albumArtUrl.startsWith('blob:')) {
                try {
                    URL.revokeObjectURL(this.albumArtUrl);
                    console.log('Cleaned up album art object URL');
                } catch (e) {
                    console.warn('Error revoking album art URL:', e);
                }
            }
            this.albumArtUrl = null;
            
            // Also clean up the backup reference if it exists
            if (this.albumArt && this.albumArt.startsWith('blob:')) {
                try {
                    URL.revokeObjectURL(this.albumArt);
                } catch (e) {
                    console.warn('Error revoking album art URL:', e);
                }
            }
            this.albumArt = null;
        },
        hookEvents() {
            if (!this.useDefaultMediaPlayer && this.$refs.videoPlayer && this.$refs.videoPlayer.player) {
                const player = this.$refs.videoPlayer.player;

                // Debug: Log player settings
                console.log('Plyr player initialized:', player);
                console.log('Plyr settings:', player.settings);
                console.log('Plyr controls:', player.config.controls);
                console.log('Plyr settings array:', player.config.settings);

                // Attach handlers only if the screen.orientation API is available.
                if (screen.orientation) {
                    player.on('enterfullscreen', this.onFullscreenEnter);
                    player.on('exitfullscreen', this.onFullscreenExit);
                }

                // Add media end event
                player.on('ended', this.handleMediaEnd);

                // Set up custom playback settings
                this.setupCustomPlaybackSettings(player);
                player.on('controlsshown', () => {
                    this.$nextTick(() => {
                        this.ensurePlaybackModeApplied();
                    });
                });
                player.on('play', () => {
                    mutations.setPlaybackState(true);
                    this.focusPlayer();
                });

                player.on('pause', () => {
                mutations.setPlaybackState(false);
                });

                player.on('ready', () => {
                    console.log('Video player ready, focusing');
                    this.focusPlayer();
                });
            }

            // Also debug audio player
            if (!this.useDefaultMediaPlayer && this.$refs.audioPlayer && this.$refs.audioPlayer.player) {
                const player = this.$refs.audioPlayer.player;
                console.log('Plyr audio player initialized:', player);
                console.log('Plyr audio settings:', player.settings);
                console.log('Plyr audio controls:', player.config.controls);
                console.log('Plyr audio settings array:', player.config.settings);

                // Add media end event
                player.on('ended', this.handleMediaEnd);

                // Set up custom playback settings
                this.setupCustomPlaybackSettings(player);
                player.on('controlsshown', () => {
                    this.$nextTick(() => {
                        this.ensurePlaybackModeApplied();
                    });
                });
                player.on('play', () => {
                    mutations.setPlaybackState(true);
                    this.focusPlayer();
                });

                player.on('pause', () => {
                mutations.setPlaybackState(false);
                });

                player.on('ready', () => {
                    console.log('Audio player ready, focusing');
                    this.focusPlayer();
                });
            }

            // Handle default HTML5 players
            if (this.useDefaultMediaPlayer) {
                const videoElement = this.$refs.defaultVideoPlayer;
                const audioElement = this.$refs.defaultAudioPlayer;

                if (videoElement) {
                    videoElement.addEventListener('ended', this.handleMediaEnd);
                    videoElement.addEventListener('play', () => {
                        mutations.setPlaybackState(true);
                        this.focusPlayer();
                    });
                    videoElement.addEventListener('pause', () => {
                        mutations.setPlaybackState(false);
                    });
                }
                if (audioElement) {
                    audioElement.addEventListener('ended', this.handleMediaEnd);
                    audioElement.addEventListener('play', () => {
                        mutations.setPlaybackState(true);
                        this.focusPlayer();
                    });
                    audioElement.addEventListener('pause', () => {
                        mutations.setPlaybackState(false);
                    });
                }
            }
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
            console.log('Setting up playback queue on mode:', this.playbackMode);
            console.log('Current req path:', this.req.path);

            // Get the current directory listing from the navigation state
            const listing = state.navigation?.listing || [];
            console.log('Current listing count:', listing.length);

            // Filter only audio/video files
            const mediaFiles = listing.filter(item => {
                const isAudio = item.type && item.type.startsWith('audio/');
                const isVideo = item.type && item.type.startsWith('video/');
                return isAudio || isVideo;
            });

            console.log('Filtered media files:', mediaFiles.length);

            if (mediaFiles.length === 0) {
                console.log('No media files found in current directory');
                mutations.setPlaybackQueue({
                queue: [],
                currentIndex: -1,
                mode: this.playbackMode
                });
                return;
            }

            // Find current file index of the file opened
            const currentIndex = mediaFiles.findIndex(item => item.path === this.req.path);
            console.log('Current file index in media files:', currentIndex);

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
                    // For sequential and loop-all, we'll use alphabetical order without rearranging
                    // On sequential mode will start playing from the file opened and find its place on the queue by the current index (you can see this on UI queue)
                    // Loop-all will do the same, but if the queue ends, will restart from the first file of the current folder (alphabetically) 
                    const sortedFiles = [...mediaFiles].sort((a, b) => a.name.localeCompare(b.name));
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
                    // For shuffle, include all files on random order and only reshuffle if forced (by cycling modes again)
                    // This is for preserve the current queue and don't lose it when is changed to the next file
                    if (forceReshuffle || this.playbackQueue.length === 0) {
                        const shuffledFiles = this.shuffleArray([...mediaFiles]);
                        finalQueue = shuffledFiles;
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

            console.log('Final playback queue length:', finalQueue.length);
            console.log('Current queue index:', finalIndex);

            // Log the paths for debugging
            this.playbackQueue.forEach((item, index) => {
                console.log(`Queue[${index}]:`, item.name, item.path);
            });
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
                console.log('Updated current queue index to:', currentIndex);
            } else {
                this.setupPlaybackQueue(true);
            }
        },
        async playNext() {
            console.log('Playing next, mode:', this.playbackMode, 'queue length:', this.playbackQueue.length, 'current index:', this.currentQueueIndex);

            if (this.isNavigating || this.playbackQueue.length === 0) {
                console.log('Cannot play next: navigating or empty queue');
                return;
            }

            this.isNavigating = true;

            // Calculate next index
            let nextIndex = this.currentQueueIndex + 1;

            // Handle end of queue based on mode
            if (nextIndex >= this.playbackQueue.length) {
                if (this.playbackMode === 'loop-all' || this.playbackMode === 'shuffle') {
                    // For shuffle mode, reshuffle the entire queue when we reach the end
                    if (this.playbackMode === 'shuffle') {
                        // Reshuffle the entire directory listing again
                        await this.setupPlaybackQueue(true); // Force reshuffle
                        nextIndex = 0;
                    } else {
                        // Loop back to beginning for loop-all mode
                        console.log('Reached end of queue, looping back to start');
                        nextIndex = 0;
                    }
                } else {
                    // Stop at end for sequential mode
                    console.log('Reached end of queue, stopping playback');
                    this.isNavigating = false;
                    return;
                }
            }

            const nextItem = this.playbackQueue[nextIndex];
            console.log('Moving to next item:', nextItem.name, 'at index:', nextIndex);

            try {
                // Update current index
                mutations.setPlaybackQueue({
                queue: this.playbackQueue,
                currentIndex: nextIndex,
                mode: this.playbackMode
                });

                // Build the proper URL for browser history
                const nextItemUrl = url.buildItemUrl(nextItem.source || this.req.source, nextItem.path);

                // Store the expected path before making changes
                const expectedPath = nextItem.path; 

                // Update state.req with the next item's data FIRST
                // This will trigger the watcher on req prop and update the media source
                mutations.replaceRequest(nextItem);

                // Then update the router URL
                // Use router.replace to properly update the route (for refresh to work)
                // This won't cause a remount because the component is keyed by route params, not the full path
                this.$router.replace({ path: nextItemUrl }).catch(err => {
                    if (err.name !== 'NavigationDuplicated') {
                        console.error('Router navigation error:', err);
                    }
                });

                // Wait for state.req to be updated
                await this.waitForReqUpdate(expectedPath); 

                const player = this.getCurrentPlayer();
                if (player) {
                    let playPromise;
                    if (this.useDefaultMediaPlayer) {
                        playPromise = player.play();
                    } else if (player.player) {
                        playPromise = player.player.play();
                    }
                    
                    if (playPromise !== undefined) {
                        playPromise.catch((error) => {
                            console.log("Auto-play prevented:", error);
                        });
                    }
                }

                // Reset navigation flag
                this.isNavigating = false;
            } catch (error) {
                console.error('Failed to navigate to next file:', error);
                this.isNavigating = false;
            }
        },

        async playPrevious() {
            if (this.isNavigating || this.playbackQueue.length === 0) return;

            this.isNavigating = true;

            let prevIndex = this.currentQueueIndex - 1;

            // Handle start of queue
            if (prevIndex < 0) {
                if (this.playbackMode === 'loop-all' || this.playbackMode === 'shuffle') {
                    prevIndex = this.playbackQueue.length - 1;
                } else {
                    this.isNavigating = false;
                    return;
                }
            }

            const prevItem = this.playbackQueue[prevIndex];

            try {
                mutations.setPlaybackQueue({
                queue: this.playbackQueue,
                currentIndex: prevIndex,
                mode: this.playbackMode
            });

            const prevItemUrl = url.buildItemUrl(prevItem.source || this.req.source, prevItem.path);

            // Store the expected path before making changes
			const expectedPath = prevItem.path;

            mutations.replaceRequest(prevItem);

            await this.$router.replace({ path: prevItemUrl }).catch(err => {
                if (err.name !== 'NavigationDuplicated') {
                    console.error('Router navigation error:', err);
                }
            });

            // Wait for state.req to be updated
            await this.waitForReqUpdate(expectedPath);
            
            const player = this.getCurrentPlayer();
            if (player) {
                let playPromise;
            if (this.useDefaultMediaPlayer) {
                playPromise = player.play();
            } else if (player.player) {
                playPromise = player.player.play();
            }

            if (playPromise !== undefined) {
                playPromise.catch((error) => {
                    console.log("Auto-play prevented:", error);
                });
            }
        }
        this.isNavigating = false;
            } catch (error) {
                console.error('Failed to navigate to previous file:', error);
               this.isNavigating = false;
            }
        },

        waitForReqUpdate(expectedPath) {
            return new Promise((resolve) => {
                if (state.req.path === expectedPath) {
                    resolve();
                    return;
                }

                const unwatch = this.$watch(
                    () => state.req.path,
                    (newPath) => {
                        if (newPath === expectedPath) {
                            unwatch();
                            resolve();
                        }
                    },
                    { immediate: false }
                );
            });
        },

        getCurrentPlayer() {
            // Return the appropriate player ref based on preview type and player type
            if (this.useDefaultMediaPlayer) {
                return this.previewType === 'video'
                    ? this.$refs.defaultVideoPlayer
                    : this.$refs.defaultAudioPlayer;
            } else {
                return this.previewType === 'video'
                    ? this.$refs.videoPlayer
                    : this.$refs.audioPlayer;
            }
        },
        restartCurrentFile() {
            console.log('Restarting current file');
            const player = this.getCurrentPlayer();
            if (player) {
                if (this.useDefaultMediaPlayer) {
                    // HTML5 player
                    player.currentTime = 0;
                    player.play();
                } else {
                    // Plyr player
                    if (player.player) {
                        player.player.currentTime = 0;
                        player.player.play();
                    }
                }
            }
        },
        handleMediaEnd() {
            console.log('Media ended on playback mode:');

            switch (this.playbackMode) {
                case 'single':
                    // Stop playback - do nothing
                    console.log('Play Once - stopping playback');
                    break;

                case 'loop-single':
                    console.log('Loop Current - restarting current file');
                    this.restartCurrentFile();
                    break;

                case 'sequential':
                case 'shuffle':
                case 'loop-all':
                    console.log('Moving to next file in queue');
                    this.playNext();
                    break;
            }
        },
        applyCustomPlaybackSettings(player) {
            // This is the actual logic to set up the settings menu
            // Separated so it can be called both on 'ready' event and after source changes

            // Sync loopEnabled with playbackMode
            this.loopEnabled = (this.playbackMode === 'loop-single');

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
                        console.log('Creating/recreating playback menu buttons with mode:', this.playbackMode);

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

                                // Update our state
                                this.playbackMode = value;

                                // Update button text
                                const currentMode = modeLabels[this.playbackMode] || 'Play Once';
                                playbackBtn.querySelector('span').innerHTML = `Playback: <span class="plyr__menu__value">${currentMode}</span>`;

                                // Set up playback queue
                                this.setupPlaybackQueue();

                                // Show toast
                                this.showToast();

                                // Ensure player stays focused and UI is updated
                                this.focusPlayer();
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

                    console.log('Custom playback settings applied successfully');
                } else {
                    console.error('Could not find playback button or panel');
                }
            } catch (error) {
                console.error('Error applying custom playback settings:', error);
            }
        },
        setupCustomPlaybackSettings(player) {
            console.log('Setting up custom playback settings for player:', player);

            // Wait for player to be ready (only fires once on initial load)
            player.on('ready', () => {
                console.log('Player ready, setting up custom settings');

                // Set up playback queue (needs to happen after navigation state is available)
                this.$nextTick(() => {
                    this.setupPlaybackQueue();
                });

                // Apply the custom settings
                this.applyCustomPlaybackSettings(player);
            });
        },
        syncMediaLoopState() {
            const player = this.getCurrentPlayer();
            if (!player) return;

            // Only enable loop for "Loop Current" mode
            const shouldLoop = this.playbackMode === 'loop-single';

            if (this.useDefaultMediaPlayer) {
                // HTML5 player
                player.loop = shouldLoop;
            } else {
                // Plyr player
                if (player.player) {
                    player.player.loop = shouldLoop;
                }
            }
            console.log('Loop state:', shouldLoop ? 'ON' : 'OFF');
        },
    },
};
</script>

<style >
@import url("plyr/dist/plyr.css");

.clickable:hover,
.plyr .plyr__control:hover,
button:hover,
.action:hover,
.listing-item.drag-hover {
    box-shadow:
        inset 0 -3em 3em rgba(217, 217, 217, 0.211),
        0 0 0 2px var(--alt-background) !important;
    /* Adjust shadow values as needed */
    transform: scale(1.02);
    /* Slightly enlarges the element */
}

.plyr-background {
    background: radial-gradient(#3b3b3b, black);
}

/**********************************
*** STYLES FOR THE MEDIA PLAYER ***
**********************************/

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
    border-radius: 12px;
    overflow: visible;
    background-color: rgb(216 216 216);
}

.plyr__controls {
    color: black;
}

.audio-controls-container.dark-mode .plyr {
    background-color: rgb(37 49 55 / 33%);
    color: white;
}
/* sidebar with backdrop-filter support */
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
    color: white;
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
    /* background: #00b2ff; */
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

/* Video settings (like rounded corners to the video itself) */
.plyr video {
    border-radius: 12px;
    width: 100%;
    height: 100%;
}

/* Video container size */
.plyr.plyr--video {
    width: 100%;
    height: 100%;
}

/* Force visibility of the buttons */
.plyr.plyr--video .plyr__control[data-plyr="captions"],
.plyr.plyr--video .plyr__control[data-plyr="pip"] {
    display: block !important;
}

/************
*** AUDIO ***
************/

/* Hide some unnesary buttons on the audio player */
.plyr--audio .plyr__control--overlaid,
.plyr--audio .plyr__control[data-plyr="captions"],
.plyr--audio .plyr__control[data-plyr="fullscreen"],
.plyr--audio .plyr__control[data-plyr="pip"] {
    display: none !important;
}

/* Style for audio player on mobile */
@media (max-width: 800px) {
    .plyr.plyr--audio {
        padding: 1em;
        border-radius: 0;
    }

    /* Buttons container more "big" for easy touch */
    .plyr--audio .plyr__control {
        min-width: 44px;
        min-height: 44px;
    }

    .plyr--audio .plyr__progress__container {
        margin: 10px 0;
    }

    .plyr--audio .plyr__controls {
        padding: 0;
        gap: 5px;
    }

    .plyr--audio .plyr__controls__items {
        justify-content: center;
        gap: 12px;
    }

    /* Play button a bit more big */
    .plyr--audio .plyr__control--play {
        transform: scale(1.25);
    }

    /* Hide volume buttons for made more space */
    .plyr--audio .plyr__volume {
        display: none;
    }

    /* Hide some items on video player*/
    .plyr--video .plyr__volume {
        display: none;
    }

    /* Time playing */
    .plyr--audio .plyr__time {
        font-size: 14px;
        margin: 0 5px;
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
    font-size: clamp(1.2rem, 4vw, 1.5rem);
    font-weight: bold;
    margin-bottom: 8px;
    word-break: break-word;
}

.metadata-info {
   text-align: center;
   color: whitesmoke;
   box-sizing: border-box;
   padding: 10px 15px;
   word-wrap: break-word;
}

.audio-artist,
.audio-album,
.audio-year {
    font-size: clamp(1rem, 3vw, 1.1rem);
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
@media (max-width: 800px) {
    .audio-player-container {
        padding: 0;
        padding-top: 1em;
    }

    .metadata-info {
        padding: 12px 15px;
    }

    .album-art-container {
        width: min(280px, 70vw);
        height: min(280px, 70vw);
        margin-top: 10px;
    }
}

/* For ultra-wide screens. This need test, I'm not sure if will work correctly */
@media (min-width: 1600px) {
    .album-art-container {
        width: min(400px, 25vw);
        height: min(400px, 25vw);
    }
}

/* For small screens in landscape orientation (Like a phone) */
@media (max-height: 500px) and (orientation: landscape) {

    .audio-player-container {
        justify-content: center;
        align-items: center;
        padding: 1em;
    }

    .audio-player-content {
        display: flex;
        flex-direction: row;
        justify-content: center;
        align-items: center;
        gap: 1.5em;
        width: auto;
        max-width: 90vw;
        margin: 0 auto;
    }

    .metadata-info {
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

    .audio-title {
      font-size: clamp(1rem, 3vw, 1.3rem);
    }

    .audio-artist,
    .audio-album,
    .audio-year {
      font-size: clamp(0.85rem, 2vw, 1rem);
    }
}

/*******************
*** QUEUE BUTTON ***
*******************/

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
}

/*****************
*** LOOP TOAST ***
*****************/

.loop-toast {
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

.loop-toast.visible {
    opacity: 1;
}

.loop-icon {
    width: 24px;
    height: 24px;
    fill: white;
}

.shuffle-icon,
.sequential-icon {
    width: 24px;
    height: 24px;
    fill: white;
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
