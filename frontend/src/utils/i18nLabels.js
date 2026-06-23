/** Join two translated labels with a space (each key is static for lint). */
function joinLabels($t, firstKey, secondKey) {
  return `${$t(firstKey)} ${$t(secondKey)}`;
}

export function pinnedFoldersLabel($t) {
  return joinLabels($t, "general.pinned", "general.folders");
}

export function pinnedFilesLabel($t) {
  return joinLabels($t, "general.pinned", "general.files");
}

export function pinnedItemsLabel($t) {
  return $t("general.pinned");
}

export function itemsSelectedLabel($t, count) {
  if (count === 1) {
    return joinLabels($t, "general.item", "general.selected");
  }
  return joinLabels($t, "general.items", "general.selected");
}

export function newUserLabel($t) {
  return joinLabels($t, "general.new", "general.user");
}

export function newFolderLabel($t) {
  return joinLabels($t, "general.new", "general.folder");
}

export function profileSettingsLabel($t) {
  return joinLabels($t, "general.profile", "general.settings");
}

export function shareSettingsLabel($t) {
  return joinLabels($t, "general.share", "general.settings");
}

export function globalSettingsLabel($t) {
  return joinLabels($t, "general.global", "general.settings");
}

export function userManagementLabel($t) {
  return joinLabels($t, "general.user", "general.management");
}

export function shareManagementLabel($t) {
  return joinLabels($t, "general.share", "general.management");
}

export function shareHashLabel($t) {
  return joinLabels($t, "general.share", "general.hash");
}

export function downloadFilesPermissionLabel($t) {
  return joinLabels($t, "general.download", "general.files");
}

export function editFilesPermissionLabel($t) {
  return joinLabels($t, "general.edit", "general.files");
}

export function createFilesPermissionLabel($t) {
  return joinLabels($t, "general.create", "general.files");
}

export function deleteFilesPermissionLabel($t) {
  return joinLabels($t, "general.delete", "general.files");
}

export function shareFilesPermissionLabel($t) {
  return joinLabels($t, "general.share", "general.files");
}

export function downloadFilesLabel($t) {
  return downloadFilesPermissionLabel($t);
}

export function shareThemeLabel($t) {
  return joinLabels($t, "general.share", "general.theme");
}

export function shareTitleLabel($t) {
  return joinLabels($t, "general.share", "general.title");
}

export function timeUnitLabel($t) {
  return joinLabels($t, "time.time", "time.unit");
}
