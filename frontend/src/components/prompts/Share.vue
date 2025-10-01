<template>
  <div class="card-title">
    <h2>{{ $t("buttons.share") }}</h2>
  </div>
  <div class="card-content">
    <div aria-label="share-path" class="searchContext button"> {{ $t('search.path') }} {{ item.path }}</div>
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
              <template v-else>{{ $t("general.permanent") }}</template>
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
      <div class="settings-items" style="margin-top: 0.5em;">
        <!--
        <ToggleSwitch class="item" v-model="allowEdit" :name="'Allow modifications'" />
        <ToggleSwitch class="item" v-model="allowUpload" :name="'Allow uploading'" />
        -->
        <ToggleSwitch class="item" v-model="disableFileViewer" :name="$t('share.disableFileViewer')" />
        <ToggleSwitch
          class="item"
          v-model="quickDownload"
          :name="$t('profileSettings.showQuickDownload')"
          :description="$t('profileSettings.showQuickDownloadDescription')"
        />
        <ToggleSwitch class="item" v-model="disableAnonymous" :name="$t('share.disableAnonymous')" :description="$t('share.disableAnonymousDescription')" />
        <ToggleSwitch class="item" v-model="enableAllowedUsernames" :name="$t('share.enableAllowedUsernames')" :description="$t('share.enableAllowedUsernamesDescription')" />

        <ToggleSwitch v-if="onlyOfficeAvailable" class="item" v-model="enableOnlyOffice" :name="$t('share.enableOnlyOffice')" :description="$t('share.enableOnlyOfficeDescription')" />
        <ToggleSwitch v-if="onlyOfficeAvailable" class="item" v-model="enableOnlyOfficeEditing" :name="$t('share.enableOnlyOfficeEditing')" :description="$t('share.enableOnlyOfficeEditingDescription')" />
        <p>
          {{ $t("share.enforceDarkLightMode") }}
          <i
            class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.enforceDarkLightModeDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <select class="input" v-model="enforceDarkLightMode">
          <option value="default">{{ $t("share.default") }}</option>
          <option value="dark">{{ $t("share.dark") }}</option>
          <option value="light">{{ $t("share.light") }}</option>
        </select>

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
        <p>
          {{ $t("share.defaultViewMode") }}
          <i
            class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.defaultViewModeDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <select class="input" v-model="viewMode">
          <option value="normal">{{ $t("buttons.normalView") }}</option>
          <option value="list">{{ $t("buttons.listView") }}</option>
          <option value="compact">{{ $t("buttons.compactView") }}</option>
          <option value="gallery">{{ $t("buttons.galleryView") }}</option>
        </select>
      <SettingsItem :title="$t('buttons.showMore')" :collapsable="true" :start-collapsed="true">
        <div class="settings-items">
          <ToggleSwitch class="item" v-model="keepAfterExpiration" :name="$t('share.keepAfterExpiration')" :description="$t('share.keepAfterExpirationDescription')" />
          <ToggleSwitch class="item" v-model="disableThumbnails" :name="$t('share.disableThumbnails')" :description="$t('share.disableThumbnailsDescription')" />
          <ToggleSwitch class="item" v-model="disableNavButtons" :name="$t('share.hideNavButtons')" :description="$t('share.hideNavButtonsDescription')" />
          <ToggleSwitch class="item" v-model="disableShareCard" :name="$t('share.disableShareCard')" :description="$t('share.disableShareCardDescription')" />
          <ToggleSwitch class="item" v-model="disableSidebar" :name="$t('share.disableSidebar')" :description="$t('share.disableSidebarDescription')" />
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
      </SettingsItem>
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
import { publicApi, shareApi } from "@/api";
import Clipboard from "clipboard";
import { fromNow } from "@/utils/moment";
import { buildItemUrl, fixDownloadURL } from "@/utils/url";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import { globalVars } from "@/utils/constants";
import { eventBus } from "@/store/eventBus";
//import ViewMode from "@/components/settings/ViewMode.vue";

