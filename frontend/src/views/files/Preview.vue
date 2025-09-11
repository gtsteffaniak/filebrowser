<template>
    <div
        id="previewer"
        @mousemove="toggleNavigation"
        @touchstart="toggleNavigation"
    >
        <div class="preview" v-if="!isDeleted">
            <ExtendedImage
                v-if="showImage"
                :src="raw"
                @navigate-previous="prev"
                @navigate-next="next"
            >
            </ExtendedImage>

            <!-- Audio with plyr -->
            <div
                v-else-if="previewType == 'audio' && !useDefaultMediaPlayer"
                class="audio-player-container"
            >
                <!-- Album art with a generic icon if no image/metadata -->
                <div
                    class="album-art-container"
                    :class="{ 'no-artwork': !albumArtUrl }"
                >
                    <img
                        v-if="albumArtUrl"
                        :src="albumArtUrl"
                        :alt="audioMetadata.album || 'Album art'"
                        class="album-art"
                    />
                    <div v-else class="album-art-fallback">
                        <i class="material-icons">music_note</i>
                    </div>
                </div>

                <!-- Metadata info -->
                <div class="metadata-info" v-if="audioMetadata">
                    <div class="audio-title">
                        {{ audioMetadata.title || state.req.name }}
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

                <div class="audio-controls-container">
                    <vue-plyr ref="audioPlayer" :options="plyrOptions">
                        <audio
                            :src="raw"
                            :autoplay="autoPlay"
                            @play="autoPlay = true"
                        ></audio>
                    </vue-plyr>
                </div>
            </div>

            <!-- Video with plyr -->
            <vue-plyr
                v-else-if="previewType == 'video' && !useDefaultMediaPlayer"
                ref="videoPlayer"
                :options="plyrOptions"
            >
                <video :src="raw" :autoplay="autoPlay" @play="autoPlay = true">
                    <track
                        kind="captions"
                        v-for="(sub, index) in subtitlesList"
                        :key="index"
                        :src="sub.src"
                        :label="'Subtitle ' + sub.name"
                        :default="index === 0"
                    />
                </video>
            </vue-plyr>

            <!-- Default HTML5 Audio -->
            <audio
                v-else-if="previewType == 'audio' && useDefaultMediaPlayer"
                ref="defaultAudioPlayer"
                :src="raw"
                controls
                :autoplay="autoPlay"
                @play="autoPlay = true"
            ></audio>

            <!-- Default HTML5 Video -->
            <video
                v-else-if="previewType == 'video' && useDefaultMediaPlayer"
                ref="defaultVideoPlayer"
                :src="raw"
                controls
                :autoplay="autoPlay"
                @play="autoPlay = true"
            >
                <track
                    kind="captions"
                    v-for="(sub, index) in subtitlesList"
                    :key="index"
                    :src="sub.src"
                    :label="'Subtitle ' + sub.name"
                    :default="index === 0"
                />
            </video>

            <div v-else-if="previewType == 'pdf'" class="pdf-wrapper">
                <iframe class="pdf" :src="raw"></iframe>
                <a
                    v-if="isMobileSafari"
                    :href="raw"
                    target="_blank"
                    class="button button--flat floating-btn"
                >
                    <div>
                        <i class="material-icons">open_in_new</i
                        >{{ $t("buttons.openFile") }}
                    </div>
                </a>
            </div>

            <div v-else class="info">
                <div class="title">
                    <i class="material-icons">feedback</i>
                    {{ $t("files.noPreview") }}
                </div>
                <div>
                    <a
                        target="_blank"
                        :href="downloadUrl"
                        class="button button--flat"
                    >
                        <div>
                            <i class="material-icons">file_download</i
                            >{{ $t("buttons.download") }}
                        </div>
                    </a>
                    <a
                        target="_blank"
                        :href="raw"
                        class="button button--flat"
                        v-if="req.type != 'directory'"
                    >
                        <div>
                            <i class="material-icons">open_in_new</i
                            >{{ $t("buttons.openFile") }}
                        </div>
                    </a>
                </div>
            </div>
        </div>

        <!-- Loop indicator, shows when you press "L" on the player -->
        <div :class="['loop-toast', toastVisible ? 'visible' : '']">
            <svg class="loop-icon" viewBox="0 0 24 24">
                <path
                    d="M12 4V1L8 5l4 4V6c3.31 0 6 2.69 6 6 0 1.01-.25 1.97-.7 2.8l1.46 1.46A7.93 7.93 0 0020 12c0-4.42-3.58-8-8-8zm0 14c-3.31 0-6-2.69-6-6 0-1.01.25-1.97.7-2.8L5.24 7.74A7.93 7.93 0 004 12c0 4.42 3.58 8 8 8v3l4-4-4-4v3z"
                />
            </svg>
            <span>{{
                loopEnabled
                    ? $t("player.LoopEnabled")
                    : $t("player.LoopDisabled")
            }}</span>
            <span
                :class="[
                    'status-indicator',
                    loopEnabled ? 'status-on' : 'status-off',
                ]"
            ></span>
        </div>

        <button
            @click="prev"
            @mouseover="hoverNav = true"
            @mouseleave="hoverNav = false"
            :class="{ hidden: !hasPrevious || !showNav }"
            :aria-label="$t('buttons.previous')"
            :title="$t('buttons.previous')"
        >
            <i class="material-icons">chevron_left</i>
        </button>
        <button
            @click="next"
            @mouseover="hoverNav = true"
            @mouseleave="hoverNav = false"
            :class="{ hidden: !hasNext || !showNav }"
            :aria-label="$t('buttons.next')"
            :title="$t('buttons.next')"
        >
            <i class="material-icons">chevron_right</i>
        </button>
        <link rel="prefetch" :href="previousRaw" />
        <link rel="prefetch" :href="nextRaw" />
    </div>
