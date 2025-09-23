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
                    <audio :src="raw" :autoplay="autoPlayEnabled" @play="handlePlay"></audio>
                </vue-plyr>
            </div>
        </div>

        <!-- Video with plyr -->
        <vue-plyr v-else-if="previewType == 'video' && !useDefaultMediaPlayer" ref="videoPlayer"
            :options="plyrOptions">
            <video :src="raw" :autoplay="autoPlayEnabled" @play="handlePlay">
                <track kind="captions" v-for="(sub, index) in subtitlesList" :key="index" :src="sub.src"
                    :label="'Subtitle ' + sub.name" :default="index === 0" />
            </video>
        </vue-plyr>

        <!-- Default HTML5 Audio -->
        <audio v-else-if="previewType == 'audio' && useDefaultMediaPlayer" ref="defaultAudioPlayer" :src="raw"
            controls :autoplay="autoPlayEnabled" @play="handlePlay"></audio>

        <!-- Default HTML5 Video -->
        <video v-else-if="previewType == 'video' && useDefaultMediaPlayer" ref="defaultVideoPlayer" :src="raw"
            controls :autoplay="autoPlayEnabled" @play="handlePlay">
            <track kind="captions" v-for="(sub, index) in subtitlesList" :key="index" :src="sub.src"
                :label="'Subtitle ' + sub.name" :default="index === 0" />
        </video>

        <!-- Loop indicator, shows when you press "L" on the player -->
        <div :class="['loop-toast', toastVisible ? 'visible' : '']">
            <svg class="loop-icon" viewBox="0 0 24 24">
                <path
                    d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6 0 1.01-.25 1.97-.7 2.8l1.46 1.46A7.93 7.93 0 0020 12c0-4.42-3.58-8-8-8zm0 14c-3.31 0-6-2.69-6-6 0-1.01.25-1.97.7-2.8L5.24 7.74A7.93 7.93 0 004 12c0 4.42 3.58 8 8 8v3l4-4-4-4v3z" />
            </svg>
            <span>{{
                loopEnabled
                    ? $t("player.LoopEnabled")
                    : $t("player.LoopDisabled")
            }}</span>
            <span :class="[
                'status-indicator',
                loopEnabled ? 'status-on' : 'status-off',
            ]"></span>
        </div>
    </div>
</template>

<script>
import { state } from '@/store/state';
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
                settings: ["quality", "speed", "loop"],
                speed: {
                    selected: 1,
                    options: [0.25, 0.5, 0.75, 1, 1.25, 1.5, 2],
                },
                disableContextMenu: true,
                seekTime: 10,
                hideControls: true,
                keyboard: { focused: true, global: true },
                tooltips: { controls: true, seek: true },
                loop: { active: true },
                blankVideo: "",
                muted: false, // Disable muting automatically
                autoplay: false, // The users will manage this from their profile settings
                clickToPlay: true,
                resetOnEnd: true,
                toggleInvert: false,
            },
        };
    },
    watch: {
        req() {
            this.updateMedia();
        },
    },
    computed: {
        darkMode() {
            return state.user.darkMode;
        },
    },
    mounted() {
        this.updateMedia();
        this.hookEvents();
    },
    beforeUnmount() {
        if (this.toastTimeout) {
            clearTimeout(this.toastTimeout);
        }
        if (this.albumArt) {
            try {
                URL.revokeObjectURL(this.albumArt);
            } catch (e) {Error;}
            this.albumArt = null;
        }
    },
    methods: {
        handlePlay() {
            this.$emit('play');
        },
        toggleLoop() {
            if (this.useDefaultMediaPlayer) {
                // Handle default HTML5 players
                let playerRef =
                    this.previewType === "video"
                        ? this.$refs.defaultVideoPlayer
                        : this.$refs.defaultAudioPlayer;

                if (playerRef) {
                    this.loopEnabled = !this.loopEnabled;
                    playerRef.loop = this.loopEnabled;
                    this.showToast();
                }
            } else {
                // Handle vue-plyr players
                let playerRef =
                    this.previewType === "video"
                        ? this.$refs.videoPlayer
                        : this.$refs.audioPlayer;

                if (playerRef && playerRef.player) {
                    // Toggle loop mode
                    this.loopEnabled = !this.loopEnabled;
                    playerRef.player.loop = this.loopEnabled;
                    this.showToast();
                }
            }
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
                this.albumArtUrl = null;
                return;
            }

            try {
                // Clean up previous album art
                if (this.albumArt) {
                    URL.revokeObjectURL(this.albumArt);
                    this.albumArt = null;
                }

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
                            this.albumArtUrl = null;
                        }
                    } else {
                        this.albumArtUrl = null;
                    }
                } else {
                    this.audioMetadata = null;
                    this.albumArtUrl = null;
                }
            } catch (error) {
                this.audioMetadata = null;
                this.albumArtUrl = null;
            }
        },
        hookEvents() {
            if (!this.useDefaultMediaPlayer && this.$refs.videoPlayer && this.$refs.videoPlayer.player) {
                const player = this.$refs.videoPlayer.player;

                // Attach handlers only if the screen.orientation API is available.
                if (screen.orientation) {
                    player.on('enterfullscreen', this.onFullscreenEnter);
                    player.on('exitfullscreen', this.onFullscreenExit);
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
    },
    expose: ['toggleLoop'],
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
