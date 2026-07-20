<template>
  <div class="user-profile-preferences">
    <SettingsItem
      aria-label="listingOptions"
      :title="$t('settings.listingOptions')"
      :collapsable="true"
      :start-collapsed="listingStartCollapsed"
      :force-collapsed="sectionForceCollapsed('listingOptions')"
      @toggle="onSectionToggle('listingOptions')"
    >
      <div class="settings-items">
        <ProfilePreferenceToggle
          field="deleteWithoutConfirming"
          section="listing"
          :name="$t('profileSettings.deleteWithoutConfirming')"
          :description="$t('profileSettings.deleteWithoutConfirmingDescription')"
        />
        <ProfilePreferenceToggle
          field="dateFormat"
          section="listing"
          :name="$t('profileSettings.setDateFormat')"
        />
        <ProfilePreferenceToggle
          field="showHidden"
          section="listing"
          :name="$t('profileSettings.showHiddenFiles')"
          :description="$t('profileSettings.showHiddenFilesDescription')"
        />
        <ProfilePreferenceToggle
          field="quickDownload"
          section="listing"
          :name="$t('profileSettings.showQuickDownload')"
          :description="$t('profileSettings.showQuickDownloadDescription')"
        />
        <ProfilePreferenceToggle
          field="showSelectMultiple"
          section="listing"
          :name="$t('profileSettings.showSelectMultiple')"
          :description="$t('profileSettings.showSelectMultipleDescription')"
        />
        <ProfilePreferenceToggle
          field="singleClick"
          section="listing"
          :name="$t('profileSettings.singleClick')"
          :description="$t('index.toggleClick')"
        />
        <ProfilePreferenceToggle
          field="showCopyPath"
          section="listing"
          :name="$t('profileSettings.showCopyPath')"
          :description="$t('profileSettings.showCopyPathDescription')"
        />
        <ProfilePreferenceToggle
          field="deleteAfterArchive"
          section="listing"
          :name="$t('profileSettings.deleteAfterArchive')"
          :description="$t('profileSettings.deleteAfterArchiveDescription')"
        />
      </div>
      <template v-if="showExtensionInputs">
        <div
          class="preference-field-block"
          :class="{ 'preference-field-block--enforceable': enforceable }"
        >
          <div class="centered-with-tooltip">
            <h3>{{ $t("profileSettings.hideFileExt") }}</h3>
            <i
              class="no-select material-symbols-outlined tooltip-info-icon"
              @mouseenter="showFieldHelp($event, 'listing', 'hideFileExt', $t('profileSettings.hideFileExtDescription'))"
              @mouseleave="hideTooltip"
            >
              help
            </i>
          </div>
          <div class="form-flex-group">
            <input
              class="input form-form flat-right disable-viewing"
              :class="{ 'form-invalid': !validateExtensions(formHideExt) }"
              type="text"
              :placeholder="$t('profileSettings.disableFileExtensions')"
              v-model="formHideExt"
              :disabled="fieldDisabled('listing', 'hideFileExt')"
            />
            <button
              type="button"
              class="button form-button flat-left"
              :disabled="fieldDisabled('listing', 'hideFileExt')"
              @click="submitHideExtChange"
            >
              {{ $t("general.save") }}
            </button>
          </div>
          <ProfileEnforceSwitch
            :visible="enforceable"
            :enforced="enforcedFlag('listing', 'hideFileExt')"
            :disabled="disabled"
            @update:enforced="(v) => emitEnforced('listing', 'hideFileExt', v)"
          />
        </div>
      </template>
    </SettingsItem>

    <SettingsItem
      aria-label="thumbnailOptions"
      :title="$t('profileSettings.thumbnailOptions')"
      :collapsable="true"
      :start-collapsed="true"
      :force-collapsed="sectionForceCollapsed('thumbnailOptions')"
      @toggle="onSectionToggle('thumbnailOptions')"
    >
      <div class="settings-items">
        <ToggleSwitch
          v-if="showThumbnailMaster"
          class="item"
          :enforceable="enforceable"
          :enforced="enforcedFlag('preview', 'image')"
          v-model="showThumbnailsForPreviews"
          @change="onThumbnailMasterChange"
          @update:enforced="(v) => emitEnforced('preview', 'image', v)"
          :disabled="fieldDisabled('preview', 'image')"
          :name="$t('profileSettings.showThumbnails')"
          :description="helpText('preview', 'image', $t('profileSettings.showThumbnailsDescription'))"
        />
        <template v-if="!showThumbnailMaster || showThumbnailsForPreviews">
          <ProfilePreferenceToggle
            field="image"
            section="preview"
            :name="$t('general.images')"
            :description="$t('profileSettings.previewDescription', { filetype: $t('general.images') })"
          />
          <ProfilePreferenceToggle
            v-if="mediaEnabled"
            field="video"
            section="preview"
            :name="$t('general.videos')"
            :description="$t('profileSettings.previewDescription', { filetype: $t('general.videos') })"
          />
          <ProfilePreferenceToggle
            field="audio"
            section="preview"
            :name="$t('general.audio')"
            :description="$t('profileSettings.previewDescription', { filetype: $t('general.audio') })"
          />
          <ProfilePreferenceToggle
            field="office"
            section="preview"
            :name="$t('general.office')"
            :description="$t('profileSettings.previewOfficeDescription')"
          />
          <ProfilePreferenceToggle
            field="folder"
            section="preview"
            :name="$t('general.folders')"
            :description="$t('profileSettings.previewFolderDescription')"
          />
          <ProfilePreferenceToggle
            field="models"
            section="preview"
            :name="$t('general.models')"
            :description="$t('profileSettings.previewDescription', { filetype: $t('general.models') })"
          />
          <ProfilePreferenceToggle
            field="popup"
            section="preview"
            :name="$t('profileSettings.popupPreview')"
            :description="$t('profileSettings.popupPreviewDescription')"
          />
          <ProfilePreferenceToggle
            v-if="motionPreviewVisible"
            field="motionVideoPreview"
            section="preview"
            :name="$t('profileSettings.previewMotion')"
            :description="$t('profileSettings.previewMotionVideosDescription')"
          />
        </template>
        <template v-if="showExtensionInputs && (!showThumbnailMaster || showThumbnailsForPreviews)">
          <div
            class="preference-field-block"
            :class="{ 'preference-field-block--enforceable': enforceable }"
          >
            <div class="centered-with-tooltip">
              <h3>{{ $t("profileSettings.disableThumbnailPreviews") }}</h3>
              <i
                class="no-select material-symbols-outlined tooltip-info-icon"
              @mouseenter="showFieldHelp($event, 'preview', 'disablePreviewExt', $t('profileSettings.disableThumbnailPreviewsDescription'))"
              @mouseleave="hideTooltip"
            >
              help
            </i>
          </div>
          <div class="form-flex-group">
            <input
              class="input form-form flat-right disable-viewing"
              :class="{ 'form-invalid': !validateExtensions(formDisablePreviews) }"
              type="text"
              :placeholder="$t('profileSettings.disableFileExtensions')"
              v-model="formDisablePreviews"
              :disabled="fieldDisabled('preview', 'disablePreviewExt')"
            />
            <button
              type="button"
              class="button form-button flat-left"
              :disabled="fieldDisabled('preview', 'disablePreviewExt')"
              @click="submitDisablePreviewsChange"
            >
                {{ $t("general.save") }}
              </button>
            </div>
            <ProfileEnforceSwitch
              :visible="enforceable"
              :enforced="enforcedFlag('preview', 'disablePreviewExt')"
              :disabled="disabled"
              @update:enforced="(v) => emitEnforced('preview', 'disablePreviewExt', v)"
            />
          </div>
        </template>
      </div>
    </SettingsItem>

    <SettingsItem
      aria-label="sidebarOptions"
      :title="$t('profileSettings.sidebarOptions')"
      :collapsable="true"
      :start-collapsed="true"
      :force-collapsed="sectionForceCollapsed('sidebarOptions')"
      @toggle="onSectionToggle('sidebarOptions')"
    >
      <div class="settings-items">
        <ProfilePreferenceToggle
          field="disableQuickToggles"
          section="sidebar"
          :name="$t('profileSettings.disableQuickToggles')"
          :description="$t('profileSettings.disableQuickTogglesDescription')"
        />
        <ProfilePreferenceToggle
          field="disableHideOnPreview"
          section="sidebar"
          :name="$t('profileSettings.disableHideSidebar')"
          :description="$t('profileSettings.disableHideSidebarDescription')"
        />
        <ProfilePreferenceToggle
          field="hideFileActions"
          section="sidebar"
          :name="$t('profileSettings.hideSidebarFileActions')"
        />
        <ProfilePreferenceToggle
          field="sticky"
          section="sidebar"
          :name="$t('profileSettings.stickySidebar')"
          :description="$t('index.toggleSticky')"
        />
        <ProfilePreferenceToggle
          field="hideFiles"
          section="sidebar"
          :name="$t('profileSettings.hideFilesInTree')"
          :description="$t('profileSettings.hideFilesInTreeDescription')"
        />
        <ToggleSwitch
          class="item"
          :enforceable="enforceable"
          :enforced="enforcedFlag('sidebar', 'showTools')"
          v-model="showToolsInSidebar"
          @change="() => emitSectionChange('sidebar', 'showTools')"
          @update:enforced="(v) => emitEnforced('sidebar', 'showTools', v)"
          :disabled="fieldDisabled('sidebar', 'showTools')"
          :name="$t('profileSettings.showToolsInSidebar')"
          :description="helpText('sidebar', 'showTools', $t('profileSettings.showToolsInSidebarDescription'))"
        />
      </div>
    </SettingsItem>

    <SettingsItem
      aria-label="searchOptions"
      :title="$t('settings.searchOptions')"
      :collapsable="true"
      :start-collapsed="true"
      :force-collapsed="sectionForceCollapsed('searchOptions')"
      @toggle="onSectionToggle('searchOptions')"
    >
      <div class="settings-items">
        <ProfilePreferenceToggle
          field="disableOptions"
          section="search"
          :name="$t('profileSettings.disableSearchOptions')"
          :description="$t('profileSettings.disableSearchOptionsDescription')"
        />
      </div>
    </SettingsItem>

    <SettingsItem
      aria-label="fileViewerOptions"
      :title="$t('profileSettings.fileViewerOptions')"
      :collapsable="true"
      :start-collapsed="true"
      :force-collapsed="sectionForceCollapsed('fileViewerOptions')"
      @toggle="onSectionToggle('fileViewerOptions')"
    >
      <div class="settings-items">
        <ProfilePreferenceToggle
          field="defaultMediaPlayer"
          section="fileViewer"
          :name="$t('profileSettings.defaultMediaPlayer')"
          :description="$t('profileSettings.defaultMediaPlayerDescription')"
        />
        <ToggleSwitch
          class="item"
          :enforceable="enforceable"
          :enforced="enforcedFlag('fileViewer', 'autoplayMedia')"
          v-model="autoplayMedia"
          @change="() => emitSectionChange('fileViewer', 'autoplayMedia')"
          @update:enforced="(v) => emitEnforced('fileViewer', 'autoplayMedia', v)"
          :disabled="fieldDisabled('fileViewer', 'autoplayMedia')"
          :name="$t('profileSettings.autoplayMedia')"
          :description="helpText('fileViewer', 'autoplayMedia', $t('profileSettings.autoplayMediaDescription'))"
        />
        <ProfilePreferenceToggle
          field="editorQuickSave"
          section="fileViewer"
          :name="$t('profileSettings.editorQuickSave')"
          :description="$t('profileSettings.editorQuickSaveDescription')"
        />
        <ProfilePreferenceToggle
          field="preferEditorForMarkdown"
          section="fileViewer"
          :name="$t('profileSettings.preferEditorForMarkdown')"
          :description="$t('profileSettings.preferEditorForMarkdownDescription')"
        />
      </div>
      <template v-if="showExtensionInputs">
        <div
          class="preference-field-block"
          :class="{ 'preference-field-block--enforceable': enforceable }"
        >
          <div class="centered-with-tooltip">
            <h3>{{ $t("profileSettings.disableViewingFiles") }}</h3>
            <i
              class="no-select material-symbols-outlined tooltip-info-icon"
              @mouseenter="showFieldHelp($event, 'fileViewer', 'disableViewingExt', $t('profileSettings.disableViewingFilesDescription'))"
              @mouseleave="hideTooltip"
            >
              help
            </i>
          </div>
          <div class="form-flex-group">
            <input
              class="input form-form flat-right disable-viewing"
              :class="{ 'form-invalid': !validateExtensions(formDisabledViewing) }"
              type="text"
              :placeholder="$t('profileSettings.disableFileExtensions')"
              v-model="formDisabledViewing"
              :disabled="fieldDisabled('fileViewer', 'disableViewingExt')"
            />
            <button
              type="button"
              class="button form-button flat-left"
              :disabled="fieldDisabled('fileViewer', 'disableViewingExt')"
              @click="submitDisabledViewingChange"
            >
              {{ $t("general.save") }}
            </button>
          </div>
          <ProfileEnforceSwitch
            :visible="enforceable"
            :enforced="enforcedFlag('fileViewer', 'disableViewingExt')"
            :disabled="disabled"
            @update:enforced="(v) => emitEnforced('fileViewer', 'disableViewingExt', v)"
          />
        </div>
        <div v-if="onlyOfficeAvailable">
          <div
            class="preference-field-block"
            :class="{ 'preference-field-block--enforceable': enforceable }"
          >
            <div class="centered-with-tooltip">
              <h3>{{ $t("profileSettings.disableOfficeEditor") }}</h3>
              <i
                class="no-select material-symbols-outlined tooltip-info-icon"
                @mouseenter="showFieldHelp($event, 'fileViewer', 'disableOnlyOfficeExt', $t('profileSettings.disableOfficeEditorDescription'))"
                @mouseleave="hideTooltip"
              >
                help
              </i>
            </div>
            <div class="form-flex-group">
              <input
                class="input form-form flat-right"
                :class="{ 'form-invalid': !validateExtensions(formDisableOfficeViewing) }"
                type="text"
                :placeholder="$t('profileSettings.disableFileExtensions')"
                v-model="formDisableOfficeViewing"
                :disabled="fieldDisabled('fileViewer', 'disableOnlyOfficeExt')"
              />
              <button
                type="button"
                class="button form-button flat-left"
                :disabled="fieldDisabled('fileViewer', 'disableOnlyOfficeExt')"
                @click="submitDisableOfficeViewingChange"
              >
                {{ $t("general.save") }}
              </button>
            </div>
            <ProfileEnforceSwitch
              :visible="enforceable"
              :enforced="enforcedFlag('fileViewer', 'disableOnlyOfficeExt')"
              :disabled="disabled"
              @update:enforced="(v) => emitEnforced('fileViewer', 'disableOnlyOfficeExt', v)"
            />
          </div>
          <div class="settings-items">
            <ProfilePreferenceToggle
              field="debugOffice"
              section="fileViewer"
              :name="$t('profileSettings.debugOfficeEditor')"
              :description="$t('profileSettings.debugOfficeEditorDescription')"
            />
          </div>
        </div>
        <div v-else class="settings-items">
          <ProfilePreferenceToggle
            field="debugOffice"
            section="fileViewer"
            :name="$t('profileSettings.debugOfficeEditor')"
            :description="$t('profileSettings.debugOfficeEditorDescription')"
          />
        </div>
      </template>
      <template v-else>
        <div class="settings-items">
          <ProfilePreferenceToggle
            field="debugOffice"
            section="fileViewer"
            :name="$t('profileSettings.debugOfficeEditor')"
            :description="$t('profileSettings.debugOfficeEditorDescription')"
          />
        </div>
      </template>
    </SettingsItem>

    <SettingsItem
      aria-label="themeLanguage"
      :title="$t('profileSettings.themeAndLanguage')"
      :collapsable="true"
      :start-collapsed="true"
      :force-collapsed="sectionForceCollapsed('themeLanguage')"
      @toggle="onSectionToggle('themeLanguage')"
    >
      <div class="settings-items">
        <ToggleSwitch
          class="item"
          :enforceable="enforceable"
          :enforced="enforcedFlag('ui', 'darkMode')"
          v-model="darkMode"
          @change="() => emitSectionChange('ui', 'darkMode')"
          @update:enforced="(v) => emitEnforced('ui', 'darkMode', v)"
          :disabled="fieldDisabled('ui', 'darkMode')"
          :name="$t('profileSettings.darkMode')"
          :description="helpText('ui', 'darkMode', $t('index.toggleDark'))"
        />
        <div
          class="preference-field-block"
          :class="{ 'preference-field-block--enforceable': enforceable }"
        >
          <h4>{{ $t("settings.themeColor") }}</h4>
          <ButtonGroup
            :buttons="colorChoices"
            @button-clicked="setColor"
            :initialActive="themeColorValue"
            :is-disabled="fieldDisabled('ui', 'themeColor')"
            :disable-message="$t('profileSettings.enforcedByAdmin')"
          />
          <ProfileEnforceSwitch
            :visible="enforceable"
            :enforced="enforcedFlag('ui', 'themeColor')"
            :disabled="disabled"
            @update:enforced="(v) => emitEnforced('ui', 'themeColor', v)"
          />
        </div>
        <div
          v-if="Object.keys(availableThemes).length > 0"
          class="preference-field-block"
          :class="{ 'preference-field-block--enforceable': enforceable }"
        >
          <h4>{{ $t("profileSettings.customTheme") }}</h4>
          <div class="form-flex-group">
            <ExpandDropdown
              v-model="selectedTheme"
              :options="themeOptions"
              :aria-label="$t('general.theme')"
              :disabled="fieldDisabled('ui', 'customTheme')"
              @update:model-value="onThemeChange"
            />
          </div>
          <ProfileEnforceSwitch
            :visible="enforceable"
            :enforced="enforcedFlag('ui', 'customTheme')"
            :disabled="disabled"
            @update:enforced="(v) => emitEnforced('ui', 'customTheme', v)"
          />
        </div>
        <div
          class="preference-field-block"
          :class="{ 'preference-field-block--enforceable': enforceable }"
        >
          <h4>{{ $t("general.language") }}</h4>
          <div class="form-flex-group">
            <Languages
              :locale="localeValue"
              :disabled="fieldDisabled('ui', 'locale')"
              @update:locale="onLocaleChange"
            />
          </div>
          <ProfileEnforceSwitch
            :visible="enforceable"
            :enforced="enforcedFlag('ui', 'locale')"
            :disabled="disabled"
            @update:enforced="(v) => emitEnforced('ui', 'locale', v)"
          />
        </div>
      </div>
    </SettingsItem>
  </div>
