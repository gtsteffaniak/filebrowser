import type Plyr from "plyr";

/**
 * Plyr builds a settings menu at construction time but its typings omit
 * `elements.settings`. Since this is plyr-specific should go away with the Video.js 10 migration.
 */
export interface PlayerSettingsElements {
  menu?: HTMLElement;
  buttons?: Record<string, HTMLElement | undefined>;
  panels?: Record<string, HTMLElement | undefined>;
}

export function getSettingsElements(player: Plyr): PlayerSettingsElements | undefined {
  return (player.elements as unknown as { settings?: PlayerSettingsElements }).settings;
}

/** Whether Plyr's settings menu is open. */
export function isSettingsMenuOpen(player: Plyr | null | undefined): boolean {
  const menu = player ? getSettingsElements(player)?.menu : undefined;
  return Boolean(menu && menu.style.display !== "none" && menu.getAttribute("hidden") === null);
}

/** Plyr's resolved config (typed as private); used for the i18n strings of the custom menus. */
export function getPlayerI18n(player: Plyr): Record<string, string | undefined> | undefined {
  return (player as unknown as { config?: { i18n?: Record<string, string | undefined> } }).config?.i18n;
}

/** Seek step (seconds) the player was configured with, as used by rewind/fast-forward. */
export function getPlayerSeekTime(player: Plyr): number | undefined {
  return (player as unknown as { config?: { seekTime?: number } }).config?.seekTime;
}
