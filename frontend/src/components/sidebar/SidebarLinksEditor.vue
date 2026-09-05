<template>
  <div :class="rootClass">
    <p v-if="!showAddForm && !yamlMode && !isSelectingPath">{{ contextDescription }}</p>

    <div v-if="!showAddForm && !isSelectingPath" class="settings-items sidebar-links-editor-header">
      <ToggleSwitch
        class="item"
        :model-value="yamlMode"
        :name="$t('settings.editAsYaml')"
        :disabled="disabled"
        @update:model-value="onYamlModeChange"
      />
    </div>

    <div v-if="yamlMode" class="yaml-editor-container">
      <YamlEditorPanel
        ref="yamlEditor"
        v-model="yamlText"
        fill
        @apply="applyYamlLinks"
        @cancel="onYamlModeChange(false)"
      />
    </div>

    <div v-else :class="bodyScrollClass">
    <!-- Existing Links List - only show when not in edit/add mode -->
    <div v-if="!showAddForm && !isSelectingPath" class="links-list">
      <h3>{{ $t('sidebar.currentLinks') }}</h3>
      <div v-if="displayEntries.length === 0" class="empty-state">
        <p>{{ $t('files.lonely') }}</p>
      </div>
      <div class="links-container">
        <div v-for="(entry, index) in displayEntries" :key="entryKey(entry, index)" :ref="el => linkItemRefs[index] = el"
          class="link-item input no-select" :class="{ 'dragging': draggingIndex === index }"
          @dragover.prevent="handleDragOver($event, index)" @drop="handleDrop($event, index)">
          <div draggable="true" @dragstart="handleDragStart($event, index)" @dragend="handleDragEnd"
            class="link-drag-handle">
            <i class="material-symbols">drag_indicator</i>
          </div>
          <div v-if="entry.link.category === 'divider'" class="link-icon">
            <i class="material-symbols">horizontal_rule</i>
          </div>
          <div v-else class="link-icon">
            <i :class="getIconClass(entry.link.icon)">{{ entry.link.icon }}</i>
          </div>
          <div class="link-details">
            <span class="link-name">{{ getLinkDisplayName(entry.link) }}</span>
            <span class="link-category">{{ getCategoryLabel(entry.link.category) }}</span>
          </div>
          <button
            type="button"
            class="action"
            @click="editLink(entry, index)"
            :disabled="disabled || isLinkEnforced(entry.link)"
            :aria-label="$t('general.edit')"
            :title="isLinkEnforced(entry.link) ? $t('profileSettings.enforcedByAdmin') : $t('general.edit')"
          >
            <i class="material-symbols">edit</i>
          </button>
          <button
            v-if="isItemDeletable(entry)"
            type="button"
            class="action"
            @click="removeLink(index)"
            :disabled="disabled || isLinkEnforced(entry.link)"
            :aria-label="$t('general.delete')"
            :title="isLinkEnforced(entry.link) ? $t('profileSettings.enforcedByAdmin') : $t('general.delete')"
          >
            <i class="material-symbols">delete</i>
          </button>
        </div>
      </div>
    </div>

    <div v-if="!showAddForm && context === 'user' && !isDefaultsMode" class="settings-items">
      <ToggleSwitch class="item" :modelValue="showToolsInSidebar"
        @update:modelValue="onShowToolsInSidebarChange"
        :name="$t('profileSettings.showToolsInSidebar')"
        :description="$t('profileSettings.showToolsInSidebarDescription')" />
    </div>

    <!-- Add New Link Section -->
    <div v-if="!showAddForm" class="add-link-section">
      <button
        type="button"
        @click="openAddLink"
        class="button button--flat button--blue add-link-button"
        :disabled="disabled"
      >
        <i class="material-symbols">add</i>
        {{ $t('sidebar.addNewLink') }}
      </button>
    </div>

    <!-- Add/Edit Link Form - replaces the list when active -->
    <div v-if="showAddForm" class="add-link-form">
      <!-- Path Browser for Source/Share Links - shown when selecting path -->
      <div v-if="isSelectingPath">
        <file-list ref="fileList" :browse-source="isSourceCategory(newLink.category) ? newLink.sourceName : null"
          :browse-share="newLink.category === 'share' ? getShareHash(newLink.target) : null"
          @update:selected="updateSelectedPath"></file-list>
      </div>

      <!-- Form fields - hidden when selecting path -->
      <div v-else>
        <h3>{{ editingIndex !== null ? $t('sidebar.editLink') : $t('sidebar.addNewLink') }}</h3>

        <div v-if="isDefaultsMode" class="settings-items">
          <ToggleSwitch
            class="item"
            enforceable
            :model-value="editMeta.enabled"
            :enforced="editMeta.enforced"
            :name="editMetaLinkName"
            :description="editMetaDescription"
            :disabled="disabled"
            :enforcement-disabled="disabled"
            @update:model-value="editMeta.enabled = $event"
            @update:enforced="editMeta.enforced = $event"
          />
        </div>

        <!-- Link Type Selection -->
        <template v-if="!isDefaultsMode">
        <p>{{ $t('sidebar.linkType') }}</p>
        <ExpandDropdown
          :model-value="linkTypeSelectValue"
          :options="linkTypeOptions"
          :default-placeholder-if-empty="$t('sidebar.selectLinkType')"
          :aria-label="$t('sidebar.linkType')"
          :disabled="disabled"
          @update:model-value="onLinkTypeChange"
        />
        </template>

        <!-- Source Selection (category is "source" or "source-minimal") -->
        <div v-if="isSourceCategory(newLink.category)" class="form-group">
          <template v-if="!isDefaultsMode">
          <p>{{ $t('sidebar.selectSource') }}</p>
          <ExpandDropdown
            v-model="newLink.sourceName"
            :options="sidebarSourceOptions"
            :default-placeholder-if-empty="$t('sidebar.chooseSource')"
            :aria-label="$t('sidebar.selectSource')"
            :disabled="disabled"
            @update:model-value="handleSourceChange"
          />
          </template>

          <!-- Custom Name for Source -->
          <div class="form-group" v-if="showSourceNameField">
            <p>{{ $t('sidebar.linkName') }}</p>
            <input aria-label="Link Name" v-model="newLink.name" type="text" class="input"
              :placeholder="$t('sidebar.linkNamePlaceholder')" :disabled="disabled" />
          </div>

          <!-- Path Selection for Source - clickable path display -->
          <div v-if="!isDefaultsMode && newLink.sourceName" class="padding-top">
            <div class="searchContext clickable button" @click="openPathBrowser('source')" aria-label="source-path">
              {{ $t('general.path', { suffix: ':' }) }} {{ newLink.sourcePath || '/' }}
            </div>
          </div>

          <!-- Source link options: Two toggles to control usage display type -->
          <div v-if="showSourceNameField" class="settings-items" style="margin-top: 0.5em;">
            <!-- Show usage from indexed files toggle -->
            <ToggleSwitch class="item"
              :modelValue="showIndexedUsage"
              @update:modelValue="updateUsageToggles('indexed', $event)"
              :name="$t('sidebar.showIndexedUsage')"
              :description="$t('sidebar.showIndexedUsageDescription')" />
            <!-- Show disk/partition usage toggle -->
            <ToggleSwitch class="item"
              :modelValue="showDiskUsage"
              @update:modelValue="updateUsageToggles('disk', $event)"
              :name="$t('sidebar.showDiskUsage')"
              :description="$t('sidebar.showDiskUsageDescription')" />
            
            <!-- Dropdown to choose which usage text to display (only shown in hybrid mode) -->
            <div v-if="showIndexedUsage && showDiskUsage" class="form-group" style="margin-top: 0.5em;">
              <p>{{ $t('sidebar.usageTextDisplay') }}</p>
              <ExpandDropdown
                :model-value="usageTextMode"
                :options="usageTextModeOptions"
                :aria-label="$t('sidebar.usageTextDisplay')"
                @update:model-value="updateUsageTextMode"
              />
            </div>
          </div>
        </div>

        <!-- Share Selection -->
        <div v-if="newLink.category === 'share'" class="form-group">
          <p>{{ $t('sidebar.selectShare') }}</p>
          <ExpandDropdown
            v-model="newLink.target"
            :options="shareTargetOptions"
            :default-placeholder-if-empty="$t('sidebar.chooseShare')"
            :aria-label="$t('sidebar.selectShare')"
            @update:model-value="handleShareChange"
          />

          <!-- Custom Name for Share -->
          <div class="form-group" v-if="newLink.target">
            <p>{{ $t('sidebar.linkName') }}</p>
            <input aria-label="Link Name" v-model="newLink.name" type="text" class="input"
              :placeholder="$t('sidebar.linkNamePlaceholder')" />
          </div>

          <!-- Path Selection for Share (subpath within the share) - clickable path display -->
          <div v-if="newLink.target">
            <div class="searchContext clickable button" @click="openPathBrowser('share')" aria-label="share-path">
              {{ $t('general.path', { suffix: ':' }) }} {{ getShareSubpath(newLink.target) }}
            </div>
          </div>
        </div>

        <!-- Tool Selection - only available for user context, not shares -->
        <div v-if="newLink.category === 'tool' && context === 'user'" class="form-group">
          <p>{{ $t('sidebar.selectTool') }}</p>
          <ExpandDropdown
            v-model="newLink.target"
            :options="toolTargetOptions"
            :default-placeholder-if-empty="$t('sidebar.chooseTool')"
            :aria-label="$t('sidebar.selectTool')"
            @update:model-value="handleToolChange"
          />

          <!-- Custom Name for Tool -->
          <div class="form-group" v-if="newLink.target">
            <p>{{ $t('sidebar.linkName') }}</p>
            <input aria-label="Link Name" v-model="newLink.name" type="text" class="input"
              :placeholder="$t('sidebar.linkNamePlaceholder')" />
          </div>
        </div>

        <!-- Share Info Link - special type for shares -->
        <div v-if="newLink.category === 'shareInfo'" class="form-group">
          <p>{{ $t('share.shareInfoDescription') }}</p>
        </div>

        <!-- Download Link - special type for shares -->
        <div v-if="newLink.category === 'download'" class="form-group">
          <p>{{ $t('share.downloadDescription') }}</p>
        </div>

        <!-- Divider - special visual separator -->
        <div v-if="newLink.category === 'divider'" class="form-group">          
          <p>{{ $t('sidebar.linkName') }}</p>
          <input aria-label="Link Name" v-model="newLink.name" type="text" class="input"
            :placeholder="$t('sidebar.dividerNamePlaceholder')" />
        </div>

        <!-- Custom Link Input -->
        <div v-if="newLink.category === 'custom'" class="form-group">
          <p>{{ $t('sidebar.linkName') }}</p>
          <input aria-label="Link Name" v-model="newLink.name" type="text" class="input"
            :placeholder="$t('sidebar.linkNamePlaceholder')" />

          <p>{{ $t('sidebar.linkUrl') }}</p>
          <input aria-label="Link Target" v-model="newLink.target" type="text" class="input"
            :placeholder="$t('sidebar.linkUrlPlaceholder')" />
        </div>

        <!-- Icon Selection - Available for ALL link types except divider -->
        <div v-if="newLink.category && newLink.category !== 'divider'" class="form-group">
          <p>{{ $t('sidebar.linkIcon') }}</p>
          <div class="icon-input-group">
            <input v-model="newLink.icon" type="text" class="input icon-input"
              :placeholder="$t('sidebar.linkIconPlaceholder')" />
            <div class="icon-preview clickable border-radius" @click="openIconPicker" :title="$t('sidebar.browseIcons')">
              <i v-if="newLink.icon" :class="getIconClass(newLink.icon)">{{ newLink.icon }}</i>
              <i v-else class="material-symbols icon-preview-placeholder">interests</i>
            </div>
          </div>
        </div>
      </div>
    </div>
    </div>
  </div>

  <div v-if="showCardActions" class="card-actions">
    <template v-if="yamlMode">
      <button
        type="button"
        class="button button--flat button--red"
        @click="exportYaml"
        :aria-label="$t('general.export')"
        :title="$t('general.export')"
      >
        <i class="material-symbols">upload</i>
        {{ $t('general.export') }}
      </button>
      <button
        type="button"
        class="button button--flat button--red"
        :disabled="disabled"
        @click="triggerYamlImport"
        :aria-label="$t('general.import')"
        :title="$t('general.import')"
      >
        <i class="material-symbols">download</i>
        {{ $t('general.import') }}
      </button>
      <input
        ref="yamlFileInput"
        type="file"
        accept=".yaml,.yml,text/yaml"
        class="hidden"
        @change="importYaml"
      />
      <span class="card-actions-spacer"></span>
      <button
        type="button"
        class="button button--flat button--grey"
        :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')"
        @click="onYamlModeChange(false)"
      >
        {{ $t("general.cancel") }}
      </button>
      <button
        type="button"
        class="button button--flat button--blue"
        :disabled="disabled"
        :aria-label="$t('general.save')"
        :title="$t('general.save')"
        @click="applyYamlFromEditor"
      >
        {{ $t("general.save") }}
      </button>
    </template>

    <!-- When selecting a path -->
    <template v-else-if="isSelectingPath">
      <button
        type="button"
        @click="cancelPathSelection"
        class="button button--flat button--grey"
        :aria-label="$t('general.cancel')"
        :title="$t('general.cancel')"
      >
        {{ $t("general.cancel") }}
      </button>
      <button
        type="button"
        @click="confirmPathSelection"
        class="button button--flat button--blue"
        :aria-label="$t('general.ok')"
        :title="$t('general.ok')"
      >
        {{ $t("general.ok") }}
      </button>
    </template>

    <!-- When in add/edit form mode -->
    <template v-else-if="showAddForm">
      <button
        type="button"
        @click="cancelAddLink"
        class="button button--flat button--grey"
        :aria-label="isDefaultsMode ? $t('general.back') : $t('general.cancel')"
        :title="isDefaultsMode ? $t('general.back') : $t('general.cancel')"
      >
        {{ isDefaultsMode ? $t("general.back") : $t("general.cancel") }}
      </button>
      <button
        type="button"
        aria-label="Add Link"
        @click="addLink"
        class="button button--flat button--blue"
        :disabled="!isNewLinkValid || disabled"
        :title="editingIndex !== null ? $t('general.save') : $t('general.add')"
      >
        {{ editingIndex !== null ? $t('general.save') : $t('general.add') }}
      </button>
    </template>

    <!-- When viewing the list -->
    <template v-else-if="showPromptActions">
      <button
        type="button"
        aria-label="Save Links"
        class="button button--flat button--blue"
        @click="saveLinks"
        :title="$t('general.save')"
      >
        {{ $t("general.save") }}
      </button>
    </template>
  </div>
