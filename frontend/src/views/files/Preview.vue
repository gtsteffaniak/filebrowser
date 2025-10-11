<template>
    <div id="previewer">
        <div class="preview" :class="{'plyr-background': previewType == 'audio' && !useDefaultMediaPlayer}" v-if="!isDeleted">
            <ExtendedImage v-if="showImage" :src="raw" @navigate-previous="navigatePrevious" @navigate-next="navigateNext"/>

            <!-- Media Player Component -->
            <plyrViewer v-else-if="previewType == 'audio' || previewType == 'video'"
                ref="plyrViewer"
                :previewType="previewType"
                :raw="raw"
                :subtitlesList="subtitlesList"
                :req="req"
                :useDefaultMediaPlayer="useDefaultMediaPlayer"
                :autoPlayEnabled="autoPlay"
                @play="autoPlay = true"
                :class="{'plyr-background': previewType == 'audio' && !useDefaultMediaPlayer}" />

            <div v-else-if="previewType == 'pdf'" class="pdf-wrapper">
                <iframe class="pdf" :src="raw"></iframe>
                <a v-if="isMobileSafari" :href="raw" target="_blank" class="button button--flat floating-btn">
                    <div>
                        <i class="material-icons">open_in_new</i>{{ $t("buttons.openFile") }}
                    </div>
                </a>
            </div>

            <div v-else class="info">
                <div class="title">
                    <i class="material-icons">feedback</i>
                    {{ $t("files.noPreview") }}
                </div>
                <div>
                    <a target="_blank" :href="downloadUrl" class="button button--flat">
                        <div>
                            <i class="material-icons">file_download</i>{{ $t("buttons.download") }}
                        </div>
                    </a>
                    <a target="_blank" :href="raw" class="button button--flat" v-if="req.type != 'directory'">
                        <div>
                            <i class="material-icons">open_in_new</i>{{ $t("buttons.openFile") }}
                        </div>
                    </a>
                </div>
                <p> {{ req.name }} </p>
            </div>
        </div>


    </div>
</template>
<script>
import { filesApi, publicApi } from "@/api";
import { url } from "@/utils";
import ExtendedImage from "@/components/files/ExtendedImage.vue";
import plyrViewer from "@/views/files/plyrViewer.vue";
import { state, getters, mutations } from "@/store";
import { getFileExtension } from "@/utils/files";
import { convertToVTT } from "@/utils/subtitles";
import { globalVars, shareInfo } from "@/utils/constants";

export default {
    name: "preview",
    components: {
        ExtendedImage,
        plyrViewer,
    },
    data() {
        return {
            listing: null,
            name: "",
            fullSize: true,
            currentPrompt: null, // Replaces Vuex getter `currentPrompt`
            subtitlesList: [],
            isDeleted: false,
            tapTimeout: null,
        };
    },
    computed: {
        showImage() {
            return this.previewType == 'image' || this.pdfConvertable || this.heicConvertable;
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
        heicConvertable() {
            return globalVars.enableHeicConversion && state.req.type == "image/heic";
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

            // Let the navigation component handle next/previous logic
            if (state.navigation.nextLink) {
                this.$router.replace({ path: state.navigation.nextLink });
            } else if (state.navigation.previousLink) {
                this.$router.replace({ path: state.navigation.previousLink });
            } else {
                this.close();
            }
            mutations.setDeletedItem(false);
        },
        req() {
            if (!getters.isLoggedIn()) {
                return;
            }
            this.isDeleted = false;
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
    },
    async mounted() {
        // Check for pre-fetched parent directory items from Files.vue
        if (state.req.parentDirItems) {
            this.listing = state.req.parentDirItems;
        } else if (state.req.items) {
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
        // Clear navigation state when leaving preview
        mutations.clearNavigation();
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
        async keyEvent(event) {
            if (getters.currentPromptName()) {
                return;
            }

            const { key } = event;

            switch (key) {
                case "Delete":
                    mutations.showHover("delete");
                    break;
                case "Escape":
                case "Backspace":
                    this.close();
                    break;
            }
        },
        async updatePreview() {
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

            // Setup navigation using the new state management
            mutations.setupNavigation({
                listing: this.listing,
                currentItem: state.req,
                directoryPath: directoryPath
            });
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
        close() {
            mutations.replaceRequest({}); // Reset request data
            let uri = url.removeLastDir(state.route.path) + "/";
            this.$router.push({ path: uri });
        },
        download() {
            const items = [state.req];
            downloadFiles(items);
        },
        navigatePrevious() {
            if (state.navigation.previousLink) {
                this.$router.replace({ path: state.navigation.previousLink });
            }
        },
        navigateNext() {
            if (state.navigation.nextLink) {
                this.$router.replace({ path: state.navigation.nextLink });
            }
        },
    },
};
</script>

<style scoped>
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
</style>
