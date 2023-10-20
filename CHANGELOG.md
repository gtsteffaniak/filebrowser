# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

# v0.2.2
  - Uses more efficient indexing
  - Uses the index for filebrowser listings as well
  - hidden files still accessible directly.
  - editor fixed on save and themes
  - running-config.yaml gets updated on settings change so running settings

# v0.2.1
  - issue #29 - Rules can now be configured and read from configuration yaml
  - issue #28 - Allow disable settings per user.
  - issue #27 - shorten download link for password protected files
  - issue #26 - enable dark mode per user and improve switching performance.
  - More rounded corners and improved listing styling
  - improve search performance
  - fixes authentication issues
  - adds compact view mode
  - improves view mode configuration and behavior
  - updates configuration file to accept new settings

# v0.2.0
 - improved UI
   - more unified coehisive look
   - Adjusted header bar look and icon behavior
 - The shell is dead.
   - If you need to use custom commands, exec into the docker container.
 - The json config file is dead.
   - All configuration is done via advanced `filebrowser.yaml`
   - The only flag that is allowed is flag to specify config file.
 - Removed old code to migrate database versions
 - Removed all unused cmd code

# v0.1.4
 - various UI fixes
   - Added download button back to toolbar
   - Added upload button to side menu
   - breadcrumb spacing fix
   - Added "compact" view option
   - fixed slash issue with css rtl logic
 - various backend fixes
   - search has a sessionId attached so searches don't collide
   - search no longer searches by word with spaces, includes space in searches
   - prepared for full json configuration
 - made size search work for smaller and larger
 - made search types not show up in search bar when used

## v0.1.3

 - improved styling, colors, transparency, blur
 - Made sidebar hidden on desktop as well
 - simplified navbar to be three buttons
   - open menu
   - search
   - toggle view
 - Changed desktop search style and included additional search options.

## v0.1.2

 - Updated UI to use search features better
   - More filter options
   - Better icons with colors
   - GUI styling
 - Improved search performance

## v0.1.1

 - Improved search with indexing

## v0.1.0

 - nothing changed from origin.

Forked from https://github.com/filebrowser/filebrowser
