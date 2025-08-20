<template>
  <div class="card-title">
    <h2>{{ $t("buttons.share") }}</h2>
  </div>
  <div class="card-content">
    <div aria-label="share-path" class="searchContext"> {{ $t('search.path') }} {{ subpath }}</div>
    <p> {{ $t('share.notice') }} </p>

    <div v-if="listing">
      <table>
        <tbody>
          <tr>
            <th>#</th> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
            <th>{{ $t("settings.shareDuration") }}</th>
            <th></th>
            <th></th>
          </tr>

          <tr v-for="link in links" :key="link.hash">
            <td>{{ link.hash }}</td>
            <td>
              <template v-if="link.expire !== 0">{{ humanTime(link.expire) }}</template>
              <template v-else>{{ $t("permanent") }}</template>
            </td>
            <td class="small">
              <button class="action copy-clipboard" :data-clipboard-text="buildLink(link)"
                :aria-label="$t('buttons.copyToClipboard')" :title="$t('buttons.copyToClipboard')">
                <i class="material-icons">content_paste</i>
              </button>
            </td>
            <td class="small" v-if="hasDownloadLink()">
              <button class="action copy-clipboard" :data-clipboard-text="buildDownloadLink(link)"
                :aria-label="$t('buttons.copyDownloadLinkToClipboard')"
                :title="$t('buttons.copyDownloadLinkToClipboard')">
                <i class="material-icons">content_paste_go</i>
              </button>
            </td>
            <td class="small">
              <button class="action" @click="deleteLink($event, link)" :aria-label="$t('buttons.delete')"
                :title="$t('buttons.delete')">
                <i class="material-icons">delete</i>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else>
      <p>
        {{ $t("settings.shareDuration") }}
        <i
          class="no-select material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip($event, $t('share.shareDurationDescription'))"
          @mouseleave="hideTooltip"
        >
          help
        </i>
      </p>
      <div class="form-flex-group">
        <input class="form-grow input flat-right" v-focus type="number" max="2147483647" min="0" @keyup.enter="submit" v-model.trim="time" />
        <select class="flat-left input form-dropdown" v-model="unit" :aria-label="$t('time.unit')">
          <option value="minutes">{{ $t("time.minutes") }}</option>
          <option value="hours">{{ $t("time.hours") }}</option>
          <option value="days">{{ $t("time.days") }}</option>
        </select>
      </div>
      <p>
        {{ $t("prompts.optionalPassword") }}
        <i
          class="no-select material-symbols-outlined tooltip-info-icon"
          @mouseenter="showTooltip($event, $t('share.passwordDescription'))"
          @mouseleave="hideTooltip"
        >
          help
        </i>
      </p>
      <input class="input" type="password" autocomplete="new-password" v-model.trim="password" />

      <div class="settings-items">
        <!--
        <ToggleSwitch class="item" v-model="allowEdit" :name="'Allow modifications'" />
        <ToggleSwitch class="item" v-model="allowUpload" :name="'Allow uploading'" />
        <ToggleSwitch class="item" v-model="disablingFileViewer" :name="'Disable File Viewer'" />
        -->
        <ToggleSwitch
          class="item"
          v-model="quickDownload"
          :name="$t('profileSettings.showQuickDownload')"
          :description="$t('profileSettings.showQuickDownloadDescription')"
        />
        <ToggleSwitch class="item" v-model="disableAnonymous" :name="$t('share.disableAnonymous')" :description="$t('share.disableAnonymousDescription')" />
        <ToggleSwitch class="item" v-model="enableAllowedUsernames" :name="$t('share.enableAllowedUsernames')" :description="$t('share.enableAllowedUsernamesDescription')" />
        <div v-if="enableAllowedUsernames" class="item">
          <input class="input" type="text" v-model.trim="allowedUsernames" :placeholder="$t('share.allowedUsernamesPlaceholder')" />
        </div>
      </div>
        <!-- <ViewMode :viewMode="viewMode" @update:viewMode="viewMode = $event" /> -->
        <p>
          {{ $t("prompts.shareTheme") }}
          <i
            class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareThemeDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <div v-if="Object.keys(availableThemes).length > 0" class="form-flex-group">
          <select class="input" v-model="shareTheme">
            <option v-for="(theme, key) in availableThemes" :key="key" :value="key">
              {{ String(key) === "default" ? $t("profileSettings.defaultThemeDescription") : `${key} - ${theme.description}` }}
            </option>
          </select>
        </div>
      <div class="advanced-toggle">
        <button
          class="button button--flat button--block"
          @click="showAdvanced = !showAdvanced"
          :aria-expanded="showAdvanced ? 'true' : 'false'"
          aria-controls="advanced-settings"
          :aria-label="showAdvanced ? $t('buttons.showLess') : $t('buttons.showMore')"
          :title="showAdvanced ? $t('buttons.showLess') : $t('buttons.showMore')"
        >
          {{ showAdvanced ? $t('buttons.showLess') : $t('buttons.showMore') }}
        </button>
      </div>

      <transition
        name="expand"
        @before-enter="beforeEnter"
        @enter="enter"
        @leave="leave"
      >
      <div id="advanced-settings" v-show="showAdvanced">
        <div class="settings-items">
          <ToggleSwitch class="item" v-model="keepAfterExpiration" :name="$t('share.keepAfterExpiration')" :description="$t('share.keepAfterExpirationDescription')" />
          <ToggleSwitch class="item" v-model="disableThumbnails" :name="$t('share.disableThumbnails')" :description="$t('share.disableThumbnailsDescription')" />
          <ToggleSwitch class="item" v-model="disableNavButtons" :name="$t('share.hideNavButtons')" :description="$t('share.hideNavButtonsDescription')" />
        </div>

        <p>
          {{ $t("prompts.downloadsLimit") }}
          <i
            class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.downloadsLimitDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <input class="input" type="number" min="0" v-model.number="downloadsLimit" />
        <p>
          {{ $t("prompts.maxBandwidth") }}
          <i
            class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.maxBandwidthDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <input class="input" type="number" min="0" v-model.number="maxBandwidth" />


        <p>
          {{ $t("prompts.shareThemeColor") }}
          <i
            class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareThemeColorDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <input class="input" type="text" v-model.trim="themeColor" />

        <p>
          {{ $t("prompts.shareTitle") }}
          <i
            class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareTitleDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <input class="input" type="text" v-model.trim="title" />

        <p>
          {{ $t("prompts.shareDescription") }}
          <i
            class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareDescriptionHelp'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <textarea class="input" v-model.trim="description"></textarea>

        <p>
          {{ $t("prompts.shareBanner") }}
          <i
            class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareBannerDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <input class="input" type="text" v-model.trim="banner" />

        <p>
          {{ $t("prompts.shareFavicon") }}
          <i
            class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareFaviconDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <input class="input" type="text" v-model.trim="favicon" />
      </div>
      </transition>
    </div>
  </div>

  <div class="card-action">
    <button v-if="listing" class="button button--flat button--grey" @click="closeHovers" :aria-label="$t('buttons.close')"
      :title="$t('buttons.close')">
      {{ $t("buttons.close") }}
    </button>
    <button v-if="listing" class="button button--flat button--blue" @click="() => switchListing()" :aria-label="$t('buttons.new')"
      :title="$t('buttons.new')">
      {{ $t("buttons.new") }}
    </button>

    <button v-if="!listing" class="button button--flat button--grey" @click="() => switchListing()" :aria-label="$t('buttons.cancel')"
      :title="$t('buttons.cancel')">
      {{ $t("buttons.cancel") }}
    </button>
    <button v-if="!listing" class="button button--flat button--blue" @click="submit" aria-label="Share-Confirm"
      :title="$t('buttons.share')">
      {{ $t("buttons.share") }}
    </button>
  </div>
