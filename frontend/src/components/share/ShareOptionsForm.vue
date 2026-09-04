<template>
  <div class="share-options-form">
    <div v-if="showBasicOptions">
      <div
        class="preference-field-block"
        :class="{ 'preference-field-block--enforceable': enforceable }"
      >
        <p>
          {{ $t("share.shareType") }}
          <i
            class="material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareTypeDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <ExpandDropdown
          :model-value="modelValue.shareType"
          :options="shareTypeOptions"
          :aria-label="$t('share.shareType')"
          :disabled="fieldDisabled('shareType')"
          @update:model-value="setField('shareType', $event)"
        />
        <ProfileEnforceSwitch
          :visible="enforceable"
          :enforced="enforcedFlag('shareType')"
          @update:enforced="(v) => emitEnforced('shareType', v)"
        />
      </div>
      <div
        class="preference-field-block"
        :class="{ 'preference-field-block--enforceable': enforceable }"
      >
      <button
        type="button"
        class="button button--flat customize-sidebar-links-button"
        :disabled="fieldDisabled('sidebarLinks')"
        @click="$emit('customize-sidebar-links')"
      >
        <i class="material-symbols">link</i>
        {{ $t("share.customizeSidebarLinksButton") }}
      </button>
      <ProfileEnforceSwitch
        :visible="enforceable"
        :enforced="enforcedFlag('sidebarLinks')"
        @update:enforced="(v) => emitEnforced('sidebarLinks', v)"
      />
      </div>
      <div class="settings-items" style="margin-top: 0.5em;">
        <ToggleSwitch
          v-if="modelValue.shareType === 'normal'"
          class="item"
          :model-value="modelValue.allowModify"
          :name="$t('share.allowModify')"
          :description="$t('share.allowModifyDescription')"
          aria-label="allow editing files toggle"
          :disabled="fieldDisabled('allowModify')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('allowModify')"
          :enforcement-disabled="enforcementDisabled('allowModify')"
          :enforcement-locked="isEnforcementLocked('allowModify')"
          :value-tooltip="enforcementLockTooltip('allowModify')"
          @update:model-value="setField('allowModify', $event)"
          @update:enforced="(v) => emitEnforced('allowModify', v)"
        />
        <ToggleSwitch
          v-if="modelValue.shareType === 'normal'"
          class="item"
          :model-value="modelValue.allowCreate"
          :name="$t('share.allowCreate')"
          :description="$t('share.allowCreateDescription')"
          aria-label="allow creating and uploading files and folders toggle"
          :disabled="fieldDisabled('allowCreate')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('allowCreate')"
          :enforcement-disabled="enforcementDisabled('allowCreate')"
          :enforcement-locked="isEnforcementLocked('allowCreate')"
          :value-tooltip="enforcementLockTooltip('allowCreate')"
          @update:model-value="setField('allowCreate', $event)"
          @update:enforced="(v) => emitEnforced('allowCreate', v)"
        />
        <ToggleSwitch
          v-if="modelValue.shareType === 'normal'"
          class="item"
          :model-value="modelValue.allowDelete"
          :name="$t('share.allowDelete')"
          :description="$t('share.allowDeleteDescription')"
          aria-label="allow deleting files toggle"
          :disabled="fieldDisabled('allowDelete')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('allowDelete')"
          :enforcement-disabled="enforcementDisabled('allowDelete')"
          :enforcement-locked="isEnforcementLocked('allowDelete')"
          :value-tooltip="enforcementLockTooltip('allowDelete')"
          @update:model-value="setField('allowDelete', $event)"
          @update:enforced="(v) => emitEnforced('allowDelete', v)"
        />
      </div>
    </div>

    <SettingsItem
      :title="showMoreExpanded ? $t('buttons.showLess') : $t('buttons.showMore')"
      :collapsable="true"
      :start-collapsed="!showMoreExpanded"
      @toggle="showMoreExpanded = $event"
    >
      <div class="settings-items">
        <div
          class="preference-field-block"
          :class="{ 'preference-field-block--enforceable': enforceable }"
        >
          <p>
            {{ shareThemeLabel() }}
            <i
              class="material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.shareThemeDescription'))"
              @mouseleave="hideTooltip"
            >
              help
            </i>
          </p>
          <div v-if="Object.keys(availableThemes).length > 0" class="form-flex-group">
            <ExpandDropdown
              :model-value="modelValue.shareTheme"
              :options="shareThemeOptions"
              :aria-label="shareThemeLabel()"
              :disabled="fieldDisabled('shareTheme')"
              @update:model-value="setField('shareTheme', $event)"
            />
          </div>
          <ProfileEnforceSwitch
            :visible="enforceable"
            :enforced="enforcedFlag('shareTheme')"
            @update:enforced="(v) => emitEnforced('shareTheme', v)"
          />
        </div>

        <div v-if="modelValue.shareType === 'normal'">
          <div
            class="preference-field-block"
            :class="{ 'preference-field-block--enforceable': enforceable }"
          >
            <p>
              {{ $t("share.defaultViewMode") }}
              <i
                class="material-symbols-outlined tooltip-info-icon"
                @mouseenter="showTooltip($event, $t('share.defaultViewModeDescription'))"
                @mouseleave="hideTooltip"
              >
                help
              </i>
            </p>
            <ExpandDropdown
              :model-value="modelValue.viewMode"
              :options="viewModeOptions"
              :aria-label="$t('share.defaultViewMode')"
              :disabled="fieldDisabled('viewMode')"
              @update:model-value="setField('viewMode', $event)"
            />
            <ProfileEnforceSwitch
              :visible="enforceable"
              :enforced="enforcedFlag('viewMode')"
              @update:enforced="(v) => emitEnforced('viewMode', v)"
            />
          </div>
        </div>

        <ToggleSwitch
          v-if="createAllowed"
          class="item"
          :model-value="modelValue.allowReplacements"
          :name="$t('share.allowReplacements')"
          :description="$t('share.allowReplacementsDescription')"
          :disabled="fieldDisabled('allowReplacements')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('allowReplacements')"
          :enforcement-disabled="enforcementDisabled('allowReplacements')"
          :enforcement-locked="isEnforcementLocked('allowReplacements')"
          :value-tooltip="enforcementLockTooltip('allowReplacements')"
          @update:model-value="setField('allowReplacements', $event)"
          @update:enforced="(v) => emitEnforced('allowReplacements', v)"
        />
        <ToggleSwitch
          v-if="modelValue.shareType === 'normal'"
          class="item"
          :model-value="modelValue.disableDownload"
          :name="$t('share.disableDownload')"
          :description="$t('share.disableDownloadDescription')"
          aria-label="disable downloading files toggle"
          :disabled="fieldDisabled('disableDownload')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('disableDownload')"
          :enforcement-disabled="enforcementDisabled('disableDownload')"
          :enforcement-locked="isEnforcementLocked('disableDownload')"
          :value-tooltip="enforcementLockTooltip('disableDownload')"
          @update:model-value="setField('disableDownload', $event)"
          @update:enforced="(v) => emitEnforced('disableDownload', v)"
        />
        <ToggleSwitch
          v-if="modelValue.shareType === 'normal'"
          class="item"
          :model-value="modelValue.disableFileViewer"
          :name="$t('share.disableFileViewer')"
          :disabled="fieldDisabled('disableFileViewer')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('disableFileViewer')"
          :enforcement-disabled="enforcementDisabled('disableFileViewer')"
          :enforcement-locked="isEnforcementLocked('disableFileViewer')"
          :value-tooltip="enforcementLockTooltip('disableFileViewer')"
          @update:model-value="setField('disableFileViewer', $event)"
          @update:enforced="(v) => emitEnforced('disableFileViewer', v)"
        />
        <ToggleSwitch
          v-if="modelValue.shareType === 'normal'"
          class="item"
          :model-value="modelValue.quickDownload"
          :name="$t('profileSettings.showQuickDownload')"
          :description="$t('profileSettings.showQuickDownloadDescription')"
          :disabled="fieldDisabled('quickDownload')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('quickDownload')"
          :enforcement-disabled="enforcementDisabled('quickDownload')"
          :enforcement-locked="isEnforcementLocked('quickDownload')"
          :value-tooltip="enforcementLockTooltip('quickDownload')"
          @update:model-value="setField('quickDownload', $event)"
          @update:enforced="(v) => emitEnforced('quickDownload', v)"
        />
        <ToggleSwitch
          class="item"
          :model-value="modelValue.disableAnonymous"
          :name="$t('share.disableAnonymous')"
          :description="$t('share.disableAnonymousDescription')"
          :disabled="fieldDisabled('disableAnonymous')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('disableAnonymous')"
          :enforcement-disabled="enforcementDisabled('disableAnonymous')"
          :enforcement-locked="isEnforcementLocked('disableAnonymous')"
          :value-tooltip="enforcementLockTooltip('disableAnonymous')"
          @update:model-value="setField('disableAnonymous', $event)"
          @update:enforced="(v) => emitEnforced('disableAnonymous', v)"
        />
        <ToggleSwitch
          class="item"
          :model-value="modelValue.enableAllowedUsernames"
          :name="$t('share.enableAllowedUsernames')"
          :description="$t('share.enableAllowedUsernamesDescription')"
          :disabled="fieldDisabled('enableAllowedUsernames')"
          @update:model-value="setField('enableAllowedUsernames', $event)"
        />

        <div v-if="modelValue.enableAllowedUsernames" class="item">
          <div
            class="preference-field-block"
            :class="{ 'preference-field-block--enforceable': enforceable }"
          >
            <input
              class="input"
              type="text"
              :value="modelValue.allowedUsernames"
              :placeholder="$t('share.allowedUsernamesPlaceholder')"
              :disabled="fieldDisabled('allowedUsernames')"
              @input="setField('allowedUsernames', $event.target.value)"
            />
            <ProfileEnforceSwitch
              :visible="enforceable"
              :enforced="enforcedFlag('allowedUsernames')"
              @update:enforced="(v) => emitEnforced('allowedUsernames', v)"
            />
          </div>
        </div>

        <ToggleSwitch
          v-if="modelValue.shareType === 'normal' && onlyOfficeAvailable"
          class="item"
          :model-value="modelValue.enableOnlyOffice"
          :name="$t('share.enableOnlyOffice')"
          :description="$t('share.enableOnlyOfficeDescription')"
          :disabled="fieldDisabled('enableOnlyOffice')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('enableOnlyOffice')"
          :enforcement-disabled="enforcementDisabled('enableOnlyOffice')"
          :enforcement-locked="isEnforcementLocked('enableOnlyOffice')"
          :value-tooltip="enforcementLockTooltip('enableOnlyOffice')"
          @update:model-value="setField('enableOnlyOffice', $event)"
          @update:enforced="(v) => emitEnforced('enableOnlyOffice', v)"
        />

        <div
          class="preference-field-block"
          :class="{ 'preference-field-block--enforceable': enforceable }"
        >
          <p>
            {{ $t("share.enforceDarkLightMode") }}
            <i
              class="material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.enforceDarkLightModeDescription'))"
              @mouseleave="hideTooltip"
            >
              help
            </i>
          </p>
          <ExpandDropdown
            :model-value="modelValue.enforceDarkLightMode"
            :options="enforceDarkLightModeOptions"
            :aria-label="$t('share.enforceDarkLightMode')"
            :disabled="fieldDisabled('enforceDarkLightMode')"
            @update:model-value="setField('enforceDarkLightMode', $event)"
          />
          <ProfileEnforceSwitch
            :visible="enforceable"
            :enforced="enforcedFlag('enforceDarkLightMode')"
            @update:enforced="(v) => emitEnforced('enforceDarkLightMode', v)"
          />
        </div>

        <ToggleSwitch
          class="item"
          :model-value="modelValue.keepAfterExpiration"
          :name="$t('share.keepAfterExpiration')"
          :description="$t('share.keepAfterExpirationDescription')"
          :disabled="fieldDisabled('keepAfterExpiration')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('keepAfterExpiration')"
          :enforcement-disabled="enforcementDisabled('keepAfterExpiration')"
          :enforcement-locked="isEnforcementLocked('keepAfterExpiration')"
          :value-tooltip="enforcementLockTooltip('keepAfterExpiration')"
          @update:model-value="setField('keepAfterExpiration', $event)"
          @update:enforced="(v) => emitEnforced('keepAfterExpiration', v)"
        />
        <ToggleSwitch
          v-if="modelValue.shareType === 'normal'"
          class="item"
          :model-value="modelValue.disableThumbnails"
          :name="$t('share.disableThumbnails')"
          :description="$t('share.disableThumbnailsDescription')"
          :disabled="fieldDisabled('disableThumbnails')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('disableThumbnails')"
          :enforcement-disabled="enforcementDisabled('disableThumbnails')"
          :enforcement-locked="isEnforcementLocked('disableThumbnails')"
          :value-tooltip="enforcementLockTooltip('disableThumbnails')"
          @update:model-value="setField('disableThumbnails', $event)"
          @update:enforced="(v) => emitEnforced('disableThumbnails', v)"
        />
        <ToggleSwitch
          v-if="modelValue.shareType === 'normal'"
          class="item"
          :model-value="modelValue.showHidden"
          :name="$t('profileSettings.showHiddenFiles')"
          :description="$t('profileSettings.showHiddenFilesDescription')"
          :disabled="fieldDisabled('showHidden')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('showHidden')"
          :enforcement-disabled="enforcementDisabled('showHidden')"
          :enforcement-locked="isEnforcementLocked('showHidden')"
          :value-tooltip="enforcementLockTooltip('showHidden')"
          @update:model-value="setField('showHidden', $event)"
          @update:enforced="(v) => emitEnforced('showHidden', v)"
        />

        <div
          class="preference-field-block"
          :class="{ 'preference-field-block--enforceable': enforceable }"
        >
          <p>
            {{ $t("profileSettings.hideFileExt") }}
            <i
              class="material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('profileSettings.hideFileExtDescription'))"
              @mouseleave="hideTooltip"
            >
              help
            </i>
          </p>
          <input
            class="input"
            :class="{ 'form-invalid': !validateExtensions(modelValue.hideFileExt) }"
            type="text"
            :placeholder="$t('profileSettings.disableFileExtensions')"
            :value="modelValue.hideFileExt"
            :disabled="fieldDisabled('hideFileExt')"
            @input="setField('hideFileExt', $event.target.value)"
          />
          <ProfileEnforceSwitch
            :visible="enforceable"
            :enforced="enforcedFlag('hideFileExt')"
            @update:enforced="(v) => emitEnforced('hideFileExt', v)"
          />
        </div>

        <ToggleSwitch
          v-if="modelValue.shareType !== 'upload'"
          class="item"
          :model-value="modelValue.hideNavButtons"
          :name="$t('share.hideNavButtons')"
          :description="$t('share.hideNavButtonsDescription')"
          :disabled="fieldDisabled('hideNavButtons')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('hideNavButtons')"
          :enforcement-disabled="enforcementDisabled('hideNavButtons')"
          :enforcement-locked="isEnforcementLocked('hideNavButtons')"
          :value-tooltip="enforcementLockTooltip('hideNavButtons')"
          @update:model-value="setField('hideNavButtons', $event)"
          @update:enforced="(v) => emitEnforced('hideNavButtons', v)"
        />
        <ToggleSwitch
          class="item"
          :model-value="modelValue.disableShareCard"
          :name="$t('share.disableShareCard')"
          :description="$t('share.disableShareCardDescription')"
          :disabled="fieldDisabled('disableShareCard')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('disableShareCard')"
          :enforcement-disabled="enforcementDisabled('disableShareCard')"
          :enforcement-locked="isEnforcementLocked('disableShareCard')"
          :value-tooltip="enforcementLockTooltip('disableShareCard')"
          @update:model-value="setField('disableShareCard', $event)"
          @update:enforced="(v) => emitEnforced('disableShareCard', v)"
        />
        <ToggleSwitch
          class="item"
          :model-value="modelValue.disableSidebar"
          :name="$t('share.disableSidebar')"
          :description="$t('share.disableSidebarDescription')"
          :disabled="fieldDisabled('disableSidebar')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('disableSidebar')"
          :enforcement-disabled="enforcementDisabled('disableSidebar')"
          :enforcement-locked="isEnforcementLocked('disableSidebar')"
          :value-tooltip="enforcementLockTooltip('disableSidebar')"
          @update:model-value="setField('disableSidebar', $event)"
          @update:enforced="(v) => emitEnforced('disableSidebar', v)"
        />
        <ToggleSwitch
          v-if="modelValue.shareType === 'normal'"
          class="item"
          :model-value="modelValue.perUserDownloadLimit"
          :name="$t('share.perUserDownloadLimit')"
          :description="$t('share.perUserDownloadLimitDescription')"
          :disabled="fieldDisabled('perUserDownloadLimit')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('perUserDownloadLimit')"
          :enforcement-disabled="enforcementDisabled('perUserDownloadLimit')"
          :enforcement-locked="isEnforcementLocked('perUserDownloadLimit')"
          :value-tooltip="enforcementLockTooltip('perUserDownloadLimit')"
          @update:model-value="setField('perUserDownloadLimit', $event)"
          @update:enforced="(v) => emitEnforced('perUserDownloadLimit', v)"
        />
        <ToggleSwitch
          v-if="modelValue.shareType === 'normal'"
          class="item"
          :model-value="modelValue.extractEmbeddedSubtitles"
          :name="$t('share.extractEmbeddedSubtitles')"
          :description="$t('share.extractEmbeddedSubtitlesDescription')"
          :disabled="fieldDisabled('extractEmbeddedSubtitles')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('extractEmbeddedSubtitles')"
          :enforcement-disabled="enforcementDisabled('extractEmbeddedSubtitles')"
          :enforcement-locked="isEnforcementLocked('extractEmbeddedSubtitles')"
          :value-tooltip="enforcementLockTooltip('extractEmbeddedSubtitles')"
          @update:model-value="setField('extractEmbeddedSubtitles', $event)"
          @update:enforced="(v) => emitEnforced('extractEmbeddedSubtitles', v)"
        />
        <ToggleSwitch
          class="item"
          :model-value="modelValue.disableLoginOption"
          :name="$t('share.disableLoginOption')"
          :description="$t('share.disableLoginOptionDescription')"
          :disabled="fieldDisabled('disableLoginOption')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('disableLoginOption')"
          :enforcement-disabled="enforcementDisabled('disableLoginOption')"
          :enforcement-locked="isEnforcementLocked('disableLoginOption')"
          :value-tooltip="enforcementLockTooltip('disableLoginOption')"
          @update:model-value="setField('disableLoginOption', $event)"
          @update:enforced="(v) => emitEnforced('disableLoginOption', v)"
        />
      </div>

      <div v-if="modelValue.shareType === 'normal'">
        <div
          class="preference-field-block"
          :class="{ 'preference-field-block--enforceable': enforceable }"
        >
          <p>
            {{ $t("prompts.downloadsLimit") }}
            <i
              class="material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.downloadsLimitDescription'))"
              @mouseleave="hideTooltip"
            >
              help
            </i>
          </p>
          <input
            class="input"
            type="number"
            min="0"
            :value="modelValue.downloadsLimit"
            :disabled="fieldDisabled('downloadsLimit')"
            @input="setField('downloadsLimit', Number($event.target.value))"
          />
          <ProfileEnforceSwitch
            :visible="enforceable"
            :enforced="enforcedFlag('downloadsLimit')"
            @update:enforced="(v) => emitEnforced('downloadsLimit', v)"
          />
        </div>
        <div
          class="preference-field-block"
          :class="{ 'preference-field-block--enforceable': enforceable }"
        >
          <p>
            {{ $t("prompts.maxBandwidth") }}
            <i
              class="material-symbols-outlined tooltip-info-icon"
              @mouseenter="showTooltip($event, $t('share.maxBandwidthDescription'))"
              @mouseleave="hideTooltip"
            >
              help
            </i>
          </p>
          <input
            class="input"
            type="number"
            min="0"
            :value="modelValue.maxBandwidth"
            :disabled="fieldDisabled('maxBandwidth')"
            @input="setField('maxBandwidth', Number($event.target.value))"
          />
          <ProfileEnforceSwitch
            :visible="enforceable"
            :enforced="enforcedFlag('maxBandwidth')"
            @update:enforced="(v) => emitEnforced('maxBandwidth', v)"
          />
        </div>
      </div>

      <div>
        <ToggleSwitch
          class="item"
          :model-value="modelValue.quotaEnabled"
          :name="$t('quotas.shareLimit')"
          :description="$t('quotas.shareLimitDescription')"
          :disabled="fieldDisabled('quotaLimitBytes')"
          :enforceable="enforceable"
          :enforced="enforcedFlag('quotaLimitBytes')"
          :enforcement-disabled="enforcementDisabled('quotaLimitBytes')"
          :enforcement-locked="isEnforcementLocked('quotaLimitBytes')"
          :value-tooltip="enforcementLockTooltip('quotaLimitBytes')"
          @update:model-value="setField('quotaEnabled', $event)"
          @update:enforced="(v) => emitEnforced('quotaLimitBytes', v)"
        />
        <div v-if="modelValue.quotaEnabled" class="quota-share-fields">
          <p>{{ $t("general.limit") }}</p>
          <QuotaCustomLimitInput
            :amount="modelValue.quotaCustomAmount"
            :unit="modelValue.quotaCustomUnit"
            :aria-label="$t('general.limit')"
            @update:amount="setField('quotaCustomAmount', $event)"
            @update:unit="setField('quotaCustomUnit', $event)"
          />
          <ProgressBar
            v-if="showQuotaUsage && quotaLimitBytes > 0"
            :val="quotaUsedBytes"
            :val-background="quotaReservedBytes"
            :max="quotaLimitBytes"
            unit="bytes"
          />
        </div>
      </div>

      <div
        class="preference-field-block"
        :class="{ 'preference-field-block--enforceable': enforceable }"
      >
        <p>
          {{ $t("prompts.shareThemeColor") }}
          <i
            class="material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareThemeColorDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <input
          class="input"
          type="text"
          :value="modelValue.themeColor"
          :disabled="fieldDisabled('themeColor')"
          @input="setField('themeColor', $event.target.value)"
        />
        <ProfileEnforceSwitch
          :visible="enforceable"
          :enforced="enforcedFlag('themeColor')"
          @update:enforced="(v) => emitEnforced('themeColor', v)"
        />
      </div>

      <div
        class="preference-field-block"
        :class="{ 'preference-field-block--enforceable': enforceable }"
      >
        <p>
          {{ shareTitleLabel() }}
          <i
            class="material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareTitleDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <input
          class="input"
          type="text"
          :value="modelValue.title"
          :disabled="fieldDisabled('title')"
          @input="setField('title', $event.target.value)"
        />
        <ProfileEnforceSwitch
          :visible="enforceable"
          :enforced="enforcedFlag('title')"
          @update:enforced="(v) => emitEnforced('title', v)"
        />
      </div>

      <div
        class="preference-field-block"
        :class="{ 'preference-field-block--enforceable': enforceable }"
      >
        <p>
          {{ $t("prompts.shareDescription") }}
          <i
            class="material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareDescriptionHelp'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <textarea
          class="input"
          :value="modelValue.description"
          :disabled="fieldDisabled('description')"
          @input="setField('description', $event.target.value)"
        ></textarea>
        <ProfileEnforceSwitch
          :visible="enforceable"
          :enforced="enforcedFlag('description')"
          @update:enforced="(v) => emitEnforced('description', v)"
        />
      </div>

      <div
        class="preference-field-block"
        :class="{ 'preference-field-block--enforceable': enforceable }"
      >
        <p>
          {{ $t("prompts.shareBanner") }}
          <i
            class="material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareBannerDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <div class="file-picker-input-group">
          <input
            class="input file-picker-input"
            type="text"
            :value="modelValue.banner"
            :disabled="fieldDisabled('banner')"
            @input="setField('banner', $event.target.value)"
          />
          <div
            class="file-picker-button clickable"
            :title="$t('share.browseFiles')"
            @click="$emit('pick-banner')"
          >
            <i class="material-symbols">folder_open</i>
          </div>
        </div>
        <ProfileEnforceSwitch
          :visible="enforceable"
          :enforced="enforcedFlag('banner')"
          @update:enforced="(v) => emitEnforced('banner', v)"
        />
      </div>

      <div
        class="preference-field-block"
        :class="{ 'preference-field-block--enforceable': enforceable }"
      >
        <p>
          {{ $t("prompts.shareFavicon") }}
          <i
            class="material-symbols-outlined tooltip-info-icon"
            @mouseenter="showTooltip($event, $t('share.shareFaviconDescription'))"
            @mouseleave="hideTooltip"
          >
            help
          </i>
        </p>
        <div class="file-picker-input-group">
          <input
            class="input file-picker-input"
            type="text"
            :value="modelValue.favicon"
            :disabled="fieldDisabled('favicon')"
            @input="setField('favicon', $event.target.value)"
          />
          <div
            class="file-picker-button clickable"
            :title="$t('share.browseFiles')"
            @click="$emit('pick-favicon')"
          >
            <i class="material-symbols">folder_open</i>
          </div>
        </div>
        <ProfileEnforceSwitch
          :visible="enforceable"
          :enforced="enforcedFlag('favicon')"
          @update:enforced="(v) => emitEnforced('favicon', v)"
        />
      </div>
    </SettingsItem>
  </div>
