<template>
  <div class="card-content no-buttons user-defaults-prompt">
    <div v-if="loading" class="loading-hint">{{ t("general.loading") }}</div>

    <template v-else>
      <UserProfilePreferences
          v-model="preferenceSections"
          enforceable
          hide-account-options
          :enforced="enforced"
          :config-locked-paths="lockedFromConfigPaths"
          show-extension-inputs
          :show-thumbnail-master="false"
          @changed="onPreferenceSectionChange"
          @enforced-change="onEnforcedChange"
        />
        <UserDefaultsAccountSection
          :account="values.account"
          :enforced="enforced.account || {}"
          :enforced-permissions="enforced.account?.permissions || {}"
          :config-locked-paths="lockedFromConfigPaths"
          :show-require-password-change="passwordAccountDefaults"
          @account-change="({ field, value }) => onAccountFieldChange(field, value)"
          @enforced-change="({ field, value }) => patchEnforcedFlag('account', field, value)"
          @enforced-permission-change="({ field, value }) => patchEnforcedFlag('account', `permissions.${field}`, value)"
        />
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
import { useI18n } from "@/i18n/index.ts";
import { notify } from "@/notify/index.ts";
import { getUserDefaults, patchUserDefaults } from "@/api/settings.ts";
import { getObjectProperty } from "@/utils/object.js";
import UserDefaultsAccountSection from "@/components/settings/UserDefaultsAccountSection.vue";
import UserProfilePreferences from "@/components/settings/UserProfilePreferences.vue";
import type {
  UserDefaults,
  UserDefaultsDraft,
  UserDefaultsEnforcement,
  UserDefaultsPatch,
  UserDefaultsResponse,
} from "@/types/settings.ts";

type DefaultsSection = keyof UserDefaults;
type DefaultsValue = string | number | boolean | string[] | null;

defineProps<{
  promptId?: string | number | null;
}>();

const PERMISSIONS_PREFIX = "permissions.";

const { t } = useI18n();

function emptySections() {
  return {
    sidebar: {},
    listing: {},
    preview: {},
    fileViewer: {},
    search: {},
    ui: {},
    account: { permissions: {} },
    fileLoading: {},
  };
}

const loading = ref(true);
const saving = ref(false);
const hydrating = ref(false);
const lockedFromConfigPaths = ref<string[]>([]);
const values = ref<UserDefaultsDraft>(emptySections());
const enforced = ref<UserDefaultsEnforcement>(emptySections());

const passwordAccountDefaults = computed(() => {
  const method = values.value.account.loginMethod;
  return !method || method === "password";
});

const preferenceSections = computed<UserDefaultsDraft>({
  get() {
    const v = values.value;
    return {
      sidebar: { ...v.sidebar },
      listing: { ...v.listing },
      preview: { ...v.preview },
      fileViewer: { ...v.fileViewer },
      search: { ...v.search },
      ui: { ...v.ui },
      account: { ...v.account, permissions: { ...v.account.permissions } },
      fileLoading: { ...v.fileLoading },
    };
  },
  set(sections) {
    values.value = {
      ...values.value,
      sidebar: { ...sections.sidebar },
      listing: { ...sections.listing },
      preview: { ...sections.preview },
      fileViewer: { ...sections.fileViewer },
      search: { ...sections.search },
      ui: { ...sections.ui },
      fileLoading: { ...sections.fileLoading },
    };
  },
});

function applyResponse(data: UserDefaultsResponse): void {
  lockedFromConfigPaths.value = data.lockedFromConfigPaths ?? [];
  const v = data.values;
  const enf = data.enforced;
  values.value = {
    sidebar: { ...v?.sidebar },
    listing: { ...v?.listing },
    preview: {
      image: v?.preview.image ?? true,
      video: v?.preview.video ?? true,
      audio: v?.preview.audio ?? true,
      office: v?.preview.office ?? true,
      folder: v?.preview.folder ?? true,
      models: v?.preview.models ?? true,
      popup: v?.preview.popup ?? true,
      motionVideoPreview: v?.preview.motionVideoPreview ?? true,
      disablePreviewExt: v?.preview.disablePreviewExt || "",
    },
    fileViewer: { ...v?.fileViewer },
    search: { ...v?.search },
    ui: { ...v?.ui },
    account: { ...v?.account, permissions: { ...v?.account.permissions } },
    fileLoading: { ...v?.fileLoading },
  };
  enforced.value = {
    sidebar: { ...enf.sidebar },
    listing: { ...enf.listing },
    preview: { ...enf.preview },
    fileViewer: { ...enf.fileViewer },
    search: { ...enf.search },
    ui: { ...enf.ui },
    account: { ...enf.account, permissions: { ...enf.account?.permissions } },
    fileLoading: { ...enf.fileLoading },
  };
}

function canPatch(): boolean {
  return !loading.value && !saving.value && !hydrating.value;
}

function isConfigLockedPath(path: string): boolean {
  return lockedFromConfigPaths.value.includes(path);
}

function reportError(e: unknown): void {
  console.error(e);
  if (e instanceof Error && e.message) {
    notify.showError(e.message);
  }
}

async function load(): Promise<void> {
  loading.value = true;
  hydrating.value = true;
  try {
    applyResponse(await getUserDefaults());
  } catch (e) {
    reportError(e);
  } finally {
    loading.value = false;
    await nextTick();
    hydrating.value = false;
  }
}

async function sendPatch(partial: UserDefaultsPatch): Promise<void> {
  if (!canPatch()) {
    return;
  }
  saving.value = true;
  try {
    await patchUserDefaults(partial);
  } catch (e) {
    reportError(e);
    await load();
  } finally {
    saving.value = false;
  }
}

function patchEnforcedFlag(section: DefaultsSection, key: string, value: boolean): void {
  if (section === "account" && key.startsWith(PERMISSIONS_PREFIX)) {
    const permKey = key.slice(PERMISSIONS_PREFIX.length);
    void sendPatch({ enforced: { account: { permissions: { [permKey]: value } } } });
    return;
  }
  void sendPatch({ enforced: { [section]: { [key]: value } } });
}

function onEnforcedChange({ section, field, value }: { section: DefaultsSection; field: string; value: boolean }): void {
  patchEnforcedFlag(section, field, value);
}

function onPreferenceSectionChange({ section, field }: { section: DefaultsSection; field: string }): void {
  if (!section || !field || isConfigLockedPath(`${section}.${field}`)) {
    return;
  }
  const sectionData = getObjectProperty(values.value, section) as Record<string, DefaultsValue> | undefined;
  if (!sectionData) {
    return;
  }
  void sendPatch({ [section]: { [field]: getObjectProperty(sectionData, field) as DefaultsValue } });
}

function onAccountFieldChange(field: string, value: DefaultsValue): void {
  if (!canPatch() || !field || isConfigLockedPath(`account.${field}`)) {
    return;
  }
  const { account } = values.value;
  if (field.startsWith(PERMISSIONS_PREFIX)) {
    const permKey = field.slice(PERMISSIONS_PREFIX.length);
    account.permissions = { ...account.permissions, [permKey]: value };
    void sendPatch({ account: { permissions: { [permKey]: value } } });
    return;
  }
  Object.assign(account, { [field]: value });
  void sendPatch({ account: { [field]: value } });
}

onMounted(() => {
  void load();
});
</script>

<style scoped>
.loading-hint {
  opacity: 0.7;
}

.user-defaults-prompt :deep(.settings-group) {
  margin-bottom: 0.75rem;
}
</style>
