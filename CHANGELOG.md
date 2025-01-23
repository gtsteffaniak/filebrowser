# Changelog

All notable changes to this project will be documented in this file. For commit guidelines, please refer to [Standard Version](https://github.com/conventional-changelog/standard-version).

## v0.4.1-beta

  **New Features**
  - right-click actions are available on search window.
  - office file previews

  **Notes**
  - delete prompt lists all items that will be affected by delete

  **Bugfixes**:
  - when closing/going back on onlyoffice document a refresh was needed.
  - calculating checksums would always error.

## v0.4.0-beta

  **New Features**
  - Better logging https://github.com/gtsteffaniak/filebrowser/issues/288
    - highly configurable
    - api logs include user
  - onlyOffice support for editing only office files (inspired from https://github.com/filebrowser/filebrowser/pull/2954)

  **Notes**
  - Breadcrumbs will only show on file listing (not on previews or editors)
  - Config file is now optional. It will run with default settings without one and throw a `[WARN ]` message.
  - Added more descriptions to swagger API

## v0.3.7-beta

  **Notes**:
  - Adding windows builds back to automated process... will replace manually if they throw malicious defender warnings.
  - Adding playwright tests to all pr's against dev/beta/release branches.
    - These playwright tests should help keep release more reliably stable.

  **Bugfixes**:
  - closing with the default bar issue.
  - tar.gz archive creation issue

## v0.3.6-beta

  **New Features**
  - Adds "externalUrl" server config https://github.com/gtsteffaniak/filebrowser/issues/272

  **Notes**:
  - All views modes to show header bar for sorting.
  - other small style changes

  **Bugfixes**:
  - select and info bug after sorting https://github.com/gtsteffaniak/filebrowser/issues/277
  - downloading from shares with public user
  - Ctrl and Shift key modifiers work on listing views as expected.
  - copy/move file/folder error and show errors https://github.com/gtsteffaniak/filebrowser/issues/278
  - file move/copy context fix.

## v0.3.5

  **New Features**
  - More indexing configuration options possible. However consider waiting on using this feature, because I will soon have a full onboarding experience in the UI to manage sources instead.
    - added config file options "sources" in the server config.
    - can enable/disable indexing a specified list of directories/files
    - can enable/disable indexing hidden files
    - prepped for multiple sources (not supported yet!)
  - Theme and Branding support (see updates to [configuration wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Configuration) on how to use)
  - Automatically expire shares https://github.com/gtsteffaniak/filebrowser/issues/208

  **Notes**:
  - MacOS application files (ending in ".app") were previously treated as folders, now they are treated as a single file.
  - No longer indexes "$RECYCLE.BIN" or "System Volume Information" directories.
  - Icon styling tweaked so all icons have a background.
  - Updated Login page styling.
  - Settings profile menu has been simplified, password changes happen in user management.
  - Improved windows compatibility and built on windows platform to fix false windows defender warning.
  - If no "root" location is provided in the server config, the default is the **current directory** (rather than `/srv` like before)

  **Bugfixes**:
  - Fixed setting share expiration time would not work due to type conversion error.
  - More safari fixes related to text-selection.
  - Sort by name value sorting ignores the extension, only sorts by name https://github.com/gtsteffaniak/filebrowser/issues/230
  - Fixed manual language selection issue.
  - Fixed exact date time issue.

New login page:

<img width="300" alt="image" src="https://github.com/user-attachments/assets/d3ed359e-a969-4f6a-9f72-94d2b68aba49" />


Example branding in sidebar:

<img width="500" alt="image2" src="https://github.com/user-attachments/assets/d8ee14ca-4495-4106-9d26-631a5937e134" />

Example user settings page:

<img width="500" alt="image3" src="https://github.com/user-attachments/assets/79757a11-669e-4597-bd3d-e41efd667a1e" />

## v0.3.4

  **Bugfixes**:
  - Safari right-click actions.
  - Some small image viewer behavior
  - Progressive webapp "install to homescreen" fix.

## v0.3.3

  **New Features**
  - Navigating remembers your previous scroll position when opening items and then navigating backwards.
  - New Icons with larger selection of file types
  - file "type" is shown on item info page.
  - added optional non-root "filebrowser" user for docker image. See https://github.com/gtsteffaniak/filebrowser/issues/251
  - File preview supports more file types:
    - images: jpg, bmp, gif, tiff, png, svg, heic, webp

  **Notes**:
  - The file "type" is now either "directory" or a specific mimetype such as "text/xml".
  - update safari styling

  **Bugfixes**:
  - Delete/move file/folders sometimes wouldn't work.
  - Possible fix for context menu not showing issue. See https://github.com/gtsteffaniak/filebrowser/issues/251
  - Fixed drag/drop not refreshing immediately to reflect changes.

## v0.3.2

  **New Features**
  - Mobile search has the same features as desktop.

  **Notes**:
  - Added compression. Helpful for browsing folders with a large number of items. Considering https://github.com/gtsteffaniak/filebrowser/issues/201 resolved, although future pagination support will still come.
  - Compressed download options limited to `.zip` and `.tar.gz`
  - right-click context menu stays in view.

  **Bugfixes**:
  - search result links when non-default baseUrl configured
  - frontend sort bug squashed https://github.com/gtsteffaniak/filebrowser/issues/230
  - bug which caused "noauth" method not to work after v0.3.0 routes update

## v0.3.1

  **New Features**
  - Adds Smart Indexing by default.

  **Notes**:
  - Optimized api request response times via improved caching and simplified actions.
  - User information persists more reliably.
  - Added [indexing doc](./docs/indexing.md) to explain the expectations around indexing and how it works.
  - The index should also use less RAM than it did in v0.3.0.

  **Bugfixes**:
  - Tweaked sorting by name, fixes case sensitive and numeric sorting. https://github.com/gtsteffaniak/filebrowser/issues/230
  - Fixed unnecessary authentication status checks each route change
  - Fix create file action issue.
  - some small javascript related issues.
  - Fixes pretty big bug viewing raw content in v0.3.0 (utf format message)

## v0.3.0

  This Release focuses on the API and making it more accessible for developers to access functions without the UI.

  **New Features**:
  - You can now long-live api tokens to interact with API from the user settings page.
    - These tokens have the same permissions as your user.
  - Helpful swagger page for API usage.
  - Some API's were refactored for friendlier API usage, moving some attributes to parameters and first looking for a api token, then using the stored cookie if none is found. This allows for all api requests from swagger page to work without a token.
  - Add file size to search preview! Should have been in last release... sorry!

  **Notes**:
  - Replaced backend http framework with go standard library.
  - Right-click Context menu can target the item that was right-clicked. To fully address https://github.com/gtsteffaniak/filebrowser/issues/214
  - adjusted settings menu for mobile, always shows all available cards rather than grayed out cards that need to be clicked.
  - longer and more cryptographically secure share links based on UUID rather than base64.

  **Bugfixes**:
  - Fixed ui bug with shares with password.
  - Fixes baseurl related bugs https://github.com/gtsteffaniak/filebrowser/pull/228 Thanks @SimLV
  - Fixed empty directory load issue.
  - Fixed image preview cutoff on mobile.
  - Fixed issue introduced in v0.2.10 where new files and folders were not showing up on ui
  - Fixed preview issue where preview would not load after viewing video files.
  - Fixed sorting issue where files were not sorted by name by default.
  - Fixed copy file prompt issue

## v0.2.10

  **New Features**:
  - Allows user creation command line arguments https://github.com/gtsteffaniak/filebrowser/issues/196
  - Folder sizes are always shown, leveraging the index. https://github.com/gtsteffaniak/filebrowser/issues/138
  - Searching files based on filesize is no longer slower.

  **Bugfixes**:
  - fixes file selection usage when in single-click mode https://github.com/gtsteffaniak/filebrowser/issues/214
  - Fixed displayed search context on root directory
  - Fixed issue searching "smaller than" actually returned files "larger than"

  **Notes**:
  - Memory usage from index is reduced by ~40%
  - Indexing time has increased 2x due to the extra processing time required to calculate directory sizes.
  - File size calculations use 1024 base vs previous 1000 base (matching windows explorer)

## v0.2.9

  This release focused on UI navigation experience. Improving keyboard navigation and adds right click context menu.

  **New Features**:
  - listing view items are middle-clickable on selected listing or when in single-click mode.
  - listing view items can be navigated via arrow keys.
  - listing view can jump to items using letters and number keys to cycle through files that start with that character.
  - You can use the enter key and backspace key to navigate backwards and forwards on selected items.
  - ctr-space will open/close the search (leaving ctr-f to browser default find prompt)
  - Added right-click context menu to replace the file selection prompt.

  **Bugfixes**:
  - Fixed drag to upload not working.
  - Fixed shared video link issues.
  - Fixed user edit bug related to other user.
  - Fixed password reset bug.
  - Fixed loading state getting stuck.

## v0.2.8

- **Feature**: New gallery view scaling options (closes [#141](https://github.com/gtsteffaniak/filebrowser/issues/141))
- **Change**: Refactored backend files functions
- **Change**: Improved UI response to filesystem changes
- **Change**: Added frontend tests for deployment integrity
- **Fix**: move/replace file prompt issue
- **Fix**: opening files from search
- **Fix**: Display count issue when hideDotFile is enabled.

## v0.2.7

 - **Change**: New sidebar style and behavior
 - **Change**: make search view and button behavior more consistent.
 - **Fix**: [upload file bug](https://github.com/gtsteffaniak/filebrowser/issues/153)
 - **Fix**: user lock out bug introduced in 0.2.6
 - **Fix**: many minor state related issues.

## v0.2.6

This change focuses on minimizing and simplifying build process.

- **Change**: Migrated to Vite / Vue 3
- **Change**: removed npm modules
  - replaced vuex with custom state management via src/store
  - replaced noty with simple card popup notifications
  - replaced moment with simple date formatter where needed
  - replaced vue-simple-progress with vue component
- **Feature**: improved error logging
  - backend errors show the root function that called them during the error
  - frontend errors print errors to console that fail try/catch
  - all frontend errors via popup notification & print to console as well
- **Fix**: Allow editing blank text based files in editor
- tweaked listing styles
- Feature: Allow disabling the index via configuration yaml

## v0.2.5

- Fix: delete user prompt works using native hovers.

## v0.2.4

- Feature: [create-folder-feature](https://github.com/gtsteffaniak/filebrowser/pull/105)
- Feature: [playable shared video](https://github.com/filebrowser/filebrowser/issues/2537)
- Feature: photos, videos, and audio get embedded preview on share instead of icon
- Fix: sharable link bug, now uses special publicUser
- Bump go version to 1.22
- In prep for vue3 migration, npm modules removed:
  - js-base64
  - pretty-bytes
  - whatwg-fetch
  - lodash.throttle
  - lodash.clonedeep

## v0.2.3

- Feature: token expiration time now configurable
- FIX: Hidden files are still directly accessible. (https://github.com/filebrowser/filebrowser/issues/2698)
- FIX: search/user context bug

## v0.2.2

- CHG: **Speed:** (0m57s) - Decreased by 78% compared to the previous release.
- CHG: **Memory Usage:** (41MB) - Reduced by 45% compared to the previous release.
- Feature: Now utilizes the index for file browser listings!
- FIX: Editor issues fixed on save and themes.

## v0.2.1

- Addressed issue #29 - Rules can now be configured and read from the configuration YAML.
- Addressed issue #28 - Allows disabling settings per user.
- Addressed issue #27 - Shortened download link for password-protected files.
- Addressed issue #26 - Enables dark mode per user and improves switching performance.
- Improved styling with more rounded corners and enhanced listing design.
- Enhanced search performance.
- Fixed authentication issues.
- Added compact view mode.
- Improved view mode configuration and behavior.
- Updated the configuration file to accept new settings.

## v0.2.0

- **Improved UI:**
  - Enhanced the cohesive and unified look.
  - Adjusted the header bar appearance and icon behavior.
- The shell feature has been deprecated.
  - Custom commands can be executed within the Docker container if needed.
- The JSON config file is no longer used.
  - All configurations are now performed via the advanced `config.yaml`.
  - The only allowed flag is specifying the config file.
- Removed old code for migrating database versions.
- Eliminated all unused `cmd` code.

## v0.1.4

- **Various UI fixes:**
  - Reintroduced the download button to the toolbar.
  - Added the upload button to the side menu.
  - Adjusted breadcrumb spacing.
  - Introduced a "compact" view option.
  - Fixed a slash issue with CSS right-to-left (RTL) logic.
- **Various backend improvements:**
  - Added session IDs to searches to prevent collisions.
  - Modified search behavior to include spaces in searches.
  - Prepared for full JSON configuration support.
- Made size-based searches work for both smaller and larger files.
- Modified search types not to appear in the search bar when used.

## v0.1.3

- Enhanced styling with improved colors, transparency, and blur effects.
- Hid the sidebar on desktop views.
- Simplified the navbar to include three buttons:
  - Open menu
  - Search
  - Toggle view
- Revised desktop search style and included additional search options.

## v0.1.2

- Updated the UI to better utilize search features:
  - Added more filter options.
  - Enhanced icons with colors.
  - Improved GUI styling.
- Improved search performance.
- **Index Changes:**
  - **Speed:** (0m32s) - Increased by 6% compared to the previous release.
  - **Memory Usage:** (93MB) - Increased by 3% compared to the previous release.

## v0.1.1

- Improved search functionality with indexing.
- **Index Changes (Baseline Results):**
  - **Speed:** (0m30s)
  - **Memory Usage:** (90MB)

## v0.1.0

- No changes from the original.

Forked from [filebrowser/filebrowser](https://github.com/filebrowser/filebrowser).