</template>

<script>
import { getters, mutations } from "@/store";
import { globalVars } from "@/utils/constants";
import { bytesFromCustomAmount } from "@/utils/quotaUnits";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import SettingsItem from "@/components/settings/SettingsItem.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import QuotaCustomLimitInput from "@/components/settings/QuotaCustomLimitInput.vue";
import ProgressBar from "@/components/ProgressBar.vue";
import ProfileEnforceSwitch from "@/components/settings/ProfileEnforceSwitch.vue";

const READ_ONLY_RESTRICTED_FIELDS = new Set([
  "allowModify",
  "allowCreate",
  "allowDelete",
  "allowReplacements",
  "enableOnlyOffice",
]);

export default {
  name: "ShareOptionsForm",
  components: {
    ToggleSwitch,
    SettingsItem,
    ExpandDropdown,
    QuotaCustomLimitInput,
    ProgressBar,
    ProfileEnforceSwitch,
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
    readOnlySource: {
      type: Boolean,
      default: false,
    },
    showQuotaUsage: {
      type: Boolean,
      default: false,
    },
    quotaUsedBytes: {
      type: Number,
      default: 0,
    },
    quotaReservedBytes: {
      type: Number,
      default: 0,
    },
    showBasicOptions: {
      type: Boolean,
      default: true,
    },
  },
  emits: [
    "update:modelValue",
    "enforced-change",
    "customize-sidebar-links",
    "pick-banner",
    "pick-favicon",
    "change",
  ],
  data() {
    return {
      showMoreExpanded: false,
    };
  },
  computed: {
    createAllowed() {
      return this.modelValue.allowCreate;
    },
    quotaLimitBytes() {
      if (!this.modelValue.quotaEnabled) {
        return 0;
      }
      return bytesFromCustomAmount(
        this.modelValue.quotaCustomAmount,
        this.modelValue.quotaCustomUnit,
      );
    },
    onlyOfficeAvailable() {
      return globalVars.onlyOfficeUrl !== "";
    },
    availableThemes() {
      return globalVars.userSelectableThemes || {};
    },
    shareTypeOptions() {
      return [
        { value: "normal", label: this.$t("share.normalShare") },
        {
          value: "upload",
          label: this.$t("share.uploadShare"),
          disabled: this.readOnlySource,
        },
      ];
    },
    shareThemeOptions() {
      return Object.entries(this.availableThemes).map(([key, theme]) => ({
        value: key,
        label: String(key) === "default"
          ? this.$t("profileSettings.defaultThemeDescription")
          : `${key} - ${theme.description}`,
      }));
    },
    viewModeOptions() {
      return [
        { value: "normal", label: this.$t("buttons.normalView") },
        { value: "list", label: this.$t("buttons.listView") },
        { value: "gallery", label: this.$t("buttons.galleryView") },
      ];
    },
    enforceDarkLightModeOptions() {
      return [
        { value: "default", label: this.$t("share.default") },
        { value: "dark", label: this.$t("share.dark") },
        { value: "light", label: this.$t("share.light") },
      ];
    },
  },
  methods: {
    setField(field, value) {
      this.$emit("update:modelValue", { ...this.modelValue, [field]: value });
      this.$emit("change");
    },
    emitEnforced(field, value) {
      this.$emit("enforced-change", field, value);
    },
    enforcedFlag(field) {
      return !!this.enforced[field];
    },
    fieldLocked(field) {
      return !this.enforceable && !getters.isAdmin() && this.enforced[field];
    },
    fieldDisabled(field) {
      if (this.fieldLocked(field)) {
        return true;
      }
      if (this.readOnlySource && READ_ONLY_RESTRICTED_FIELDS.has(field)) {
        return true;
      }
      return false;
    },
    isEnforcementLocked(field) {
      if (getters.isAdmin() || this.enforceable) {
        return false;
      }
      return this.enforcedFlag(field);
    },
    enforcementDisabled(field) {
      return this.isEnforcementLocked(field);
    },
    enforcementLockTooltip(field) {
      if (this.isEnforcementLocked(field)) {
        return this.$t("profileSettings.enforcedByAdmin");
      }
      return "";
    },
    shareThemeLabel() {
      return this.$t("general.shareTheme");
    },
    shareTitleLabel() {
      return this.$t("general.shareTitle");
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
    validateExtensions(value) {
      const normalized = String(value || "").trim();
      if (normalized === "" || normalized === "*") {
        return true;
      }
      const parts = normalized.split(/\s+/);
      const extensionRegex = /^\.\w+$/;
      return parts.every((part) => extensionRegex.test(part));
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

.input {
  height: auto;
}

.file-picker-input-group {
  display: flex;
  gap: 0.5em;
  align-items: center;
  margin-bottom: 1em;
}

.file-picker-input {
  flex: 1;
  border-top-right-radius: 0 !important;
  border-bottom-right-radius: 0 !important;
}

.file-picker-button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 3em;
  height: 2.5em;
  background: var(--surfaceSecondary);
  border: 1px solid var(--borderColor);
  border-radius: var(--borderRadius);
  border-top-left-radius: 0;
  border-bottom-left-radius: 0;
  cursor: pointer;
}

.preference-field-block--enforceable {
  padding: 0.35em;
  border-radius: var(--borderRadius);
  margin-bottom: 0.5em;
}
</style>
