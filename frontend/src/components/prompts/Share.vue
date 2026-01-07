<template>
  <div class="card-title">
    <h2>{{ $t("general.share") }}</h2>
  </div>
  <div class="card-content">
    <!-- Warning banner for missing path when editing a share -->
    <div v-if="!pathExists && isEditMode && !isEditingPath" class="warning-banner">
      <i class="material-icons">warning</i>
      <span>{{ $t("messages.pathNotFoundMessage") }}</span>
      <button class="button button--flat button--blue" @click="startPathReassignment">
        {{ $t("messages.reassignPath") }}
      </button>
    </div>

    <div v-if="isEditingPath">
      <file-list @update:selected="updateTempPath" :browse-source="displaySource"></file-list>
      <div class="card-action">
        <button class="button button--flat" @click="cancelPathChange" :aria-label="$t('general.cancel')"
          :title="$t('general.cancel')">
          {{ $t("general.cancel") }}
        </button>
        <button class="button button--flat button--blue" @click="confirmPathChange" :aria-label="$t('general.ok')"
          :title="$t('general.ok')">
          {{ $t("general.ok") }}
        </button>
      </div>
    </div>

    <div v-else>
      <div aria-label="share-path" class="searchContext button"> {{ $t('general.path', { suffix: ':' }) }} {{
        displayPath }}</div>
      <p> {{ $t('share.notice') }} </p>

      <div v-if="listing">
        <table>
          <tbody>
            <tr>
              <th>#</th> <!-- eslint-disable-line @intlify/vue-i18n/no-raw-text -->
              <th>{{ $t("time.unit") }}</th>
              <th></th>
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
                <button class="action" @click="editLink(link)" :aria-label="$t('general.edit')"
                  :title="$t('general.edit')">
                  <i class="material-icons">edit</i>
                </button>
              </td>
              <td class="small">
                <button class="action copy-clipboard" :data-clipboard-text="link.shareURL"
                  :aria-label="$t('buttons.copyToClipboard')" :title="$t('buttons.copyToClipboard')">
                  <i class="material-icons">content_paste</i>
                </button>
              </td>
              <td class="small">
                <button :disabled="link.shareType == 'upload'" class="action copy-clipboard"
                  :data-clipboard-text="link.downloadURL" :aria-label="$t('buttons.copyDownloadLinkToClipboard')"
                  :title="$t('buttons.copyDownloadLinkToClipboard')">
                  <i class="material-icons">content_paste_go</i>
                </button>
              </td>
              <td class="small">
                <button class="action" @click="deleteLink($event, link)" :aria-label="$t('general.delete')"
                  :title="$t('general.delete')">
                  <i class="material-icons">delete</i>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else>
        <p>
          {{ $t("files.duration") }}
          <i class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareDurationDescription'))" @mouseleave="hideTooltip">
            help
          </i>
        </p>
        <div class="form-flex-group">
          <input class="form-grow input flat-right" v-focus type="number" max="2147483647" min="0" @keyup.enter="submit"
            v-model.trim="time" />
          <select class="flat-left input form-dropdown" v-model="unit" :aria-label="$t('time.unit')">
            <option value="minutes">{{ $t("time.minutes") }}</option>
            <option value="hours">{{ $t("time.hours") }}</option>
            <option value="days">{{ $t("time.days") }}</option>
          </select>
        </div>
        <p>
          {{ $t("prompts.optionalPassword") }}
          <i class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.passwordDescription'))" @mouseleave="hideTooltip">
            help
          </i>
        </p>
        <div v-if="hasExistingPassword && !isChangingPassword" class="password-change-section">
          <button class="button button--flat button--blue" @click="isChangingPassword = true" style="width: 100%;">
            <i class="material-icons">lock_reset</i>
            {{ $t("general.change") }}
          </button>
        </div>
        <input v-else class="input" type="password" autocomplete="new-password" v-model.trim="password" />
        <p>
          {{ $t("share.shareType") }}
          <i class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareTypeDescription'))" @mouseleave="hideTooltip">
            help
          </i>
        </p>
        <select class="input" v-model="shareType">
          <option value="normal">{{ $t("share.normalShare") }}</option>
          <option value="upload">{{ $t("share.uploadShare") }}</option>
        </select>
        <button @click="openSidebarLinksCustomization" class="button button--flat customize-sidebar-links-button">
          <i class="material-icons">link</i>
          {{ $t('share.customizeSidebarLinksButton') }}
        </button>
        <div class="settings-items" style="margin-top: 0.5em;">
          <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="disableDownload"
            :name="$t('share.disableDownload')" :description="$t('share.disableDownloadDescription')"
            aria-label="disable downloading files toggle" />
          <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="allowModify" :name="$t('share.allowModify')"
            :description="$t('share.allowModifyDescription')" aria-label="allow editing files toggle" />
          <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="allowDelete" :name="$t('share.allowDelete')"
            :description="$t('share.allowDeleteDescription')" aria-label="allow deleting files toggle" />
          <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="allowCreate" :name="$t('share.allowCreate')"
            :description="$t('share.allowCreateDescription')"
            aria-label="allow creating and uploading files and folders toggle" />
          <ToggleSwitch v-if="createAllowed" class="item" v-model="allowReplacements"
            :name="$t('share.allowReplacements')" :description="$t('share.allowReplacementsDescription')" />
          <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="disableFileViewer"
            :name="$t('share.disableFileViewer')" />
          <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="quickDownload"
            :name="$t('profileSettings.showQuickDownload')"
            :description="$t('profileSettings.showQuickDownloadDescription')" />
          <ToggleSwitch class="item" v-model="disableAnonymous" :name="$t('share.disableAnonymous')"
            :description="$t('share.disableAnonymousDescription')" />
          <ToggleSwitch class="item" v-model="enableAllowedUsernames" :name="$t('share.enableAllowedUsernames')"
            :description="$t('share.enableAllowedUsernamesDescription')" />

          <div v-if="enableAllowedUsernames" class="item">
            <input class="input" type="text" v-model.trim="allowedUsernames"
              :placeholder="$t('share.allowedUsernamesPlaceholder')" />
          </div>
          <ToggleSwitch v-if="shareType === 'normal' && onlyOfficeAvailable" class="item" v-model="enableOnlyOffice"
            :name="$t('share.enableOnlyOffice')" :description="$t('share.enableOnlyOfficeDescription')" />
          <p>
            {{ $t("share.enforceDarkLightMode") }}
            <i class="no-select material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.enforceDarkLightModeDescription'))" @mouseleave="hideTooltip">
              help
            </i>
          </p>
          <select class="input" v-model="enforceDarkLightMode">
            <option value="default">{{ $t("share.default") }}</option>
            <option value="dark">{{ $t("share.dark") }}</option>
            <option value="light">{{ $t("share.light") }}</option>
          </select>
        </div>
        <!-- <ViewMode :viewMode="viewMode" @update:viewMode="viewMode = $event" /> -->
        <p>
          {{ $t("prompts.shareTheme") }}
          <i class="no-select material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareThemeDescription'))" @mouseleave="hideTooltip">
            help
          </i>
        </p>
        <div v-if="Object.keys(availableThemes).length > 0" class="form-flex-group">
          <select class="input" v-model="shareTheme">
            <option v-for="(theme, key) in availableThemes" :key="key" :value="key">
              {{ String(key) === "default" ? $t("profileSettings.defaultThemeDescription") : `${key} -
              ${theme.description}`
              }}
            </option>
          </select>
        </div>
        <div v-if="shareType === 'normal'">
          <p>
            {{ $t("share.defaultViewMode") }}
            <i class="no-select material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.defaultViewModeDescription'))" @mouseleave="hideTooltip">
              help
            </i>
          </p>
          <select class="input" v-model="viewMode">
            <option value="normal">{{ $t("buttons.normalView") }}</option>
            <option value="list">{{ $t("buttons.listView") }}</option>
            <option value="gallery">{{ $t("buttons.galleryView") }}</option>
          </select>
        </div>
        <SettingsItem :title="$t('buttons.showMore')" :collapsable="true" :start-collapsed="true">
          <div class="settings-items">
            <ToggleSwitch class="item" v-model="keepAfterExpiration" :name="$t('share.keepAfterExpiration')"
              :description="$t('share.keepAfterExpirationDescription')" />
            <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="disableThumbnails"
              :name="$t('share.disableThumbnails')" :description="$t('share.disableThumbnailsDescription')" />
            <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="showHidden"
              :name="$t('profileSettings.showHiddenFiles')" :description="$t('profileSettings.showHiddenFilesDescription')" />
            <ToggleSwitch class="item" v-model="disableNavButtons" :name="$t('share.hideNavButtons')"
              :description="$t('share.hideNavButtonsDescription')" />
            <ToggleSwitch class="item" v-model="disableShareCard" :name="$t('share.disableShareCard')"
              :description="$t('share.disableShareCardDescription')" />
            <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="disableSidebar"
              :name="$t('share.disableSidebar')" :description="$t('share.disableSidebarDescription')" />
            <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="perUserDownloadLimit"
              :name="$t('share.perUserDownloadLimit')" :description="$t('share.perUserDownloadLimitDescription')" />
            <ToggleSwitch v-if="shareType === 'normal'" class="item" v-model="extractEmbeddedSubtitles"
              :name="$t('share.extractEmbeddedSubtitles')"
              :description="$t('share.extractEmbeddedSubtitlesDescription')" />
            <ToggleSwitch class="item" v-model="disableOGMetadata" :name="$t('share.disableOGMetadata')"
              :description="$t('share.disableOGMetadataDescription')" />
          </div>

          <div v-if="shareType === 'normal'">
            <p>
              {{ $t("prompts.downloadsLimit") }}
              <i class="no-select material-symbols-outlined tooltip-info-icon"
                @mouseenter="showTooltip($event, $t('share.downloadsLimitDescription'))" @mouseleave="hideTooltip">
                help
              </i>
            </p>
            <input class="input" type="number" min="0" v-model.number="downloadsLimit" />
            <p>
              {{ $t("prompts.maxBandwidth") }}
              <i class="no-select material-symbols-outlined tooltip-info-icon"
                @mouseenter="showTooltip($event, $t('share.maxBandwidthDescription'))" @mouseleave="hideTooltip">
                help
              </i>
            </p>
            <input class="input" type="number" min="0" v-model.number="maxBandwidth" />
          </div>


          <p>
            {{ $t("prompts.shareThemeColor") }}
            <i class="no-select material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.shareThemeColorDescription'))" @mouseleave="hideTooltip">
              help
            </i>
          </p>
          <input class="input" type="text" v-model.trim="themeColor" />

          <p>
            {{ $t("prompts.shareTitle") }}
            <i class="no-select material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.shareTitleDescription'))" @mouseleave="hideTooltip">
              help
            </i>
          </p>
          <input class="input" type="text" v-model.trim="title" />

          <p>
            {{ $t("prompts.shareDescription") }}
            <i class="no-select material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.shareDescriptionHelp'))" @mouseleave="hideTooltip">
              help
            </i>
          </p>
          <textarea class="input" v-model.trim="description"></textarea>

          <p>
            {{ $t("prompts.shareBanner") }}
            <i class="no-select material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.shareBannerDescription'))" @mouseleave="hideTooltip">
              help
            </i>
          </p>
          <input class="input" type="text" v-model.trim="banner" />

          <p>
            {{ $t("prompts.shareFavicon") }}
            <i class="no-select material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.shareFaviconDescription'))" @mouseleave="hideTooltip">
              help
            </i>
          </p>
          <input class="input" type="text" v-model.trim="favicon" />
        </SettingsItem>
      </div>
    </div>
  </div>

  <div v-if="!isEditingPath" class="card-action">
    <button v-if="listing" class="button button--flat button--grey" @click="closeHovers"
      :aria-label="$t('general.close')" :title="$t('general.close')">
      {{ $t("general.close") }}
    </button>
    <button v-if="listing" class="button button--flat button--blue" @click="() => switchListing()"
      :aria-label="$t('general.new')" :title="$t('general.new')">
      {{ $t("general.new") }}
    </button>

    <button v-if="!listing" class="button button--flat button--grey" @click="() => switchListing()"
      :aria-label="$t('general.cancel')" :title="$t('general.cancel')">
      {{ $t("general.cancel") }}
    </button>
    <button v-if="!listing" class="button button--flat button--blue" @click="submit" aria-label="Share-Confirm"
      :title="$t('general.share')">
      {{ $t("general.share") }}
    </button>
  </div>