export default {
  name: "share",
  components: {
    ToggleSwitch,
    SettingsItem,
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
    item: {
      type: Object,
      default: () => ({
        path: "",
        source: "",
        isDir: false,
        type: "",
      }),
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
      password: "",
      listing: true,
      allowEdit: false,
      downloadsLimit: "",
      shareTheme: "default",
      disableAnonymous: false,
      allowUpload: false,
      maxBandwidth: "",
      disableFileViewer: false,
      disableThumbnails: false,
      enableAllowedUsernames: false,
      allowedUsernames: "",
      keepAfterExpiration: false,
      themeColor: "",
      banner: "",
      title: "",
      description: "",
      favicon: "",
      quickDownload: false,
      disableNavButtons: false,
      disableShareCard: false,
      disableSidebar: false,
      enforceDarkLightMode: "default",
      viewMode: "normal",
      enableOnlyOffice: false,
      enableOnlyOfficeEditing: false,
      //viewMode: "normal",
    };
  },
  computed: {
    onlyOfficeAvailable() {
      return globalVars.onlyOfficeUrl !== "";
    },
    availableThemes() {
      return globalVars.userSelectableThemes || {};
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
          this.disableFileViewer = this.link.disableFileViewer || false;
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
          this.disableShareCard = this.link.disableShareCard || false;
          this.disableSidebar = this.link.disableSidebar || false;
          this.enforceDarkLightMode = this.link.enforceDarkLightMode || "default";
          this.viewMode = this.link.viewMode || "normal";
          this.enableOnlyOffice = this.link.enableOnlyOffice || false;
          this.enableOnlyOfficeEditing = this.link.enableOnlyOfficeEditing || false;
          //this.viewMode = this.link.viewMode || "normal";
        }
      },
    },
  },
  async beforeMount() {
    if (this.isEditMode) {
      return;
    }
    try {
      // get last element of the path
      const links = await shareApi.get(this.item.path, this.item.source);
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
        if (!this.description) {
          this.description = this.$t("share.descriptionDefault");
        }
        if (!this.title) {
          this.title = this.$t("share.titleDefault", { title: this.item.name || "share" });
        }
        let isPermanent = !this.time || this.time === "0";
        const payload = {
          path: this.item.path,
          source: this.item.source,
          password: this.password,
          expires: isPermanent ? "" : this.time.toString(),
          unit: this.unit,
          disableAnonymous: this.disableAnonymous,
          allowUpload: this.allowUpload,
          maxBandwidth: this.maxBandwidth ? parseInt(this.maxBandwidth) : 0,
          downloadsLimit: this.downloadsLimit ? parseInt(this.downloadsLimit) : 0,
          shareTheme: this.shareTheme,
          disableFileViewer: this.disableFileViewer,
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
          disableShareCard: this.disableShareCard,
          disableSidebar: this.disableSidebar,
          enforceDarkLightMode: this.enforceDarkLightMode,
          viewMode: this.viewMode,
          enableOnlyOffice: this.enableOnlyOffice,
          enableOnlyOfficeEditing: this.enableOnlyOfficeEditing,
        };
        if (this.isEditMode) {
          payload.hash = this.link.hash;
        }

        const res = await shareApi.create(payload);

        if (!this.isEditMode) {
          this.links.push(res);
          this.sort();
        } else {
          // emit event to reload shares in settings view
          eventBus.emit('sharesChanged');
          mutations.closeHovers();
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
        await shareApi.remove(link.hash);
        this.links = this.links.filter((item) => item.hash !== link.hash);
        // emit event to reload shares in settings view
        eventBus.emit('sharesChanged');
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
      // Check if we have a single selected item that can be downloaded
      return !this.item?.isDir;
    },
    /**
     * @param {Share} share
     */
    buildDownloadLink(share) {
      if (share.downloadURL) {
        // Only fix the URL if it doesn't already have the correct external domain
        if (globalVars.externalUrl) {

          // URL already has the correct external domain, use as-is
          return share.downloadURL;
        }
        // URL needs fixing (internal domain or no externalUrl set)
        return this.fixDownloadURL(share.downloadURL);
      }
      return publicApi.getDownloadURL(share, [this.item.name]);
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
    fixDownloadURL(downloadUrl) {
      return fixDownloadURL(downloadUrl);
    },
  },
};
</script>

<style scoped>

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
</style>