</template>
<script>
import { notify } from "@/notify";
import { state, getters, mutations } from "@/store";
import { publicApi } from "@/api";
import Clipboard from "clipboard";
import { fromNow } from "@/utils/moment";
import { buildItemUrl } from "@/utils/url";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import { userSelectableThemes } from "@/utils/constants";
//import ViewMode from "@/components/settings/ViewMode.vue";

/**
 * @typedef {import('@/api/public').Share} Share
 */

/**
 * @typedef {object} FilebrowserFile
 * @property {string} name
 * @property {string} path
 * @property {string} source
 * @property {boolean} isDir
 * @property {string} type
 */

/**
 * @typedef {object} FilebrowserRequest
 * @property {FilebrowserFile[]} items
 * @property {number} numDirs
 * @property {number} numFiles
 * @property {{by: string, asc: boolean}} sorting
 * @property {string} path
 * @property {string} source
 */

export default {
  name: "share",
  components: {
    ToggleSwitch,
    //ViewMode,
  },
  props: {
    editing: {
      type: Boolean,
      default: false,
    },
    link: {
      type: Object,
      default: () => ({}),
    },
  },
  data() {
    return {
      time: "",
      unit: "hours",
      /** @type {Share[]} */
      links: [],
      /** @type {Clipboard | null} */
      clip: null,
      subpath: "",
      source: "",
      password: "",
      listing: true,
      allowEdit: false,
      downloadsLimit: "",
      shareTheme: "default",
      disableAnonymous: false,
      allowUpload: false,
      maxBandwidth: "",
      disablingFileViewer: false,
      disableThumbnails: false,
      enableAllowedUsernames: false,
      allowedUsernames: "",
      keepAfterExpiration: false,
      themeColor: "",
      banner: "",
      title: "",
      description: "",
      favicon: "",
      showAdvanced: false,
      quickDownload: false,
      disableNavButtons: false,
      //viewMode: "normal",
    };
  },
  computed: {
    availableThemes() {
      return userSelectableThemes || {};
    },
    closeHovers() {
      return mutations.closeHovers;
    },
    req() {
      return /** @type {FilebrowserRequest} */ (/** @type {unknown} */ (state.req));
    },
    selected() {
      return /** @type {(number | FilebrowserFile)[]} */ (state.selected);
    },
    selectedCount() {
      return state.selected.length; // Compute selectedCount directly from state
    },
    isListing() {
      return getters.isListing(); // Access getter directly from the store
    },
    url() {
      if (state.isSearchActive) {
        const file = /** @type {FilebrowserFile} */ (this.selected[0]);
        return buildItemUrl(file.source, file.path);
      }
      if (!this.isListing) {
        return this.req.path;
      }
      if (this.selectedCount !== 1) {
        // selecting current view image
        return this.req.path;
      }
      const index = /** @type {number} */ (this.selected[0]);
      return buildItemUrl(this.req.items[index].source, this.req.items[index].path);
    },
    isEditMode() {
      return this.editing && this.link && Object.keys(this.link).length > 0;
    }
  },
  watch: {
    listing(isListing) {
      if (!isListing) {
        this.password = "";
      }
    },
    isEditMode: {
      immediate: true,
      handler(isEditMode) {
        if (isEditMode) {
          this.listing = false;
          this.time = this.link.expire
            ? String(Math.round((new Date(this.link.expire * 1000).getTime() - new Date().getTime()) / 3600000))
            : "0";
          this.unit = "hours";
          this.password = "";
          this.downloadsLimit = this.link.downloadsLimit ? String(this.link.downloadsLimit) : "";
          this.maxBandwidth = this.link.maxBandwidth ? String(this.link.maxBandwidth) : "";
          this.shareTheme = this.link.shareTheme || "default";
          this.disableAnonymous = this.link.disableAnonymous || false;
          this.disableThumbnails = this.link.disableThumbnails || false;
          this.enableAllowedUsernames = Array.isArray(this.link.allowedUsernames) && this.link.allowedUsernames.length > 0;
          this.allowedUsernames = this.enableAllowedUsernames ? this.link.allowedUsernames.join(", ") : "";
          this.keepAfterExpiration = this.link.keepAfterExpiration || false;
          this.themeColor = this.link.themeColor || "";
          this.banner = this.link.banner || "";
          this.title = this.link.title || "";
          this.description = this.link.description || "";
          this.favicon = this.link.favicon || "";
          this.quickDownload = this.link.quickDownload || false;
          this.disableNavButtons = this.link.hideNavButtons || false;
          this.viewMode = this.link.viewMode || "normal";
        }
      },
    },
  },
  async beforeMount() {
    if (this.isEditMode) {
      this.subpath = this.link.path;
      this.source = this.link.source;
      return;
    }
    let path = this.req.path;
    this.source = this.req.source;
    if (state.isSearchActive) {
      const file = /** @type {FilebrowserFile} */ (this.selected[0]);
      path = file.path;
      this.source = file.source;
    } else if (this.selectedCount === 1) {
      const index = /** @type {number} */ (this.selected[0]);
      const selected = this.req.items[index];
      path = selected.path;
      this.source = selected.source;
    }
    // double encode # to fix issue with # in path
    // replace all # with %23
    this.subpath = path.replace(/#/g, "%23");
    try {
      // get last element of the path
      const links = await publicApi.get(this.subpath, this.source);
      this.links = links;
    } catch (err) {
      notify.showError(err);
      return;
    }
    this.sort();

    if (this.links.length === 0) {
      this.listing = false;
    }
  },
  mounted() {
    this.clip = new Clipboard(".copy-clipboard");
    this.clip.on("success", () => {
      notify.showSuccess(this.$t("success.linkCopied"));
    });
  },
  methods: {
    /**
     * @param {Element} el
     */
    beforeEnter(el) {
      const element = /** @type {HTMLElement} */ (el);
      element.style.height = '0';
      element.style.opacity = '0';
    },
    /**
     * @param {Element} el
     * @param {() => void} done
     */
    enter(el, done) {
      const element = /** @type {HTMLElement} */ (el);
      element.style.transition = '';
      element.style.height = '0';
      element.style.opacity = '0';
      void element.offsetHeight;
      element.style.transition = 'height 0.3s, opacity 0.3s';
      element.style.height = element.scrollHeight + 'px';
      element.style.opacity = '1';
      setTimeout(() => {
        element.style.height = 'auto';
        done();
      }, 300);
    },
    /**
     * @param {Element} el
     * @param {() => void} done
     */
    leave(el, done) {
      const element = /** @type {HTMLElement} */ (el);
      element.style.transition = 'height 0.3s, opacity 0.3s';
      element.style.height = element.scrollHeight + 'px';
      void element.offsetHeight;
      element.style.height = '0';
      element.style.opacity = '0';
      setTimeout(done, 300);
    },
    /**
     * @param {MouseEvent} event
     * @param {string} text
     */
    showTooltip(event, text) {
      mutations.showTooltip({
        content: text,
        x: event.clientX,
        y: event.clientY,
      });
    },
    hideTooltip() {
      mutations.hideTooltip();
    },
    async submit() {
      try {
        let isPermanent = !this.time || this.time === "0";
        const payload = {
          path: this.subpath,
          sourceName: this.source,
          source: this.source,
          password: this.password,
          expires: isPermanent ? "" : this.time.toString(),
          unit: this.unit,
          disableAnonymous: this.disableAnonymous,
          allowUpload: this.allowUpload,
          maxBandwidth: this.maxBandwidth,
          downloadsLimit: this.downloadsLimit,
          shareTheme: this.shareTheme,
          disablingFileViewer: this.disablingFileViewer,
          disableThumbnails: this.disableThumbnails,
          allowedUsernames: this.enableAllowedUsernames ? this.allowedUsernames.split(',').map(u => u.trim()) : [],
          keepAfterExpiration: this.keepAfterExpiration,
          hash: '',
          themeColor: this.themeColor,
          banner: this.banner,
          title: this.title,
          description: this.description,
          favicon: this.favicon,
          quickDownload: this.quickDownload,
          hideNavButtons: this.disableNavButtons,
        };
        if (this.isEditMode) {
          payload.hash = this.link.hash;
        }

        const res = await publicApi.create(payload.path, payload.source, {
          password: payload.password,
          expires: payload.expires,
          unit: payload.unit,
          disableAnonymous: payload.disableAnonymous,
          allowUpload: payload.allowUpload,
          maxBandwidth: Number(payload.maxBandwidth) || 0,
          downloadsLimit: Number(payload.downloadsLimit) || 0,
          shareTheme: payload.shareTheme,
          disableFileViewer: payload.disablingFileViewer,
          disableThumbnails: payload.disableThumbnails,
          allowedUsernames: payload.allowedUsernames,
          hash: payload.hash,
          keepAfterExpiration: payload.keepAfterExpiration,
          themeColor: payload.themeColor,
          banner: payload.banner,
          title: payload.title,
          description: payload.description,
          favicon: payload.favicon,
          quickDownload: payload.quickDownload,
          hideNavButtons: payload.hideNavButtons,
          //viewMode: this.viewMode,
        });

        if (!this.isEditMode) {
          this.links.push(res);
          this.sort();
        } else {
          // reload page to see changes
          window.location.reload();
        }

        this.time = "";
        this.unit = "hours";
        this.password = "";

        this.listing = true;
      } catch (err) {
        notify.showError(err);
      }
    },
    /**
     * @param {Event} event
     * @param {Share} link
     */
    async deleteLink(event, link) {
      event.preventDefault();
      try {
        await publicApi.remove(link.hash);
        this.links = this.links.filter((item) => item.hash !== link.hash);
        if (this.links.length === 0) {
          this.listing = false;
        }
      } catch (err) {
        notify.showError(err);
      }
    },
    /**
     * @param {number} time
     */
    humanTime(time) {
      return fromNow(time, state.user.locale)
    },
    /**
     * @param {Share} share
     */
    buildLink(share) {
      return publicApi.getShareURL(share);
    },
    hasDownloadLink() {
      if (state.isSearchActive) {
        const file = /** @type {FilebrowserFile} */ (this.selected[0]);
        return file.type !== "directory";
      }
      const index = /** @type {number} */ (this.selected[0]);
      return this.selected.length === 1 && !this.req.items[index].isDir;
    },
    /**
     * @param {Share} share
     */
    buildDownloadLink(share) {
      share.source = this.source;
      share.path = "/";
      const index = /** @type {number} */ (this.selected[0]);
      return publicApi.getDownloadURL(share, [this.req.items[index].name]);
    },
    sort() {
      this.links = this.links.sort((a, b) => {
        if (a.expire === 0) return -1;
        if (b.expire === 0) return 1;
        return a.expire - b.expire;
      });
    },
    switchListing() {
      if (this.links.length === 0 && !this.listing) {
        // Access the store directly if needed
        mutations.closeHovers();
      }

      this.listing = !this.listing;
    },
  },
};
</script>

<style scoped>

.advanced-toggle {
  margin-top: 1em;
}
.setting-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.description {
  font-size: 0.9em;
  color: #666;
  margin-top: 4px;
  margin-bottom: 8px;
}

/* Prevent inputs from expanding to container height during expand transition */
.input {
  height: auto;
}

.expand-enter-active,
.expand-leave-active {
  transition: height 0.3s cubic-bezier(0.4, 0, 0.2, 1), opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  overflow: hidden;
}
.expand-enter,
.expand-leave-to {
  height: 0 !important;
  opacity: 0;
}
</style>