</template>
<script>
import { notify } from "@/notify";
import { state, getters, mutations } from "@/store";
import { shareApi } from "@/api";
import Clipboard from "clipboard";
import { fromNow } from "@/utils/moment";
import { buildItemUrl } from "@/utils/url";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import FileList from "./FileList.vue";
import { globalVars } from "@/utils/constants";
import { eventBus } from "@/store/eventBus";
//import ViewMode from "@/components/settings/ViewMode.vue";

export default {
  name: "share",
  components: {
    ToggleSwitch,
    SettingsItem,
    FileList,
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
      allowModify: false,
      disableDownload: false,
      allowDelete: false,
      allowCreate: false,
      allowReplacements: false,
      downloadsLimit: "",
      perUserDownloadLimit: false,
      shareTheme: "default",
      disableAnonymous: false,
      maxBandwidth: "",
      disableFileViewer: false,
      disableThumbnails: false,
      showHidden: false,
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
      shareType: "normal",
      extractEmbeddedSubtitles: false,
      disableOGMetadata: false,
      sidebarLinks: [],
      /** @type {Share | null} */
      editingLink: null,
      isEditingPath: false,
      isReassigningPath: false,
      tempPath: "",
      tempSource: "",
      pathExists: true,
      isChangingPassword: false,
      //viewMode: "normal",
    };
  },
  computed: {
    createAllowed() {
      return this.allowCreate;
    },
    displayPath() {
      // When editing, use the link's path; otherwise use the item's path
      return this.isEditMode ? this.link.path : this.item.path;
    },
    displaySource() {
      // When editing, use the link's source; otherwise use the item's source
      return this.isEditMode ? this.link.source : this.item.source;
    },
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
    },
    hasExistingPassword() {
      // Check if we're editing a link and it has a password
      const currentLink = this.isEditMode ? this.link : this.editingLink;
      return currentLink && currentLink.hasPassword;
    }
  },
  watch: {
    listing(isListing) {
      if (!isListing) {
        this.password = "";
        this.isChangingPassword = false;
      }
    },
    isEditMode: {
      immediate: true,
      handler(isEditMode) {
        if (isEditMode) {
          this.listing = false;
          // Check if path exists
          this.pathExists = this.link.pathExists !== false;
          this.time = this.link.expire
            ? String(Math.round((new Date(this.link.expire * 1000).getTime() - new Date().getTime()) / 3600000))
            : "0";
          this.unit = "hours";
          this.password = "";
          this.isChangingPassword = false;
          this.disableDownload = this.link.disableDownload || false;
          this.allowModify = this.link.allowModify || false;
          this.allowDelete = this.link.allowDelete || false;
          this.allowCreate = this.link.allowCreate || false;
          this.allowReplacements = this.link.allowReplacements || false;
          this.downloadsLimit = this.link.downloadsLimit ? String(this.link.downloadsLimit) : "";
          this.perUserDownloadLimit = this.link.perUserDownloadLimit || false;
          this.maxBandwidth = this.link.maxBandwidth ? String(this.link.maxBandwidth) : "";
          this.shareTheme = this.link.shareTheme || "default";
          this.disableAnonymous = this.link.disableAnonymous || false;
          this.disableThumbnails = this.link.disableThumbnails || false;
          this.disableFileViewer = this.link.disableFileViewer || false;
          this.showHidden = this.link.showHidden || false;
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
          this.shareType = this.link.shareType || "normal";
          this.extractEmbeddedSubtitles = this.link.extractEmbeddedSubtitles || false;
          this.disableOGMetadata = this.link.disableOGMetadata || false;
          this.sidebarLinks = Array.isArray(this.link.sidebarLinks) ? [...this.link.sidebarLinks] : [];
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
      console.error(err);
      return;
    }
    this.sort();

    if (this.links.length === 0) {
      this.listing = false;
      // Set default sidebar links for new shares
      this.setDefaultSidebarLinks();
      this.populateDefaults();
    }
  },
  mounted() {
    this.initClipboard();
    // Listen for sidebar links updates from the SidebarLinks prompt
    eventBus.on('shareSidebarLinksUpdated', this.handleSidebarLinksUpdate);
  },
  beforeUnmount() {
    // Clean up event listeners
    eventBus.off('apiKeysChanged', this.reloadApiKeys);
    eventBus.off('shareSidebarLinksUpdated', this.handleSidebarLinksUpdate);
    // Clean up clipboard
    if (this.clip) {
      this.clip.destroy();
    }
  },
  methods: {
    initClipboard() {
      // Destroy existing clipboard first
      if (this.clip) {
        this.clip.destroy();
      }

      // Create new clipboard instance
      this.clip = new Clipboard(".copy-clipboard");
      this.clip.on("success", () => {
        notify.showSuccessToast(this.$t("success.linkCopied"));
      });
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
        if (!this.description) {
          if (this.shareType === 'upload') {
            this.description = this.$t("share.descriptionUploadDefault");
          } else {
            this.description = this.$t("share.descriptionDefault");
          }
        }
        if (!this.title) {
          this.title = this.$t("share.titleDefault", { title: this.item.name || "share" });
        }
        let isPermanent = !this.time || this.time === "0";
        const payload = {
          path: this.displayPath,
          source: this.displaySource,
          expires: isPermanent ? "" : this.time.toString(),
          unit: this.unit,
          disableAnonymous: this.disableAnonymous,
          disableDownload: this.disableDownload,
          allowModify: this.allowModify,
          allowDelete: this.allowDelete,
          allowCreate: this.allowCreate,
          allowReplacements: this.allowReplacements,
          maxBandwidth: this.maxBandwidth ? parseInt(this.maxBandwidth) : 0,
          downloadsLimit: this.downloadsLimit ? parseInt(this.downloadsLimit) : 0,
          perUserDownloadLimit: this.perUserDownloadLimit,
          shareTheme: this.shareTheme,
          disableFileViewer: this.disableFileViewer,
          disableThumbnails: this.disableThumbnails,
          showHidden: this.showHidden,
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
          shareType: this.shareType,
          extractEmbeddedSubtitles: this.extractEmbeddedSubtitles,
          disableOGMetadata: this.disableOGMetadata,
          sidebarLinks: this.sidebarLinks,
        };

        // Handle password inclusion logic:
        // - Always include for new shares
        // - For editing: only include if no existing password OR explicitly changing it
        const isEditing = this.isEditMode || this.editingLink;
        if (!isEditing || !this.hasExistingPassword || this.isChangingPassword) {
          payload.password = this.password;
        }

        if (this.isEditMode) {
          payload.hash = this.link.hash;
        } else if (this.editingLink) {
          payload.hash = this.editingLink.hash;
        }

        const res = await shareApi.create(payload);

        if (!this.isEditMode && !this.editingLink) {
          this.links.push(res);
          this.sort();
        // reinitialize the clipboard after adding a new link
        this.$nextTick(() => {
          this.initClipboard();
        });
        } else if (this.editingLink) {
          // Update the link in the local list
          const index = this.links.findIndex(l => l.hash === this.editingLink.hash);
          if (index !== -1) {
            this.links[index] = res;
          }
          this.editingLink = null;
          // emit event to reload shares in settings view
          eventBus.emit('sharesChanged');
        // Reinitialize clipboard after edit the share
        this.$nextTick(() => {
          this.initClipboard();
        });
        } else {
          // emit event to reload shares in settings view
          eventBus.emit('sharesChanged');
          mutations.closeHovers();
        }

        this.time = "";
        this.unit = "hours";
        this.password = "";
        this.isChangingPassword = false;

        this.listing = true;
      } catch (err) {
        if (!err.message) {
          // didn't come from api, show error to user
          notify.showError(err);
        }
      }
    },
    /**
     * @param {Share} link
     */
    editLink(link) {
      this.listing = false;
      this.time = link.expire
        ? String(Math.round((new Date(link.expire * 1000).getTime() - new Date().getTime()) / 3600000))
        : "0";
      this.unit = "hours";
      this.password = "";
      this.isChangingPassword = false;
      this.disableDownload = link.disableDownload || false;
      this.allowModify = link.allowModify || false;
      this.allowDelete = link.allowDelete || false;
      this.allowCreate = link.allowCreate || false;
      this.allowReplacements = link.allowReplacements || false;
      this.downloadsLimit = link.downloadsLimit ? String(link.downloadsLimit) : "";
      this.perUserDownloadLimit = link.perUserDownloadLimit || false;
      this.maxBandwidth = link.maxBandwidth ? String(link.maxBandwidth) : "";
      this.shareTheme = link.shareTheme || "default";
      this.disableAnonymous = link.disableAnonymous || false;
      this.disableThumbnails = link.disableThumbnails || false;
      this.disableFileViewer = link.disableFileViewer || false;
      this.showHidden = link.showHidden || false;
      this.enableAllowedUsernames = Array.isArray(link.allowedUsernames) && link.allowedUsernames.length > 0;
      this.allowedUsernames = this.enableAllowedUsernames ? link.allowedUsernames.join(", ") : "";
      this.keepAfterExpiration = link.keepAfterExpiration || false;
      this.themeColor = link.themeColor || "";
      this.banner = link.banner || "";
      this.title = link.title || "";
      this.description = link.description || "";
      this.favicon = link.favicon || "";
      this.quickDownload = link.quickDownload || false;
      this.disableNavButtons = link.hideNavButtons || false;
      this.disableShareCard = link.disableShareCard || false;
      this.disableSidebar = link.disableSidebar || false;
      this.enforceDarkLightMode = link.enforceDarkLightMode || "default";
      this.viewMode = link.viewMode || "normal";
      this.enableOnlyOffice = link.enableOnlyOffice || false;
      this.shareType = link.shareType || "normal";
      this.extractEmbeddedSubtitles = link.extractEmbeddedSubtitles || false;
      this.disableOGMetadata = link.disableOGMetadata || false;
      this.sidebarLinks = Array.isArray(link.sidebarLinks) ? [...link.sidebarLinks] : [];
      // Store the link being edited
      this.editingLink = link;
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
        // Reinitialize clipboard after deletion too
        this.$nextTick(() => {
          this.initClipboard();
        });
        // emit event to reload shares in settings view
        eventBus.emit('sharesChanged');
        if (this.links.length === 0) {
          this.listing = false;
        }
      } catch (err) {
        console.error(err);
      }
    },
    /**
     * @param {number} time
     */
    humanTime(time) {
      return fromNow(time, state.user.locale)
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

      // Clear editing link when switching back to listing
      if (this.listing) {
        this.editingLink = null;
      } else {
        // Clear editing link when switching to create new share
        this.editingLink = null;
        this.isChangingPassword = false;
        // Set default sidebar links for new shares
        this.setDefaultSidebarLinks();
        this.populateDefaults();
      }
    },
    setDefaultSidebarLinks() {
      // Only set defaults if creating a new share (not editing) and no links are configured
      if (!this.isEditMode && !this.editingLink && this.sidebarLinks.length === 0) {
        this.sidebarLinks = [
          {
            name: "Share QR Code and Info",
            category: "shareInfo",
            target: "#",
            icon: "qr_code"
          },
          {
            name: "Download",
            category: "download",
            target: "#",
            icon: "download"
          }
        ];
      }
    },
    populateDefaults() {
      this.title = this.$t("share.titleDefault", { title: this.item.name || "share" });
      this.description = this.$t("share.descriptionDefault");
    },
    /**
     * @param {{path: string, source: string}} pathOrData
     */
    updateTempPath(pathOrData) {
      if (pathOrData && pathOrData.path) {
        this.tempPath = pathOrData.path;
        this.tempSource = pathOrData.source;
      }
    },
    async confirmPathChange() {
      if (this.isReassigningPath && this.link) {
        // Reassigning path - call API to update
        try {
          await shareApi.updatePath(this.link.hash, this.tempPath);
          notify.showSuccessToast(this.$t("messages.pathReassigned"));
          this.link.path = this.tempPath;
          this.pathExists = true;
          this.isEditingPath = false;
          this.isReassigningPath = false;
          // Emit event to reload shares in settings view
          eventBus.emit('sharesChanged');
        } catch (e) {
          notify.showError(this.$t("messages.pathReassignFailed"));
          console.error(e);
        }
      }
    },
    cancelPathChange() {
      this.isEditingPath = false;
      this.isReassigningPath = false;
    },
    startPathReassignment() {
      this.isReassigningPath = true;
      this.tempPath = this.displayPath;
      this.tempSource = this.displaySource;
      this.isEditingPath = true;
    },
    handleSidebarLinksUpdate(data) {
      // Update local sidebarLinks when the SidebarLinks prompt saves
      if (data && data.sidebarLinks) {
        this.sidebarLinks = [...data.sidebarLinks];
      }
    },
    openSidebarLinksCustomization() {
      // Prepare share data for the SidebarLinks component
      const shareData = this.isEditMode ? this.link : this.editingLink || {
        hash: this.$route.params.hash || 'new',
        sidebarLinks: this.sidebarLinks,
      };

      mutations.showHover({
        name: 'sidebarLinks',
        props: {
          context: 'share',
          shareData: {
            ...shareData,
            sidebarLinks: this.sidebarLinks,
          },
        },
      });
    },
  },
};
</script>

<style scoped>
.customize-sidebar-links-button {
  width: 100%;
  margin-top: 0.5em;
  display: flex;
  align-items: center;
  justify-content: center;
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

.password-change-section {
  margin-bottom: 1em;
}

.password-change-section button {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5em;
}
</style>
