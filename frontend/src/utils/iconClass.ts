// Icons drawn from the outlined Material Symbols set; everything else uses the filled core font.
const OUTLINED_ICONS = new Set([
  "table_rows_narrow",
  "content_copy",
  "file_copy",
  "copy_all",
  "insert_drive_file",
]);

export function actionIconClass(icon: string, customIconClass?: string): string {
  if (customIconClass) return customIconClass;
  return OUTLINED_ICONS.has(icon) ? "material-symbols-outlined" : "material-symbols";
}
