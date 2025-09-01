<template>
    <div
        id="previewer"
        @mousemove="toggleNavigation"
        @touchstart="toggleNavigation"
    >
        <div class="preview" v-if="!isDeleted">
            <ExtendedImage
                v-if="previewType == 'image' || pdfConvertable"
                :src="raw"
                @navigate-previous="prev"
                @navigate-next="next"
            >
            </ExtendedImage>

            <!-- Audio with plyr -->
            <vue-plyr
                v-else-if="previewType == 'audio' && !useDefaultMediaPlayer"
                ref="audioPlayer"
                :options="plyrOptions"
            >
                <audio
                    :src="raw"
                    :autoplay="autoPlay"
                    @play="autoPlay = true"
                ></audio>
            </vue-plyr>

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
import { muPdfAvailable } from "@/utils/constants";
import { shareInfo } from "@/utils/constants";

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
                autoplay: false, // The users will manage this from their profile settings
                clickToPlay: true,
                resetOnEnd: true,
                toggleInvert: false,
            },
        };
    },
    computed: {
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
            if (!muPdfAvailable) {
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
            if (this.pdfConvertable) {
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
    },
    beforeUnmount() {
        window.removeEventListener("keydown", this.keyEvent);
        if (this.toastTimeout) {
            clearTimeout(this.toastTimeout);
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
                if (!subtitleTrack.content || subtitleTrack.content.length === 0) {
                    console.warn("Subtitle track has no content:", subtitleTrack.name);
                    continue;
                }
                let vttContent = subtitleTrack.content;
                if (!subtitleTrack.content.startsWith('WEBVTT')) {
                    const ext = getFileExtension(subtitleTrack.name);
                    vttContent = convertToVTT(ext, subtitleTrack.content);
                }
                if (vttContent.startsWith('WEBVTT')) {
                    // Create a virtual file (Blob) and get a URL for it
                    const blob = new Blob([vttContent], { type: "text/vtt" });
                    const vttURL = URL.createObjectURL(blob);
                    subs.push({
                        name: subtitleTrack.name,
                        src: vttURL,
                    });
                } else {
                    console.warn("Skipping subtitle track because it has no WEBVTT header:", subtitleTrack.name);
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
                this.listing = res.items
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
                    let composedListing = this.listing[j];
                    composedListing.path =
                        directoryPath + "/" + composedListing.name;
                    this.previousLink = url.buildItemUrl(
                        composedListing.source,
                        composedListing.path,
                    );
                    if (
                        getTypeInfo(composedListing.type).simpleType == "image"
                    ) {
                        this.previousRaw = this.prefetchUrl(composedListing);
                    }
                    break;
                }
                for (let j = i + 1; j < this.listing.length; j++) {
                    let composedListing = this.listing[j];
                    composedListing.path =
                        directoryPath + "/" + composedListing.name;
                    this.nextLink = url.buildItemUrl(
                        composedListing.source,
                        composedListing.path,
                    );
                    if (
                        getTypeInfo(composedListing.type).simpleType == "image"
                    ) {
                        this.nextRaw = this.prefetchUrl(composedListing);
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
    },
};
</script>

<style>
@import url("@skjnldsv/vue-plyr/dist/vue-plyr.css");
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
    --plyr-video-controls-background: linear-gradient(transparent,
            rgba(0, 0, 0, 0.7));
    border-radius: 12px;
    overflow: visible;
}

.plyr.plyr--video {
    width: 100%;
    height: 100%;
}

.plyr.plyr--video .plyr__control[data-plyr="captions"],
.plyr.plyr--video .plyr__control[data-plyr="pip"] {
    display: block !important;
}

.plyr .plyr__controls {
    display: flex;
    flex-direction: row;
    gap: 8px;
}

/* Progress bar with full width */
.plyr .plyr__progress__container {
    flex: 100%;
    margin: 0;
}

/* Buttons */
.plyr .plyr__controls__items {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: nowrap;
}

/* Button styling */
.plyr .plyr__control {
    transition: all 0.2s ease;
    flex-shrink: 0;
    display: flex;
    min-width: 2em;
    justify-content: center;
    align-items: center;
}

.plyr video {
    border-radius: 12px;
    width: 100%;
    height: 100%;
}

/* Style for audio player */
.plyr.plyr--audio {
    background: rgba(40, 40, 55, 1);
    border-radius: 16px;
    padding: 15px;
    max-width: 800px;
    width: 90%;
    max-height: 300px;
    margin: auto auto;
    position: absolute;
    bottom: 40px;
    left: 50%;
    transform: translateX(-50%);
}

.plyr--full-ui.plyr--video .plyr__control--overlaid {
    display: flex;
    justify-content: center;
    align-items: center;
}

.plyr__control--overlaid {
    /* background: #00b2ff; */
    background: var(--plyr-video-control-background-hover, var(--plyr-color-main, var(--plyr-color-main, #00b2ff)));
    border: 0;
    display: none;
    position: fixed;
    transition: .3s;
    z-index: 2;
    height: 4em;
    transform: none;
    padding: unset;
    left: unset;
    right: unset;
    bottom: unset;
    width: 4em !important;
    border-radius: 5em !important
}

/* Mobile */
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

    /* Made the buttons more "big" */
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
        gap: 15px;
    }

    .plyr--audio .plyr__control--play {
        transform: scale(1.2);
    }

    /* Hide some items on audio player*/
    .plyr--audio .plyr__control[data-plyr="settings"],
    .plyr--audio .plyr__control[data-plyr="pip"],
    .plyr--audio .plyr__volume {
        display: none;
    }

    /* Hide some items on video player*/
    .plyr--video .plyr__control[data-plyr="pip"],
    .plyr--video .plyr__volume {
        display: none;
    }

    /* Time playing */
    .plyr--audio .plyr__time {
        font-size: 14px;
        margin: 0 5px;
    }
}

/* Loop toast */
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