</template>

<script>
import { state, getters, mutations } from "@/store";
import { notify } from "@/notify";
import { shareApi } from "@/api";
import { tools } from "@/utils/constants";
import { getIconClass } from "@/utils/material-symbols";
import { getObjectProperty } from '@/utils/object.js';
import FileList from "../files/FileList.vue";
import ToggleSwitch from "@/components/settings/ToggleSwitch.vue";
import ExpandDropdown from "@/components/settings/ExpandDropdown.vue";
import YamlEditorPanel from "@/components/prompts/YamlEditorPanel.vue";
import { sidebarLinkKey } from "@/utils/sidebarLinkKeys.js";
import yaml from "js-yaml";

export default {
  name: "SidebarLinksEditor",
  components: {
    FileList,
    ToggleSwitch,
    ExpandDropdown,
    YamlEditorPanel,
  },
  props: {
    mode: {
      type: String,
      default: "user", // 'user', 'share', or 'defaults'
    },
    modelValue: {
      type: Array,
      default: () => [],
    },
    shareData: {
      type: Object,
      default: null,
    },
    description: {
      type: String,
      default: "",
    },
    yamlDescription: {
      type: String,
      default: "",
    },
    disabled: {
      type: Boolean,
      default: false,
    },
    showPromptActions: {
      type: Boolean,
      default: true,
    },
    embedded: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:modelValue", "change", "save"],
  data() {
    return {
      links: [],
      showAddForm: false,
      newLink: {
        name: "",
        category: "",
        target: "",
        icon: "",
        sourceName: "",
        sourcePath: "",
      },
      draggingIndex: null,
      dragOverIndex: null,
      linkItemRefs: {},
      originalLinks: null, // Store original order in case drag is cancelled
      availableTools: [
        { name: 'tools.title', path: '/tools', icon: 'build' }, // Main tools page
        ...tools() // Individual tools
      ],
      availableShares: [],
      editingIndex: null,
      isSelectingPath: false,
      tempSelectedPath: "",
      tempSelectedSource: "",
      showToolsInSidebar: true,
      yamlMode: false,
      yamlText: "",
      editMeta: { enabled: true, enforced: false },
    };
  },
  computed: {
    context() {
      return this.mode === "defaults" ? "user" : this.mode;
    },
    isDefaultsMode() {
      return this.mode === "defaults";
    },
    displayEntries() {
      if (this.isDefaultsMode) {
        return Array.isArray(this.modelValue) ? this.modelValue : [];
      }
      return this.links.map((link) => ({ link }));
    },
    enforcedLinkKeys() {
      const items = state.sidebarLinkDefaultsPolicy?.items;
      if (!Array.isArray(items)) {
        return new Set();
      }
      return new Set(
        items
          .filter((item) => item?.enforced && item?.link)
          .map((item) => sidebarLinkKey(item.link))
      );
    },
    availableSources() {
      return state.sources?.info || {};
    },
    contextDescription() {
      if (this.description) {
        return this.description;
      }
      return this.context === "share"
        ? this.$t("sidebar.customizeShareLinksDescription")
        : this.$t("sidebar.customizeLinksDescription");
    },
    isNewLinkValid() {
      if (!this.newLink.category) return false;

      // Special link types that don't need additional validation
      if (this.newLink.category === "shareInfo" || 
          this.newLink.category === "download" || 
          this.newLink.category === "divider") {
        return true;
      }

      if (this.newLink.category === "custom") {
        return this.newLink.name && this.newLink.target;
      }

      if (this.isSourceCategory(this.newLink.category)) {
        return this.newLink.sourceName && this.newLink.name;
      }

      if (this.newLink.category === "share") {
        return this.newLink.target && this.newLink.name;
      }

      return this.newLink.target && this.newLink.name;
    },
    showIndexedUsage() {
      return this.newLink.category === 'source' || this.newLink.category === 'source-hybrid' || this.newLink.category === 'source-hybrid-2';
    },
    showDiskUsage() {
      return this.newLink.category === 'source-alt' || this.newLink.category === 'source-hybrid' || this.newLink.category === 'source-hybrid-2';
    },
    usageTextMode() {
      if (this.newLink.category === 'source-hybrid-2') {
        return 'disk';
      }
      return 'indexed';
    },
    /** Single "Source" row in the link-type dropdown; actual variant stays in newLink.category (toggles). */
    linkTypeSelectValue() {
      if (this.isSourceCategory(this.newLink.category)) {
        return 'source';
      }
      return this.newLink.category;
    },
    /** Share link type requires manage-share permission; avoids listing shares when forbidden */
    canListShares() {
      return this.context === "user" && getters.globalPermissions().share;
    },
    linkTypeOptions() {
      const options = [];
      if (this.context === "user") {
        options.push({ value: "source", label: this.$t("general.source") });
      }
      if (this.canListShares) {
        options.push({ value: "share", label: this.$t("general.share") });
      }
      if (this.context === "user") {
        options.push({ value: "tool", label: this.$t("general.tool") });
      }
      options.push(
        { value: "custom", label: this.$t("sidebar.customLink") },
        { value: "divider", label: this.$t("general.divider") },
      );
      if (this.context === "share") {
        options.push(
          { value: "shareInfo", label: this.$t("share.shareInfo") },
          { value: "download", label: this.$t("general.download") },
        );
      }
      return options;
    },
    sidebarSourceOptions() {
      return Object.keys(this.availableSources).map((name) => ({
        value: name,
        label: name,
      }));
    },
    shareTargetOptions() {
      return this.availableShares.map((share) => ({
        value: `/public/share/${share.hash}`,
        label: `${share.hash} ${this.$t("general.of")} ${share.path}`,
      }));
    },
    toolTargetOptions() {
      return this.availableTools.map((tool) => ({
        value: tool.path,
        label: this.$t(tool.name),
      }));
    },
    usageTextModeOptions() {
      return [
        { value: "indexed", label: this.$t("sidebar.usageTextIndexed") },
        { value: "disk", label: this.$t("sidebar.usageTextDisk") },
      ];
    },
    isEditingDefaultsSource() {
      return this.isDefaultsMode
        && this.editingIndex !== null
        && this.isSourceCategory(this.newLink.category);
    },
    showSourceNameField() {
      if (this.isDefaultsMode && this.isSourceCategory(this.newLink.category)) {
        return this.isEditingDefaultsSource || Boolean(this.newLink.sourceName);
      }
      return Boolean(this.newLink.sourceName);
    },
    editMetaLinkName() {
      return this.newLink.name
        || this.newLink.sourceName
        || this.newLink.target
        || this.$t("sidebar.customLink");
    },
    editMetaDescription() {
      if (this.isSourceCategory(this.newLink.category)) {
        return this.$t("sidebar.sidebarLinkDefaultSourceHelp");
      }
      const target = this.newLink.target || "/";
      return `${this.getCategoryLabel(this.newLink.category)} · ${target}`;
    },
    rootClass() {
      const classes = ["sidebar-links-content"];
      if (this.embedded || this.yamlMode) {
        classes.unshift("card-content", "prompt-panel");
      } else {
        classes.unshift("card-content");
      }
      return classes.join(" ");
    },
    bodyScrollClass() {
      return this.embedded ? "sidebar-links-body-scroll" : "";
    },
    showCardActions() {
      if (this.yamlMode) {
        return true;
      }
      if (this.isSelectingPath || this.showAddForm) {
        return true;
      }
      return this.showPromptActions;
    },
  },
  async mounted() {
    if (this.isDefaultsMode) {
      return;
    }

    // Initialize with existing sidebar links based on context
    if (this.context === 'share' && this.shareData?.sidebarLinks) {
      this.links = [...this.shareData.sidebarLinks];
    } else if (this.context === 'user' && state.user?.sidebarLinks && state.user?.sidebarLinks.length > 0) {
      this.links = [...state.user.sidebarLinks];
    } else if (this.context === 'user') {
      // Generate default links from sources for user context
      this.links = this.getDefaultLinks();
    }

    if (this.context === 'user') {
      if (typeof state.user?.showToolsInSidebar === 'boolean') {
        this.showToolsInSidebar = state.user?.showToolsInSidebar;
      } else {
        this.showToolsInSidebar = true;
      }
      this.syncMainToolsHubLinkRow(this.showToolsInSidebar);
    }

    // Share list loads lazily when adding/editing a share link (requires permission).
  },
  methods: {
    entryKey(entry, index) {
      return sidebarLinkKey(entry.link) || `item-${index}`;
    },
    isItemDeletable(entry) {
      if (!this.isDefaultsMode) {
        return true;
      }
      const cat = entry?.link?.category || "";
      return cat !== "source" && !cat.startsWith("source-");
    },
    cloneDefaultsItems(items) {
      return (items || []).map((item) => ({
        enabled: !!item.enabled,
        enforced: !!item.enforced,
        link: { ...item.link },
      }));
    },
    emitDefaultsUpdate(items) {
      const next = this.cloneDefaultsItems(items);
      this.$emit("update:modelValue", next);
      this.$emit("change", next);
    },
    getReorderSnapshot() {
      if (this.isDefaultsMode) {
        return this.cloneDefaultsItems(this.modelValue);
      }
      return [...this.links];
    },
    applyReorderSnapshot(snapshot) {
      if (this.isDefaultsMode) {
        this.emitDefaultsUpdate(snapshot);
        return;
      }
      this.links = snapshot;
    },
    /** Main Tools hub uses category "tool" and target "/tools"; keep in sync when the toggle changes. */
    isMainToolsHubLink(link) {
      return link?.category === "tool" && link?.target === "/tools";
    },
    syncMainToolsHubLinkRow(showHub) {
      if (this.context !== "user") return;
      if (!showHub) {
        this.links = this.links.filter((link) => !this.isMainToolsHubLink(link));
        return;
      }
      if (this.links.some((link) => this.isMainToolsHubLink(link))) {
        return;
      }
      const mainEntry = this.availableTools.find((t) => t.path === "/tools");
      const name = mainEntry ? this.$t(mainEntry.name) : this.$t("tools.title");
      const icon = mainEntry ? mainEntry.icon : "build";
      this.links.push({
        name,
        category: "tool",
        target: "/tools",
        icon,
      });
    },
    onShowToolsInSidebarChange(value) {
      this.showToolsInSidebar = value;
      this.syncMainToolsHubLinkRow(value);
    },
    isLinkEnforced(link) {
      if (this.context !== "user" || getters.isAdmin()) {
        return false;
      }
      return this.enforcedLinkKeys.has(sidebarLinkKey(link));
    },
    applyYamlFromEditor() {
      this.$refs.yamlEditor?.apply();
    },
    exportYaml() {
      const text = this.$refs.yamlEditor?.getValue() ?? this.yamlText;
      const blob = new Blob([text], { type: "text/yaml" });
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = this.isDefaultsMode ? "sidebar-link-defaults.yaml" : "sidebar-links.yaml";
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    },
    triggerYamlImport() {
      this.$refs.yamlFileInput?.click();
    },
    importYaml(event) {
      const file = event.target.files?.[0];
      event.target.value = ""; // to allow selecting the same file again
      if (!file) return;
      const reader = new FileReader();
      reader.onload = () => {
        this.yamlText = typeof reader.result === "string" ? reader.result : "";
      };
      reader.readAsText(file);
    },
    onYamlModeChange(enabled) {
      if (enabled) {
        const payload = this.isDefaultsMode ? this.defaultsYamlPayload() : this.links;
        this.yamlText = yaml.dump(payload, { lineWidth: 120, noRefs: true });
        this.yamlMode = true;
        return;
      }
      this.yamlMode = false;
    },
    defaultsYamlPayload() {
      return (this.modelValue || []).map((item) => ({
        enabled: !!item.enabled,
        enforced: !!item.enforced,
        link: this.linkForYamlDisplay(item.link),
      }));
    },
    linkForYamlDisplay(link) {
      const out = { ...link };
      if (!this.isSourceCategory(out.category)) {
        return out;
      }
      const sources = this.availableSources;
      if (out.sourceName && sources[out.sourceName]) {
        return out;
      }
      for (const [name, info] of Object.entries(sources)) {
        if (info?.path === out.sourceName || name === out.name) {
          out.sourceName = name;
          return out;
        }
      }
      return out;
    },
    applyYamlLinks(text) {
      try {
        const parsed = yaml.load(text);
        if (!Array.isArray(parsed)) {
          throw new Error("expected array");
        }
        if (this.isDefaultsMode) {
          const items = parsed.map((item) => this.parseYamlDefaultItem(item));
          this.emitDefaultsUpdate(items);
          this.yamlMode = false;
          return;
        }
        this.links = parsed.map((link) => this.parseYamlLink(link));
        this.yamlMode = false;
        void this.saveLinks();
      } catch (_e) {
        notify.showError(this.$t("sidebar.sidebarLinkDefaultsYamlInvalid"));
      }
    },
    parseYamlDefaultItem(item) {
      if (!item || typeof item !== "object" || Array.isArray(item)) {
        throw new Error("invalid defaults item");
      }
      return {
        enabled: !!item.enabled,
        enforced: !!item.enforced,
        link: this.parseYamlLink(item.link),
      };
    },
    parseYamlLink(link) {
      if (!link || typeof link !== "object" || Array.isArray(link)) {
        throw new Error("invalid link");
      }
      if (typeof link.category !== "string" || !link.category.trim()) {
        throw new Error("invalid link category");
      }
      return { ...link };
    },
    getIconClass,
    getShareHash(target) {
      // Extract hash from /public/share/<hash> or /public/share/<hash>/path
      if (!target) return '';
      const parts = target.split('/');
      // parts: ['', 'public', 'share', '<hash>', ...subpath]
      if (parts.length >= 4 && parts[1] === 'public' && parts[2] === 'share') {
        return parts[3];
      }
      return '';
    },
    getShareSubpath(target) {
      // Extract subpath from /public/share/<hash>/subpath
      if (!target) return '/';
      const parts = target.split('/');
      // parts: ['', 'public', 'share', '<hash>', ...subpath]
      if (parts.length >= 4 && parts[1] === 'public' && parts[2] === 'share') {
        return parts.length > 4 ? `/${parts.slice(4).join('/')}` : '/';
      }
      return '/';
    },
    openIconPicker() {
      mutations.showPrompt({
        name: "IconPicker",
        props: {
          onSelect: this.handleIconSelect,
        },
      });
    },
    handleIconSelect(iconName) {
      this.newLink.icon = iconName;
    },
    async loadAvailableShares() {
      if (!this.canListShares) {
        this.availableShares = [];
        return;
      }
      try {
        this.availableShares = await shareApi.list();
      } catch (error) {
        console.error("Failed to load shares:", error);
        this.availableShares = [];
      }
    },
    getDefaultLinks() {
      // Generate default links from sources
      const defaultLinks = [];

      if (this.availableSources) {
        Object.keys(this.availableSources).forEach(sourceName => {
          defaultLinks.push({
            name: sourceName,
            category: 'source',
            target: '/', // Relative path to source root
            icon: '', // No icon by default - will show animated status indicator
            sourceName: sourceName,
          });
        });
      }

      return defaultLinks;
    },
    isSourceCategory(category) {
      return category === 'source' || category === 'source-minimal' || category === 'source-alt' || category === 'source-hybrid' || category === 'source-hybrid-2';
    },
    updateUsageToggles(toggleType, value) {
      // Determine the new category based on toggle states
      // indexed=true, disk=false  -> 'source'
      // indexed=false, disk=true  -> 'source-alt'
      // indexed=true, disk=true   -> 'source-hybrid' or 'source-hybrid-2' (depends on usageTextMode)
      // indexed=false, disk=false -> 'source-minimal'
      
      const indexed = toggleType === 'indexed' ? value : this.showIndexedUsage;
      const disk = toggleType === 'disk' ? value : this.showDiskUsage;
      
      if (indexed && disk) {
        // Preserve the hybrid mode variant if it was already set
        if (this.newLink.category === 'source-hybrid-2') {
          this.newLink.category = 'source-hybrid-2';
        } else {
          this.newLink.category = 'source-hybrid';
        }
      } else if (indexed && !disk) {
        this.newLink.category = 'source';
      } else if (!indexed && disk) {
        this.newLink.category = 'source-alt';
      } else {
        this.newLink.category = 'source-minimal';
      }
    },
    updateUsageTextMode(mode) {
      if (mode === "disk") {
        this.newLink.category = 'source-hybrid-2';
      } else {
        this.newLink.category = 'source-hybrid';
      }
    },
    getCategoryLabel(category) {
      switch (category) {
        case 'source':
        case 'source-minimal':
        case 'source-alt':
        case 'source-hybrid':
        case 'source-hybrid-2':
          return this.$t('general.source');
        case 'tool':
          return this.$t('general.tool');
        case 'custom':
          return this.$t('sidebar.customLink');
        case 'share':
          return this.$t('general.share');
        case 'shareInfo':
          return this.$t('share.shareInfo');
        case 'download':
          return this.$t('general.download');
        case 'divider':
          return this.$t('general.divider');
        default:
          return category;
      }
    },
    getLinkDisplayName(link) {
      // For dividers, show the name or a default
      if (link.category === 'divider') {
        return link.name || this.$t('general.divider');
      }
      
      // Check if the name looks like a translation key that needs translating
      if (link.category === 'shareInfo' && link.name === 'share.shareInfo') {
        return this.$t('share.shareInfo');
      }
      if (link.category === 'download' && link.name === 'general.download') {
        return this.$t('general.download');
      }
      // Check if it's a general translation key pattern
      if (typeof link.name === 'string' && link.name.includes('.') && link.name.split('.').length === 2) {
        // Try to translate, if it fails, return original
        try {
          const translated = this.$t(link.name);
          // If translation returns the same key, it means it doesn't exist, return original
          return translated !== link.name ? translated : link.name;
        } catch (_e) {
          return link.name;
        }
      }
      return link.name;
    },
    resetNewLinkFormForTypeSwitch() {
      this.newLink.name = "";
      this.newLink.target = "";
      this.newLink.icon = "";
      this.newLink.sourceName = "";
      this.newLink.sourcePath = "";
    },
    onLinkTypeChange(value) {
      if (value === "source") {
        if (!this.isSourceCategory(this.newLink.category)) {
          this.resetNewLinkFormForTypeSwitch();
          this.newLink.category = "source";
        }
        return;
      }
      if (value !== this.newLink.category || this.isSourceCategory(this.newLink.category)) {
        this.resetNewLinkFormForTypeSwitch();
        this.newLink.category = value;
        if (value === "shareInfo") {
          this.newLink.name = this.$t("share.shareInfo");
          this.newLink.icon = "qr_code";
        } else if (value === "download") {
          this.newLink.name = this.$t("general.download");
          this.newLink.icon = "download";
        }
      }
      if (value === "share") {
        void this.loadAvailableShares();
      }
    },
    handleSourceChange() {
      if (this.newLink.sourceName) {
        // Only set default name if user hasn't entered one yet
        if (!this.newLink.name) {
          this.newLink.name = this.newLink.sourceName;
        }
        // No icon by default - will show animated status indicator
        if (!this.newLink.icon) {
          this.newLink.icon = "";
        }
        this.newLink.sourcePath = "/";
      }
    },
    handleShareChange() {
      if (this.newLink.target) {
        const hash = this.getShareHash(this.newLink.target);
        const share = this.availableShares.find(s => s.hash === hash);
        if (share) {
          // Only set default name if user hasn't entered one yet
          if (!this.newLink.name) {
            this.newLink.name = `Share: ${share.hash}`;
          }
          // Suggest default icon if not set
          if (!this.newLink.icon) {
            this.newLink.icon = "share";
          }
        }
      }
    },
    handleToolChange() {
      const tool = this.availableTools.find(t => t.path === this.newLink.target);
      if (tool) {
        // Only set default name and icon if user hasn't entered them yet
        if (!this.newLink.name) {
          this.newLink.name = this.$t(tool.name);
        }
        if (!this.newLink.icon) {
          this.newLink.icon = tool.icon;
        }
      }
    },
    openPathBrowser(type) {
      // Show file list for path browsing
      this.isSelectingPath = true;
      if (type === 'source') {
        this.tempSelectedPath = this.newLink.sourcePath || '/';
        this.tempSelectedSource = this.newLink.sourceName;
      } else if (type === 'share') {
        this.tempSelectedPath = this.getShareSubpath(this.newLink.target);
        this.tempSelectedSource = '';
      }
    },
    updateSelectedPath(pathOrData) {
      // Handle both old format (string) and new format (object with path and source)
      if (typeof pathOrData === 'string') {
        this.tempSelectedPath = pathOrData;
      } else if (pathOrData?.path) {
        this.tempSelectedPath = pathOrData.path;
        this.tempSelectedSource = pathOrData.source;
      }
    },
    confirmPathSelection() {
      // Apply the selected path to the link based on category
      if (this.isSourceCategory(this.newLink.category)) {
        this.newLink.sourcePath = this.tempSelectedPath;
      } else if (this.newLink.category === 'share') {
        // Update target with new subpath
        const hash = this.getShareHash(this.newLink.target);
        const subpath = this.tempSelectedPath === '/' ? '' : this.tempSelectedPath;
        this.newLink.target = `/public/share/${hash}${subpath}`;
      }
      this.isSelectingPath = false;
    },
    cancelPathSelection() {
      // Cancel path selection and return to form
      this.isSelectingPath = false;
      this.tempSelectedPath = "";
      this.tempSelectedSource = "";
    },
    openAddLink() {
      this.editingIndex = null;
      this.editMeta = { enabled: true, enforced: false };
      this.newLink = {
        name: "",
        category: this.isDefaultsMode ? "custom" : "",
        target: "",
        icon: "",
        sourceName: "",
        sourcePath: "",
      };
      this.showAddForm = true;
    },
    editLink(entry, index) {
      const link = entry?.link;
      if (!link || this.isLinkEnforced(link)) return;
      this.editingIndex = index;

      if (this.isDefaultsMode) {
        this.editMeta = {
          enabled: !!entry.enabled,
          enforced: !!entry.enforced,
        };
      }

      this.showAddForm = true;

      // Populate form with existing link data (category stays source or source-minimal)
      const isSource = this.isSourceCategory(link.category);
      this.newLink = {
        name: link.name,
        category: link.category,
        target: isSource ? "" : (link.target || ""),
        icon: link.icon || "",
        sourceName: link.sourceName || "",
        sourcePath: isSource ? (link.target || "/") : "/",
      };
      if (link.category === "share") {
        void this.loadAvailableShares();
      }
    },
    addLink() {
      if (!this.isNewLinkValid) return;

      // Build the link object based on category
      // Always include: name, category, target, icon, and conditionally sourceName/shareHash
      const linkData = {
        name: this.newLink.name,
        category: this.newLink.category,
        icon: this.newLink.icon,
        target: "",
      };

      // Process target and additional fields based on category
      if (this.newLink.category === "shareInfo") {
        // ShareInfo is a special action link
        linkData.target = "#";
      } else if (this.newLink.category === "download") {
        // Download is a special action link
        linkData.target = "#";
      } else if (this.newLink.category === "divider") {
        // Divider is a visual separator with no action
        linkData.target = "#";
        linkData.name = this.newLink.name || ""; // Keep the custom name if provided
        linkData.icon = ""; // Dividers don't need an icon
      } else if (this.newLink.category === "custom") {
        linkData.target = this.processCustomUrl(this.newLink.target);
      } else if (this.isSourceCategory(this.newLink.category)) {
        // For sources: target is relative path, sourceName identifies the source; category is "source" or "source-minimal"
        linkData.category = this.newLink.category;
        linkData.target = this.newLink.sourcePath || '/';
        linkData.sourceName = this.newLink.sourceName;
      } else if (this.newLink.category === "share") {
        // For shares: target is already the full path /public/share/<hash>/<subpath>
        linkData.target = this.newLink.target;
      } else if (this.newLink.category === "tool") {
        linkData.target = this.newLink.target;
      }

      if (this.editingIndex !== null) {
        if (this.isDefaultsMode) {
          const items = this.cloneDefaultsItems(this.modelValue);
          items[this.editingIndex] = {
            enabled: this.editMeta.enabled,
            enforced: this.editMeta.enforced,
            link: linkData,
          };
          this.emitDefaultsUpdate(items);
        } else {
          this.links[this.editingIndex] = linkData;
        }
      } else if (this.isDefaultsMode) {
        const items = this.cloneDefaultsItems(this.modelValue);
        items.push({
          enabled: this.editMeta.enabled,
          enforced: this.editMeta.enforced,
          link: linkData,
        });
        this.emitDefaultsUpdate(items);
      } else {
        this.links.push(linkData);
      }

      // Close the form and return to list view
      this.cancelAddLink();
    },
    processCustomUrl(url) {      
      // Check if it's an external URL (case insensitive)
      const lowerUrl = url.toLowerCase();
      if (lowerUrl.startsWith('http://') || lowerUrl.startsWith('https://')) {
        // Leave external URLs as-is
        return url;
      }
      // For internal paths, normalize by adding a starting slash if missing
      if (!url.startsWith('/')) {
        return `/${url}`; // assume relative path
      }
      return url;
    },
    removeLink(index) {
      if (this.isDefaultsMode) {
        const items = this.cloneDefaultsItems(this.modelValue);
        items.splice(index, 1);
        this.emitDefaultsUpdate(items);
        return;
      }
      this.links.splice(index, 1);
    },
    cancelAddLink() {
      this.showAddForm = false;
      this.editingIndex = null;
      this.editMeta = { enabled: true, enforced: false };
      this.newLink = {
        name: "",
        category: "",
        target: "",
        icon: "",
        sourceName: "",
        sourcePath: "",
      };
    },
    handleDragStart(event, index) {
      this.draggingIndex = index;
      this.dragOverIndex = null;

      // Store original order in case drag is cancelled
      this.originalLinks = this.getReorderSnapshot();

      event.dataTransfer.effectAllowed = "move";
      event.dataTransfer.setData("text/html", event.target);

      // Set the entire link item as the drag image
      const linkItem = getObjectProperty(this.linkItemRefs, index);
      if (linkItem) {
        // Create a clone for the drag image to avoid affecting the original
        const clone = linkItem.cloneNode(true);
        clone.style.position = 'fixed';
        clone.style.top = '-9999px';
        clone.style.left = '-9999px';

        // Set the clone width to match the original element's width
        const originalWidth = linkItem.offsetWidth;
        clone.style.width = `${originalWidth}px`;
        linkItem.parentNode.insertBefore(clone, linkItem.nextSibling);

        // Set it as the drag image
        event.dataTransfer.setDragImage(clone, event.offsetX, event.offsetY);

        // Clean up the clone after a brief delay
        setTimeout(() => {
          clone.remove();
        }, 0);
      }
    },
    handleDragOver(_event, index) {
      if (this.draggingIndex === null || this.draggingIndex === index) return;

      // Only reorder if we're hovering over a different item
      if (this.dragOverIndex !== index) {
        this.dragOverIndex = index;

        // Live reorder: move the dragged item to the new position
        const newLinks = [...this.getReorderSnapshot()];
        const draggedLink = newLinks[this.draggingIndex];

        // Remove from current position
        newLinks.splice(this.draggingIndex, 1);

        // Insert at hover position
        newLinks.splice(index, 0, draggedLink);

        // Update the array and dragging index
        this.applyReorderSnapshot(newLinks);
        this.draggingIndex = index; // Update to new position
      }
    },
    handleDrop(event) {
      event.preventDefault();

      // The array is already in the correct order from handleDragOver
      // Just clean up the drag state
      this.draggingIndex = null;
      this.dragOverIndex = null;
      this.originalLinks = null; // Clear the backup
    },
    handleDragEnd() {
      // If drag was cancelled (no drop event), restore original order
      if (this.originalLinks !== null) {
        this.applyReorderSnapshot(this.originalLinks);
        this.originalLinks = null;
      }

      this.draggingIndex = null;
      this.dragOverIndex = null;
    },
    async saveLinks() {
      this.$emit("save", {
        links: [...this.links],
        showToolsInSidebar: this.showToolsInSidebar,
      });
    },
  },
};
</script>

<style scoped>

.settings-items {
  margin-top: 0.5em;
  margin-bottom: 0.5em;
}

.padding-top {
  margin-top: 0.5em;
}

.links-list h3,
.add-link-form h3 {
  margin-bottom: 0.5em;
  font-size: 1em;
  font-weight: 600;
}

.empty-state {
  padding: 2em 1em;
  text-align: center;
  color: var(--textSecondary);
  font-style: italic;
}

.add-link-form h3 {
  margin-top: 0;
  margin-bottom: 0.75em;
  font-size: 0.95em;
}

.links-container {
  display: flex;
  flex-direction: column;
  padding-bottom: 0.5em;
  gap: 0.5em;
}

/* Link item styles */
.link-item {
  display: flex;
  align-items: center;
  gap: 0.5em;
  background: var(--surfaceSecondary);
  transition: all 0.2s ease;
}

.link-item.dragging {
  opacity: 0.5;
  border-color: var(--primaryColor);
  background: var(--surfaceTertiary);
}

.link-drag-handle {
  color: var(--textSecondary);
  cursor: grab;
}

.link-drag-handle:active {
  cursor: grabbing;
}

.link-icon {
  color: var(--primaryColor);
}

.link-details {
  display: flex;
  flex-direction: column;
  gap: 0.25em;
  flex-grow: 1;
  width: 100%;
}

.link-name {
  font-weight: 500;
}

.link-category {
  font-size: 0.85em;
  color: var(--textSecondary);
}

.add-link-section {
  display: flex;
  flex-direction: column;
  gap: 0.5em;
}

.add-link-button {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5em;
}

.add-link-form {
  padding: 0;
  margin-top: 0;
}

.form-group p {
  margin: 0.5em
}

.form-group p:first-of-type,
.add-link-form>p:first-of-type {
  margin-top: 0;
}

/* Icon preview styles */
.icon-input-group {
  display: flex;
  align-items: center;
  gap: 0.5em;
}

.icon-input {
  flex: 1;
}

.sidebar-links-content.prompt-panel {
  display: flex;
  flex-direction: column;
  gap: 0.75em;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.sidebar-links-editor-header {
  flex: 0 0 auto;
}

.yaml-editor-container {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.sidebar-links-body-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  overscroll-behavior: contain;
}

</style>