</template>

<script>
import { notify } from "@/notify";
import { globalVars } from "@/utils/constants.js";
import { state, mutations, getters } from "@/store";
import { getObjectProperty, setObjectProperty } from "@/utils/object.js";
import ProfilePreferenceToggle from "@/components/settings/ProfilePreferenceToggle.vue";
import ProfileEnforceSwitch from "@/components/settings/ProfileEnforceSwitch.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import Languages from "@/components/settings/Languages.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import ButtonGroup from "@/components/ButtonGroup.vue";

export default {
  name: "UserProfilePreferences",
  components: {
    ToggleSwitch,
    SettingsItem,
    Languages,
    ExpandDropdown,
    ButtonGroup,
    ProfilePreferenceToggle,
    ProfileEnforceSwitch,
  },
  provide() {
    return { profilePrefs: this };
  },
  props: {
    modelValue: {
      type: Object,
      required: true,
    },
    enforced: {
      type: Object,
      default: () => ({}),
    },
    enforceable: {
      type: Boolean,
      default: false,
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    showExtensionInputs: {
      type: Boolean,
      default: false,
    },
    showThumbnailMaster: {
      type: Boolean,
      default: false,
    },
    /** When set, sections use accordion collapse (Profile settings page). */
    accordionExpanded: {
      type: String,
      default: null,
    },
    listingStartCollapsed: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:modelValue", "change", "enforced-change", "update:accordionExpanded", "theme-color", "locale-change"],
  data() {
    return {
      formDisablePreviews: "",
      formDisabledViewing: "",
      formDisableOfficeViewing: "",
      formHideExt: "",
    };
  },
  computed: {
    sections: {
      get() {
        return this.modelValue;
      },
      set(val) {
        this.$emit("update:modelValue", val);
      },
    },
    mediaEnabled() {
      return globalVars.mediaAvailable;
    },
    onlyOfficeAvailable() {
      return globalVars.onlyOfficeUrl !== "";
    },
    availableThemes() {
      return globalVars.userSelectableThemes || {};
    },
    colorChoices() {
      return [
        { label: this.$t("colors.blue"), value: "var(--blue)" },
        { label: this.$t("colors.red"), value: "var(--red)" },
        { label: this.$t("colors.green"), value: "var(--icon-green)" },
        { label: this.$t("colors.violet"), value: "var(--icon-violet)" },
        { label: this.$t("colors.yellow"), value: "var(--icon-yellow)" },
        { label: this.$t("colors.orange"), value: "var(--icon-orange)" },
      ];
    },
    themeOptions() {
      return Object.entries(this.availableThemes).map(([key, theme]) => ({
        value: key,
        label: String(key) === "default"
          ? this.$t("profileSettings.defaultThemeDescription")
          : `${key} - ${theme.description}`,
      }));
    },
    motionPreviewVisible() {
      const p = this.sections.preview || {};
      return !!p.popup && ((this.mediaEnabled && p.video) || p.folder);
    },
    showThumbnailsForPreviews: {
      get() {
        const p = this.sections.preview || {};
        return !!(p.image || p.audio || p.video || p.motionVideoPreview || p.office || p.popup || p.folder || p.models);
      },
      set(enabled) {
        const next = { ...this.sections, preview: { ...(this.sections.preview || {}) } };
        if (enabled) {
          next.preview.image = true;
        } else {
          next.preview.image = false;
          next.preview.audio = false;
          next.preview.video = false;
          next.preview.motionVideoPreview = false;
          next.preview.office = false;
          next.preview.popup = false;
          next.preview.folder = false;
          next.preview.models = false;
        }
        this.sections = next;
      },
    },
    showToolsInSidebar: {
      get() {
        const v = this.sections.sidebar?.showTools;
        return v === undefined || v === null ? true : !!v;
      },
      set(val) {
        const next = { ...this.sections, sidebar: { ...(this.sections.sidebar || {}), showTools: val } };
        this.sections = next;
      },
    },
    darkMode: {
      get() {
        const v = this.sections.ui?.darkMode;
        return v === undefined || v === null ? true : !!v;
      },
      set(val) {
        const next = { ...this.sections, ui: { ...(this.sections.ui || {}), darkMode: val } };
        this.sections = next;
      },
    },
    autoplayMedia: {
      get() {
        const v = this.sections.fileViewer?.autoplayMedia;
        return v === undefined || v === null ? true : !!v;
      },
      set(val) {
        const next = { ...this.sections, fileViewer: { ...(this.sections.fileViewer || {}), autoplayMedia: val } };
        this.sections = next;
      },
    },
    themeColorValue() {
      return this.sections.ui?.themeColor || "";
    },
    localeValue() {
      return this.sections.ui?.locale || "";
    },
    selectedTheme: {
      get() {
        return this.sections.ui?.customTheme || "default";
      },
      set(value) {
        const next = { ...this.sections, ui: { ...(this.sections.ui || {}), customTheme: value } };
        this.sections = next;
      },
    },
  },
  watch: {
    modelValue: {
      deep: true,
      handler() {
        this.syncExtensionFormsFromSections();
      },
    },
  },
  mounted() {
    this.syncExtensionFormsFromSections();
  },
  methods: {
    syncExtensionFormsFromSections() {
      this.formHideExt = this.sections.listing?.hideFileExt || "";
      this.formDisablePreviews = this.sections.preview?.disablePreviewExt || "";
      this.formDisabledViewing = this.sections.fileViewer?.disableViewingExt || "";
      this.formDisableOfficeViewing = this.sections.fileViewer?.disableOnlyOfficeExt || "";
    },
    sectionForceCollapsed(sectionKey) {
      if (this.accordionExpanded === null) {
        return null;
      }
      return this.accordionExpanded !== sectionKey;
    },
    onSectionToggle(sectionKey) {
      if (this.accordionExpanded === null) {
        return;
      }
      const next = this.accordionExpanded === sectionKey ? null : sectionKey;
      this.$emit("update:accordionExpanded", next);
    },
    enforcedFlag(section, field) {
      if (section === "account" && field.includes(".")) {
        const [head, ...rest] = field.split(".");
        if (head === "permissions") {
          const key = rest.join(".");
          const perms = getObjectProperty(getObjectProperty(this.enforced, "account"), "permissions");
          return !!getObjectProperty(perms, key);
        }
      }
      const sectionEnforced = getObjectProperty(this.enforced, section);
      return !!getObjectProperty(sectionEnforced, field);
    },
    fieldLocked(section, field) {
      return !this.enforceable && this.enforcedFlag(section, field);
    },
    fieldDisabled(section, field) {
      return this.disabled || this.fieldLocked(section, field);
    },
    helpText(section, field, description) {
      if (this.fieldLocked(section, field)) {
        return this.$t("profileSettings.enforcedByAdmin");
      }
      return description || "";
    },
    showFieldHelp(event, section, field, description) {
      this.showTooltip(event, this.helpText(section, field, description));
    },
    sectionBool(section, field) {
      if (section === "account" && field.includes(".")) {
        const [head, ...rest] = field.split(".");
        if (head === "permissions") {
          const key = rest.join(".");
          const perms = getObjectProperty(getObjectProperty(this.sections, "account"), "permissions");
          const val = getObjectProperty(perms, key);
          if (val === undefined || val === null) {
            return key === "download" ? true : false;
          }
          return !!val;
        }
      }
      const sectionData = getObjectProperty(this.sections, section);
      const val = getObjectProperty(sectionData, field);
      if (val === undefined || val === null) {
        if (section === "preview") {
          return true;
        }
        return false;
      }
      return !!val;
    },
    setSectionBool(section, field, value) {
      if (section === "account" && field.includes(".")) {
        const [head, ...rest] = field.split(".");
        if (head === "permissions") {
          const key = rest.join(".");
          const next = {
            ...this.sections,
            account: {
              ...(this.sections.account || {}),
              permissions: {
                ...(this.sections.account?.permissions || {}),
                [key]: value,
              },
            },
          };
          this.sections = next;
          return;
        }
      }
      const sectionSnapshot = getObjectProperty(this.sections, section) || {};
      const updatedSection = setObjectProperty(sectionSnapshot, field, value);
      this.sections = setObjectProperty(this.sections, section, updatedSection);
    },
    emitSectionChange(section, field) {
      this.$emit("change", { section, field });
    },
    emitEnforced(section, field, value) {
      this.$emit("enforced-change", { section, field, value });
    },
    onThumbnailMasterChange() {
      const previewFields = [
        "image",
        "audio",
        "video",
        "motionVideoPreview",
        "office",
        "popup",
        "folder",
        "models",
      ];
      if (this.showThumbnailsForPreviews) {
        this.emitSectionChange("preview", "image");
        return;
      }
      for (const field of previewFields) {
        this.emitSectionChange("preview", field);
      }
    },
    setColor(color) {
      if (getters.eventTheme() === "halloween" && !state.disableEventThemes) {
        mutations.disableEventThemes();
      }
      const next = { ...this.sections, ui: { ...(this.sections.ui || {}), themeColor: color } };
      this.sections = next;
      this.$emit("theme-color", color);
      this.emitSectionChange("ui", "themeColor");
    },
    onThemeChange() {
      this.emitSectionChange("ui", "customTheme");
    },
    onLocaleChange(locale) {
      const next = { ...this.sections, ui: { ...(this.sections.ui || {}), locale } };
      this.sections = next;
      this.$emit("locale-change", locale);
      this.emitSectionChange("ui", "locale");
    },
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
    validateExtensions(value) {
      const normalized = String(value ?? "").trim();
      if (normalized === "" || normalized === "*") {
        return true;
      }
      const parts = normalized.split(/\s+/);
      const extensionRegex = /^\.\w+$/;
      return parts.every((part) => extensionRegex.test(part));
    },
    submitDisablePreviewsChange() {
      if (!this.validateExtensions(this.formDisablePreviews)) {
        notify.showError("Invalid input, does not match requirement.");
        return;
      }
      const next = {
        ...this.sections,
        preview: { ...(this.sections.preview || {}), disablePreviewExt: this.formDisablePreviews },
      };
      this.sections = next;
      this.emitSectionChange("preview", "disablePreviewExt");
    },
    submitDisabledViewingChange() {
      if (!this.validateExtensions(this.formDisabledViewing)) {
        notify.showError("Invalid input, does not match requirement.");
        return;
      }
      const next = {
        ...this.sections,
        fileViewer: { ...(this.sections.fileViewer || {}), disableViewingExt: this.formDisabledViewing },
      };
      this.sections = next;
      this.emitSectionChange("fileViewer", "disableViewingExt");
    },
    submitHideExtChange() {
      if (!this.validateExtensions(this.formHideExt)) {
        notify.showError("Invalid input, does not match requirement.");
        return;
      }
      const next = {
        ...this.sections,
        listing: { ...(this.sections.listing || {}), hideFileExt: this.formHideExt },
      };
      this.sections = next;
      this.emitSectionChange("listing", "hideFileExt");
    },
    submitDisableOfficeViewingChange() {
      if (!this.validateExtensions(this.formDisableOfficeViewing)) {
        notify.showError("Invalid input, does not match requirement.");
        return;
      }
      const next = {
        ...this.sections,
        fileViewer: {
          ...(this.sections.fileViewer || {}),
          disableOnlyOfficeExt: this.formDisableOfficeViewing,
        },
      };
      this.sections = next;
      this.emitSectionChange("fileViewer", "disableOnlyOfficeExt");
    },
  },
};
</script>

<style scoped>
.disable-viewing {
  width: 100%;
}
.centered-with-tooltip {
  display: flex;
  justify-content: center;
  align-items: center;
}
.preference-field-block--enforceable {
  padding: 0.35em;
  border-radius: var(--borderRadius);
  margin-bottom: 0.5em;
}
</style>
