# Changelog

All notable changes to this project will be documented in this file. For commit guidelines, please refer to [Standard Version](https://github.com/conventional-changelog/standard-version).

## v0.7.0-beta

 **New Features**:
 - New authentication methods:
   - OIDC (OpenID Connect)
   #- LDAP
 - Enhanced source info on the UI
   - User must have permission `realtime: true` property to get realtime events.
   - Sources shows status of the directory `ready`, `indexing`, and `unavailable`
 - new preview types:
   - Video thumbnails available via new media integration (see configuration wiki for help) https://github.com/gtsteffaniak/filebrowser/issues/351
   - Office file previews if you have office integration enabled. https://github.com/gtsteffaniak/filebrowser/issues/460
 - New scrollbar which includes information about the listing https://github.com/gtsteffaniak/filebrowser/issues/304
 - Refreshed icons and styles to provide more contrast https://github.com/gtsteffaniak/filebrowser/issues/493
  **Notes**:
    - sesssionId is now unique per window. Previously it was shared accross browser tabs.
    - replaced checkboxes with toggles https://github.com/gtsteffaniak/filebrowser/issues/461
    #- disableUsedPercentage is a backend property now.
  **Bug Fixes**:
    - Fix nil pointer error when source media is disconnected while running.

TODO:

- Safely checks ffmpeg on startup, test and warn if binary has errors and disable if it does.
- only show pulse when confirmed realtime connection
- test all onlyoffice file previews, things like csv do not work.
- add debouncer to source broadcasts
- ensure source broadcast doesn't send to wrong users info

## v0.6.8-beta

 **New Features**
 - environment variables are available for certain secrets.
   - see wiki https://github.com/gtsteffaniak/filebrowser/wiki/Environment-Variables
   - thanks @aaronkyriesenbach https://github.com/gtsteffaniak/filebrowser/pull/511

 **Notes**:
 - config validation (see https://github.com/gtsteffaniak/filebrowser/wiki/Full-Config-Example)
   - fails when config file contains unknown fields (helps spot typos)
   - some light value validation on certain fields
   - removed recaptcha -- was disabled and not used before.
   - moved `recaptcha` and `signup` configs to `auth.methods.password`

 **BugFixes**:
 - fix scope reset on restart https://github.com/gtsteffaniak/filebrowser/issues/515
 - Clicking empty space to deselect https://github.com/gtsteffaniak/filebrowser/issues/492

## v0.6.7-beta

 **Notes**:
 - added full tests for single source example.
 - adds descriptive error if temp dir can't be created on fatal startup
 - clears temp directory on shutdown.
 - removed put settings api (unused)
 - removed more unused config properties.

 **BugFixes**:
 - fix url encoding issue for search links when theres only one source https://github.com/gtsteffaniak/filebrowser/issues/501
 - files with # could have problems, double encoded.

## v0.6.6-beta

 **New Feature**:
 - limit tar size creation to limit server burden. For example, don't let customers try to download the entire filesystem as a zip. see `server.maxArchiveSize` on config wiki.

 **Notes**:
 - disableUsedPercentage also hides text and source bar.
 - share errors show up in logs in more verbose way.
 - archive creation occurs on disk rather than in memory, use `server.cacheDir` to determine where temp files are stored.
 - automatically ensures leading slash for scope
   - https://github.com/gtsteffaniak/filebrowser/issues/472
   - https://github.com/gtsteffaniak/filebrowser/issues/476

 **BugFixes**:
 - fix proxy user creation issue https://github.com/gtsteffaniak/filebrowser/issues/478
 - externalUrl prefix issue fixed for shares. https://github.com/gtsteffaniak/filebrowser/issues/465
 - fix File Opens Instead of Just Downloading https://github.com/gtsteffaniak/filebrowser/issues/480
 - fix Download file name https://github.com/gtsteffaniak/filebrowser/issues/481

## v0.6.5-beta

 **Notes**:
 - added more share and download tests

 **BugFixes**:
 - fix share download issue https://github.com/gtsteffaniak/filebrowser/issues/465
 - fix content length size calculation issue when downloading multiple files.

## v0.6.4-beta

 **BugFixes**:
 - fix preview arow issue. https://github.com/gtsteffaniak/filebrowser/issues/457
 - fix password change issue.
 - apply user defaults to publi user on startup https://github.com/gtsteffaniak/filebrowser/issues/451

## v0.6.3-beta

 **Notes**:
 - windows directories get better naming, root directories like "D:\ get named "D", otherwise base filepath is the name when unselected "D:\path\to\folder" gets named "folder" (just like linux)
 - `.pdf` files added to default onlyoffice exclusion list.

 **BugFixes**:
 - windows would not refresh file info automatically when viewing because of path issue.
 - windows paths without name for "D:\" would cause issues.
 - share path error https://github.com/gtsteffaniak/filebrowser/issues/429
 - fix bug where resource content flag would load entire file into memory.

## v0.6.2-beta

 **Notes**:
 - Added playwright tests for bugfixes for permantent fix for stability.
    (except onlyoffice since it requires integrations)

 **BugFixes**:
 - Context menu should only be available inside the folder/files container https://github.com/gtsteffaniak/filebrowser/issues/430
 - drag and drop files from desktop to browser is fixed.
 - replace prompt cancel button didn't work.
 - key events on listing page not working (like delete key)
 - fixed share viewing issue https://github.com/gtsteffaniak/filebrowser/issues/429
 - disableUsedPercentage hides entire source https://github.com/gtsteffaniak/filebrowser/issues/438
 - createUserDir fix for proxy users and new users https://github.com/gtsteffaniak/filebrowser/issues/440

## v0.6.1-beta

 **New Feature**:
 - download size information is added, including when downloding multiple files in zip/tar.gz. The browser will see the XMB of X GB and will show browser native progress.

 **BugFixes**:
 - fixed onlyoffice bug https://github.com/gtsteffaniak/filebrowser/issues/418
 - fixed breadcrumbs bug https://github.com/gtsteffaniak/filebrowser/issues/419
 - fixed search context bug https://github.com/gtsteffaniak/filebrowser/issues/417
 - fixed sessionID for search

## v0.6.0-beta

> [!WARNING]
> This release includes several config changes that could cause issues. Please backup your database file before upgrading.

This release has several changes that should work without issues... however, still backup your database file first and proceed with caution. User permissions and source config changes have been updated -- and the `server.root` paramter is no longer used.

This is a significant step towards a stable release. There shouldn't be any major breaking config changes after this.

 **New Features**:
  - multiple sources support https://github.com/gtsteffaniak/filebrowser/issues/360
    - listing view keeps them independant, you switch between the two and the url address will have a prefix `/files/<sourcename>/path/to/file` when there is more than 1 source.
    - search also happens independantly, with a selection toggle per source. searching current source searches the current scope in the listing view, if you toggle to an alternative source it will search from the source root.
    - copy/moving is currently only supported within the same source -- that will come in a future release.
  - `FILEBROWSER_CONFIG` environment variable is respected if no CLI config parameter is provided. https://github.com/gtsteffaniak/filebrowser/issues/413

 **Notes**:
  - downloads no longer open new window.
  - swagger updated with auth api help for things like api token.
    - GET api keys now uses `name` query instead of `key`. eg `GET /api/auth/tokens?name=apikeyname`
  - user permissions simplified to four permission groups (no config change required):
    - **removed**  : create, rename, delete, download
    - **remaining**: admin, modify, share, api
    - `scope` is deprecated, but still supported, applies to default source. if using multiple sources, set `defaultUserScope` at the [source config](https://github.com/gtsteffaniak/filebrowser/wiki/Configuration#default-source-configuration) instead.
  - **removed** user rules and commands.
    - commands feature has never been enabled so just removing the references.
    - rules will come back in a different form (not applied to the user).
  - `server.root` is completely removed in favor of `server.sources`

 **BugFixes**:
  - fix conflict resolution issue https://github.com/gtsteffaniak/filebrowser/issues/384
  - many user creation page bugfixes.
  - fix share delete issue https://github.com/gtsteffaniak/filebrowser/issues/408

## v0.5.4-beta

 **BugFixes**:
  - default scope share issue. @theryecatcher https://github.com/gtsteffaniak/filebrowser/pull/387
  - drag and drop on empty folders https://github.com/gtsteffaniak/filebrowser/issues/361
  - preview navigation issue https://github.com/gtsteffaniak/filebrowser/issues/372
  - auth proxy password length error https://github.com/gtsteffaniak/filebrowser/issues/375

<img width="294" alt="image" src="https://github.com/user-attachments/assets/669bca75-98d4-47c1-838b-1ffee2967d7d" />

## v0.5.3-beta

 **New Features**:
  - onlyoffice disable filetypes for user specified file types. https://github.com/gtsteffaniak/filebrowser/issues/346

 **Notes**:
  - navbar/sidebar lightmode style tweaks.
  - any item that has utf formatted text will get editor.
  - tweaks to create options on context menu.
  - removed small delay on preview before detecting the file.

 **BugFixes**:
  - fix `/files/` prefix loading issue https://github.com/gtsteffaniak/filebrowser/issues/362
  - fix special characters in filename issue https://github.com/gtsteffaniak/filebrowser/issues/357
  - fix drag and drop issue https://github.com/gtsteffaniak/filebrowser/issues/361
  - fix conflict issue with creating same file after deletion.
  - fix mimetype detection https://github.com/gtsteffaniak/filebrowser/issues/327
  - subtitles for videos https://github.com/gtsteffaniak/filebrowser/issues/358
    - supports caption sidecar files : ".vtt", ".srt", ".lrc", ".sbv", ".ass", ".ssa", ".sub", ".smi"
    - embedded subtitles not yet supported.

## v0.5.2-beta

 **New Features**:
  - Markdown file preview https://github.com/gtsteffaniak/filebrowser/issues/343
  - Easy access download button https://github.com/gtsteffaniak/filebrowser/issues/341

 **Notes**:
  - Adds message about what sharing means when creating a link.
  - api log duration is now always in milliseconds for consistency.
  - advanced index config option `fileEndsWith` is now respected.
  - Added Informative error for missing files for certificate load https://github.com/gtsteffaniak/filebrowser/issues/354

 **BugFixes**:
  - onlyoffice close window missing files issue https://github.com/gtsteffaniak/filebrowser/issues/345
  - fixed download link inside file preview

## v0.5.1-beta

 > Note: I changed the [config](https://github.com/gtsteffaniak/filebrowser/wiki/Configuration#example-auth-config) for password auth again... It was a mistake just to make it a boolean, so now you can provide options, going forward this allows for more.

  **New Features**:
  - password length requirement config via `auth.methods.password.minLength` as a number of characters required.

  **Bugfixes**:
  - NoAuth error message "resource not found"
  - CLI user configuration works and simplified see examples in the [Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/CLI)

## v0.5.0-beta

 > Note: This Beta release includes a configuration change: `auth.method` is now deprecated. This is done to allow multiple login methods at once. Auth methods are specified via `auth.methods` instead. see [example on the wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Configuration#example-auth-config).

  **New Features**:
  - Upload progress notification https://github.com/gtsteffaniak/filebrowser/issues/303
  - proxy auth auto create user when `auth.methods.proxy.createUser: true` while using proxy auth.

  **Notes**:
  - Context menu positioning tweaks.
  - using /tmp cachedir is disabled by default, cache dir can be specified via `server.cacheDir: /tmp` to enable it. https://github.com/gtsteffaniak/filebrowser/issues/326

  **Bugfixes**:
  - Gracefully shutdown to protect database. https://github.com/gtsteffaniak/filebrowser/issues/317
  - validates auth method provided before server startup.
  - fix sidebar disk space usage calculation. https://github.com/gtsteffaniak/filebrowser/issues/315
  - Fixed proxy auth header support (make sure your proxy and server are secure!). https://github.com/gtsteffaniak/filebrowser/issues/322

## v0.4.2-beta

  **New Features**:
  - Hidden files changes
    - windows hidden file properties are respected -- when running on windows binary (not docker) with NTFS filesystem.
    - windows "system" files are considered hidden.
    - changed user property from `hideDotFiles` to `showHidden`. Defaults to false, so a user would need to must unhide hidden files if they want to view hidden files.

  **Notes**:
  - cleaned up old and deprecated config.
  - removed unneeded "Global settings". All system configuration is done on config yaml, See configuration wiki for more help.

  **Bugfixes**:
  - Another fix for memory https://github.com/gtsteffaniak/filebrowser/issues/298

## v0.4.1-beta

  **New Features**:
  - right-click actions are available on search. https://github.com/gtsteffaniak/filebrowser/issues/273

  **Notes**:
  - delete prompt now lists all items that will be affected by delete
  - Debug and logger output tweaks.

  **Bugfixes**:
  - calculating checksums errors.
  - copy/move issues for some circumstances.
  - The previous position wasn't returned when closing a preview window https://github.com/gtsteffaniak/filebrowser/issues/298
  - fixed sources configuration mapping error (advanced `server.sources` config)

## v0.4.0-beta

  **New Features**:
  - Better logging https://github.com/gtsteffaniak/filebrowser/issues/288
    - highly configurable
    - api logs include user
  - onlyOffice support for editing only office files (inspired from https://github.com/filebrowser/filebrowser/pull/2954)

  **Notes**:
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

  **New Features**:
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

  **New Features**:
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