</template>
<script>
import { filesApi, publicApi } from "@/api";
import { url } from "@/utils";
import throttle from "@/utils/throttle";
import ExtendedImage from "@/components/files/ExtendedImage.vue";
import { state, getters, mutations } from "@/store";
import { getFileExtension } from "@/utils/files";
import { convertToVTT } from "@/utils/subtitles";
import { getTypeInfo } from "@/utils/mimetype";
import { globalVars, shareInfo } from "@/utils/constants";
// Audio metadata is now provided by the backend

export default {
    name: "preview",
    components: {
        ExtendedImage,
    },
    data() {
        return {
            previousLink: "",
            nextLink: "",
            listing: null,
            name: "",
            fullSize: true,
            showNav: true,
            navTimeout: null,
            hoverNav: false,
            previousRaw: "",
            nextRaw: "",
            currentPrompt: null, // Replaces Vuex getter `currentPrompt`
            subtitlesList: [],
            isDeleted: false,
            tapTimeout: null,
            loopEnabled: false, // The toast on the media player
            toastVisible: false,
            toastTimeout: null,
            audioMetadata: null, // Null by default, will be loaded from the audio file.
            albumArtUrl: null,
            albumArt: null,
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
                autoplay: false, // The users will manage this from their profile settings
                clickToPlay: true,
                resetOnEnd: true,
                toggleInvert: false,
            },
        };
    },
    computed: {
        showImage() {
            return (this.previewType == 'image' || this.pdfConvertable) && (!globalVars.disableHeicConversion && state.req.type == "image/heic");
        },
        autoPlay() {
            return state.user.preview.autoplayMedia;
        },
        useDefaultMediaPlayer() {
            return state.user.preview.defaultMediaPlayer === true;
        },
        isMobileSafari() {
            const userAgent = window.navigator.userAgent;
            const isIOS =
                /iPad|iPhone|iPod/.test(userAgent) && !window.MSStream;
            const isSafari = /^((?!chrome|android).)*safari/i.test(userAgent);
            return isIOS && isSafari;
        },
        pdfConvertable() {
            if (!globalVars.muPdfAvailable) {
                return false;
            }
            const ext = "." + state.req.name.split(".").pop().toLowerCase(); // Ensure lowercase and dot
            const pdfConvertCompatibleFileExtensions = {
                ".xps": true,
                ".mobi": true,
                ".fb2": true,
                ".cbz": true,
                ".svg": true,
                ".docx": true,
                ".pptx": true,
                ".xlsx": true,
                ".hwp": true,
                ".hwpx": true,
            };
            if (state.user.disableViewingExt.includes(ext)) {
                return false;
            }
            return !!pdfConvertCompatibleFileExtensions[ext];
        },
        sidebarShowing() {
            return getters.isSidebarVisible();
        },
        previewType() {
            return getters.previewType();
        },
        raw() {
            const showFullSizeHeic = state.req.type === "image/heic" && !state.isSafari && globalVars.mediaAvailable && !globalVars.disableHeicConversion;
            if (this.pdfConvertable || showFullSizeHeic) {
                if (getters.isShare()) {
                    const previewPath = url.removeTrailingSlash(state.req.path);
                    return publicApi.getPreviewURL(previewPath, "original");
                }
                return (
                    filesApi.getPreviewURL(
                        state.req.source,
                        state.req.path,
                        state.req.modified,
                    ) + "&size=original"
                );
            }
            if (getters.isShare()) {
                return publicApi.getDownloadURL(
                    {
                        path: state.share.subPath,
                        hash: state.share.hash,
                        token: state.share.token,
                    },
                    [state.req.path],
                    true,
                );
            }
            return filesApi.getDownloadURL(
                state.req.source,
                state.req.path,
                true,
            );
        },
        isDarkMode() {
            return getters.isDarkMode();
        },
        hasPrevious() {
            return this.previousLink !== "";
        },
        hasNext() {
            return this.nextLink !== "";
        },
        downloadUrl() {
            if (getters.isShare()) {
                return publicApi.getDownloadURL(
                    {
                        path: state.share.subPath,
                        hash: state.share.hash,
                        token: state.share.token,
                    },
                    [state.req.path],
                );
            }
            return filesApi.getDownloadURL(state.req.source, state.req.path);
        },
        getSubtitles() {
            return this.subtitles();
        },
        req() {
            return state.req;
        },
        deletedItem() {
            return state.deletedItem;
        },
        disableFileViewer() {
            return shareInfo.disableFileViewer;
        },
    },
    watch: {
        deletedItem() {
            if (!state.deletedItem) {
                return;
            }
            this.isDeleted = true;
            this.listing = null; // Invalidate the listing to force a refresh
            this.nextRaw = "";
            this.previousRaw = "";
            if (this.hasNext) {
                this.next();
            } else if (!this.hasPrevious && !this.hasNext) {
                this.close();
            } else {
                this.prev();
            }
            mutations.setDeletedItem(false);
        },
        req() {
            if (!getters.isLoggedIn()) {
                return;
            }
            this.isDeleted = false;
            this.updatePreview();
            this.toggleNavigation();
            mutations.resetSelected();
            mutations.addSelected({
                name: state.req.name,
                path: state.req.path,
                size: state.req.size,
                type: state.req.type,
                source: state.req.source,
            });
        },
    },
    async mounted() {
        if (state.req.items) {
            this.listing = state.req.items;
        }
        mutations.setDeletedItem(false);
        window.addEventListener("keydown", this.keyEvent);
        this.subtitlesList = await this.subtitles();
        this.updatePreview();
        mutations.resetSelected();
        mutations.addSelected({
            name: state.req.name,
            path: state.req.path,
            size: state.req.size,
            type: state.req.type,
            source: state.req.source,
        });
        if (this.previewType === "audio") {
            this.loadAudioMetadata();
        }
    },
    beforeUnmount() {
        window.removeEventListener("keydown", this.keyEvent);
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
        async subtitles() {
            if (!state.req?.subtitles?.length) {
                return [];
            }
            let subs = [];
            for (const subtitleTrack of state.req.subtitles) {
                // All subtitle content is now pre-loaded when content=true
                // Simply use the content that's already available
                if (
                    !subtitleTrack.content ||
                    subtitleTrack.content.length === 0
                ) {
                    console.warn(
                        "Subtitle track has no content:",
                        subtitleTrack.name,
                    );
                    continue;
                }
                let vttContent = subtitleTrack.content;
                if (!subtitleTrack.content.startsWith("WEBVTT")) {
                    const ext = getFileExtension(subtitleTrack.name);
                    vttContent = convertToVTT(ext, subtitleTrack.content);
                }
                if (vttContent.startsWith("WEBVTT")) {
                    // Create a virtual file (Blob) and get a URL for it
                    const blob = new Blob([vttContent], { type: "text/vtt" });
                    const vttURL = URL.createObjectURL(blob);
                    subs.push({
                        name: subtitleTrack.name,
                        src: vttURL,
                    });
                } else {
                    console.warn(
                        "Skipping subtitle track because it has no WEBVTT header:",
                        subtitleTrack.name,
                    );
                }
            }
            return subs;
        },
        prev() {
            this.hoverNav = false;
            this.$router.replace({ path: this.previousLink });
        },
        next() {
            this.hoverNav = false;
            this.$router.replace({ path: this.nextLink });
        },
        async keyEvent(event) {
            if (getters.currentPromptName()) {
                return;
            }

            const { key } = event;

            switch (key) {
                case "ArrowRight":
                    if (this.hasNext) {
                        this.next();
                    }
                    break;
                case "ArrowLeft":
                    if (this.hasPrevious) {
                        this.prev();
                    }
                    break;
                case "Delete":
                    mutations.showHover("delete");
                    break;
                case "Escape":
                case "Backspace":
                    this.close();
                    break;
                case "l":
                case "L":
                    // Toggle loop mode with 'L' key
                    if (
                        this.previewType === "video" ||
                        this.previewType === "audio"
                    ) {
                        event.preventDefault();
                        this.toggleLoop();
                    }
                    break;
            }
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
        async updatePreview() {
            // Try to autoplay media, handle browser restrictions
            if (
                this.autoPlay &&
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
                            const playPromise = playerRef.play();
                            if (playPromise !== undefined) {
                                playPromise.catch((error) => {
                                    console.log("autoplay failed", error);
                                    playerRef.muted = true;
                                    playerRef.play();
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
                            const playPromise = playerRef.player.play();
                            if (playPromise !== undefined) {
                                playPromise.catch((error) => {
                                    console.log("autoplay failed", error);
                                    playerRef.player.muted = true;
                                    playerRef.player.play();
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
            const directoryPath = url.removeLastDir(state.req.path);
            if (!this.listing || this.listing == "undefined") {
                let res;
                if (getters.isShare()) {
                    // Use public API for shared files
                    res = await publicApi.fetchPub(
                        directoryPath,
                        state.share.hash,
                    );
                } else {
                    // Use regular files API for authenticated users
                    res = await filesApi.fetchFiles(
                        state.req.source,
                        directoryPath,
                    );
                }
                this.listing = res.items;
            }
            if (!this.listing) {
                this.listing = [state.req];
            }
            this.name = state.req.name;
            this.previousLink = "";
            this.nextLink = "";

            for (let i = 0; i < this.listing.length; i++) {
                if (this.listing[i].name !== this.name) {
                    continue;
                }
                for (let j = i - 1; j >= 0; j--) {
                    let clistItem = this.listing[j];
                    // Skip directories - only navigate between files
                    if (clistItem.type === 'directory') {
                        continue;
                    }
                    clistItem.path =
                        directoryPath + "/" + clistItem.name;
                    this.previousLink = url.buildItemUrl(
                        clistItem.source,
                        clistItem.path,
                    );
                    if (
                        getTypeInfo(clistItem.type).simpleType == "image"
                    ) {
                        this.previousRaw = this.prefetchUrl(clistItem);
                    }
                    break;
                }
                for (let j = i + 1; j < this.listing.length; j++) {
                    let clistItem = this.listing[j];
                    // Skip directories - only navigate between files
                    if (clistItem.type === 'directory') {
                        continue;
                    }
                    clistItem.path = directoryPath + "/" + clistItem.name;
                    this.nextLink = url.buildItemUrl(clistItem.source,clistItem.path);
                    if (getTypeInfo(clistItem.type).simpleType == "image") {
                        this.nextRaw = this.prefetchUrl(clistItem);
                    }
                    break;
                }
                return;
            }
        },

        prefetchUrl(item) {
            if (getters.isShare()) {
                return this.fullSize
                    ? publicApi.getDownloadURL(
                          {
                              path: item.path,
                              hash: state.share.hash,
                              token: state.share.token,
                              inline: true,
                          },
                          [item.path],
                      )
                    : publicApi.getPreviewURL(state.share.hash, item.path);
            }
            return this.fullSize
                ? filesApi.getDownloadURL(state.req.source, item.path, true)
                : filesApi.getPreviewURL(
                      state.req.source,
                      item.path,
                      item.modified,
                  );
        },
        resetPrompts() {
            this.currentPrompt = null;
        },
        toggleSize() {
            this.fullSize = !this.fullSize;
        },
        toggleNavigation: throttle(function () {
            this.showNav = true;

            if (this.navTimeout) {
                clearTimeout(this.navTimeout);
            }

            this.navTimeout = setTimeout(() => {
                this.showNav = false || this.hoverNav;
                this.navTimeout = null;
            }, 1500);
        }, 100),
        close() {
            mutations.replaceRequest({}); // Reset request data
            let uri = url.removeLastDir(state.route.path) + "/";
            this.$router.push({ path: uri });
        },
        download() {
            const items = [state.req];
            downloadFiles(items);
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
                if (state.req.audioMeta) {
                    this.audioMetadata = {
                        title: state.req.audioMeta.title || null,
                        artist: state.req.audioMeta.artist || null,
                        album: state.req.audioMeta.album || null,
                        year: state.req.audioMeta.year || null,
                    };

                    // Handle base64 encoded album art from backend
                    if (state.req.audioMeta.albumArt) {
                        try {
                            // Decode base64 album art
                            const byteCharacters = atob(state.req.audioMeta.albumArt);
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
    },
};
</script>

<style>
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
.pdf-wrapper {
    position: relative;
    width: 100%;
    height: 100%;
}

.pdf-wrapper .pdf {
    width: 100%;
    height: 100%;
    border: 0;
}

.pdf-wrapper .floating-btn {
    background: rgba(0, 0, 0, 0.5);
    color: white;
}

.pdf-wrapper .floating-btn:hover {
    background: rgba(0, 0, 0, 0.7);
}

/**********************************
*** STYLES FOR THE MEDIA PLAYER ***
**********************************/

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
    --plyr-audio-controls-background: transparent;
    --plyr-video-controls-background: linear-gradient(
        transparent,
        rgba(0, 0, 0, 0.7)
    );
    border-radius: 12px;
    overflow: visible;
}

/* Position/space of the buttons */
.plyr .plyr__controls {
    display: flex;
    flex-direction: row;
    gap: 8px;
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
    position: fixed;
    transition: 0.3s;
    z-index: 2;
    height: 4em;
    transform: none;
    padding: unset;
    left: unset;
    right: unset;
    bottom: unset;
    width: 4em !important;
    border-radius: 5em !important;
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

/* Style for audio player */
.plyr.plyr--audio {
    background: rgba(40, 40, 55, 1);
    border-radius: 16px;
    padding: 15px;
    max-width: flex;
    width: 75%;
    max-height: 300px;
    margin: auto auto;
    position: absolute;
    bottom: 18px;
    left: 50%;
    transform: translateX(-50%);
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
    .plyr.plyr--audio {
        position: fixed;
        bottom: 0;
        left: 0;
        right: 0;
        width: 100%;
        max-width: 100%;
        max-height: 70px;
        border-radius: 5px;
        padding: 10px 15px;
        box-shadow: 0 -2px 10px rgba(0, 0, 0, 0.3);
        margin: 0;
        transform: none;
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
        transform: scale(1.2);
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
    gap: 20px;
    max-width: 800px;
    margin: 0 auto;
    padding: 20px;
    width: 100%;
    box-sizing: border-box;
    height: 100%;
    justify-content: center;
}

.album-art-container {
    width: min(350px, 80vw);
    height: min(350px, 80vw);
    border-radius: 12px;
    overflow: hidden;
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.2);
    margin-top: 30px;
    bottom: 0;
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
    background: linear-gradient(
        115deg,
        var(--primaryColor),
        rgba(2, 0, 36, 0.9)
    );
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

.audio-metadata {
    text-align: center;
    color: var(--text-color);
    margin-top: 15px;
    padding: 15px;
    background: rgba(0, 0, 0, 0.05);
    border-radius: 8px;
    width: 100%;
    max-width: min(350px, 80vw);
    box-sizing: border-box;
}

.audio-title {
    font-size: clamp(1.2rem, 4vw, 1.5rem);
    font-weight: bold;
    margin-bottom: 8px;
    word-break: break-word;
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
    margin-top: 15px;
}

/* Mobile */
@media (max-width: 480px) and (orientation: portrait) {
    .audio-player-container {
        padding: 10px 15px 20px;
        gap: 20px;
        justify-content: center;
        min-height: 70vh;
        display: flex;
    }

    .album-art-container {
        width: min(280px, 75vw);
        height: min(280px, 75vw);
        margin-top: 50px;
    }

    .album-art-fallback i.material-icons {
        font-size: 4rem;
    }

    .audio-metadata {
        max-width: min(300px, 85vw);
        margin-top: 5px;
        padding: 12px 15px;
        background: rgba(0, 0, 0, 0.08);
    }

    .audio-title {
        font-size: 1.4rem;
        margin-bottom: 6px;
    }

    .audio-artist {
        font-size: 1.1rem;
        font-weight: 500;
        margin-bottom: 4px;
    }

    .audio-album {
        font-size: 1rem;
    }

    .audio-controls-container {
        width: 100%;
        margin-top: 20px;
        position: relative;
    }
}

/* For medium screens (like tablets) */
@media (max-width: 1024px) and (min-width: 769px) {
    .album-art-container {
        width: min(300px, 60vw);
        height: min(300px, 60vw);
    }

    .audio-metadata {
        max-width: min(300px, 60vw);
    }
}

/* For small tablets and phones with big screen */
@media (max-width: 768px) {
    .audio-player-container {
        padding: 15px;
        gap: 15px;
    }

    .album-art-container {
        width: min(280px, 70vw);
        height: min(280px, 70vw);
        margin-top: 10px;
    }

    .audio-metadata {
        max-width: min(280px, 70vw);
        margin-top: 10px;
        padding: 12px;
    }

    .audio-controls-container {
        margin-top: 10px;
    }
}

/* For small phones */
@media (max-width: 480px) and (orientation: landscape) {
    .audio-player-container {
        padding: 10px;
        gap: 10px;
        margin-top: 10px;
    }

    .album-art-container {
        width: min(220px, 65vw);
        height: min(220px, 65vw);
        margin-top: 10px;
    }

    .audio-metadata {
        max-width: min(220px, 65vw);
        margin-top: 8px;
        padding: 10px;
    }

    .audio-title {
        font-size: clamp(1.1rem, 5vw, 1.3rem);
    }

    .audio-artist,
    .audio-album {
        font-size: clamp(0.9rem, 4vw, 1rem);
    }
}

/* For small screens in landscape orientation (Like a phone) */
@media (max-height: 500px) and (orientation: landscape) {
    .audio-player-container {
        flex-direction: row;
        flex-wrap: wrap;
        justify-content: center;
        align-items: center;
        gap: 15px;
    }

    .album-art-container {
        width: min(150px, 30vh);
        height: min(150px, 30vh);
        margin-right: 10px;
        margin-top: 10px;
    }

    .audio-metadata {
        max-width: calc(100% - 180px);
        text-align: left;
        margin-top: 0;
        flex: 1;
    }

    .audio-controls-container {
        width: 100%;
        margin-top: 10px;
        order: 3;
    }
}

/* For ultra-wide screens. This need test, I'm not sure if will work correctly */
@media (min-width: 1600px) {
    .album-art-container {
        width: min(400px, 25vw);
        height: min(400px, 25vw);
    }

    .audio-metadata {
        max-width: min(400px, 25vw);
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

