export function pinnedFoldersLabel($t) {
  return $t("general.pinnedFolders");
}

export function pinnedFilesLabel($t) {
  return $t("general.pinnedFiles");
}

export function pinnedItemsLabel($t) {
  return $t("general.pinned");
}

export function itemsSelectedLabel($t, count) {
  if (count === 1) {
    return $t("general.selectionSingle");
  }
  return $t("general.selectionMultiple");
}

export function newUserLabel($t) {
  return $t("general.newUser");
}

export function newFolderLabel($t) {
  return $t("general.newFolder");
}

export function profileSettingsLabel($t) {
  return $t("general.profileSettings");
}

export function shareSettingsLabel($t) {
  return $t("general.shareSettings");
}

export function globalSettingsLabel($t) {
  return $t("general.globalSettings");
}

export function userManagementLabel($t) {
  return $t("general.userManagement");
}

export function shareManagementLabel($t) {
  return $t("general.shareManagement");
}

export function shareHashLabel($t) {
  return $t("general.shareHash");
}

export function downloadFilesPermissionLabel($t) {
  return $t("general.downloadFiles");
}

export function editFilesPermissionLabel($t) {
  return $t("general.editFiles");
}

export function createFilesPermissionLabel($t) {
  return $t("general.createFiles");
}

export function deleteFilesPermissionLabel($t) {
  return $t("general.deleteFiles");
}

export function shareFilesPermissionLabel($t) {
  return $t("general.shareFiles");
}

export function downloadFilesLabel($t) {
  return downloadFilesPermissionLabel($t);
}

export function shareThemeLabel($t) {
  return $t("general.shareTheme");
}

export function shareTitleLabel($t) {
  return $t("general.shareTitle");
}

export function timeUnitLabel($t) {
  return $t("time.timeUnit");
}
