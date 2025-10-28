# Changelog

All notable changes to this project will be documented in this file. For commit guidelines, please refer to [Standard Version](https://github.com/conventional-changelog/standard-version).

## v0.8.11-beta

 **New Features**:
 - OnlyOffice debugger now shows backend logs as well for admin users.
 - If proxy auth based `username` equals `auth.adminUsername`, the user will be promoted to admin.
 - Guard against accidently cancelling uploads #1419
 - Added a quirky halloween theme that automatically applies on october 31st -- you can disable this from happening by setting `frontend.styling.disableEventThemes`

 **Notes**:
 - Removed share setting `enableOnlyOfficeEditing` -- uses value from `allowEditing`
 - Anonymous users can now edit if a share has edit permissions enabled.
 - No "Incompatible user settings detected" message if its first setup
 - cookie handling revamped
   - fully backend managed
   - `auth` cookie name changed to `filebrowser_quantum_jwt`
   - auth cookie for password users no longer session based #1439
   - removed state jwt state variable -- 100% cookie based.

 **BugFixes**:
 - Fixed issue editing onlyoffice on shares #1397
 - Added proper and robust logging / error handling for the onlyoffice callback. #1422 #1068
 - If a file was moved/renamed/deleted onlyoffice would re-save the original file without error, now it gives error the file doesn't exist anymore.
 - OnlyOffice integration does not work behind proxy authentication #1422
 - proxy-based authentication with docker does not work unless set up with password-based login first #1226
 - Select all bug on safari mobile #1421
 - Dragging a item into itself #1446

## v0.8.10-beta

 **New Features**:
 - Add marquee selection to listing view -- thanks @Kurami32 #1388

 **Notes**:
 - More changes to login auth flow
 - respects `FILEBROWSER_DISABLE_AUTOMATIC_BACKUP='true'` env var #1398
 - Deprecating `conditionals.hidden` instead use `conditionals.ignoreHidden`

 **BugFixes**:
 - Fix material outline styling
 - "feels lonely here" when clicking at source tab #1387
 - Anonymous upload bug 0.8.9 #1383
 - changing previews between documents in onlyoffice #1410
 - onlyoffice should display user language #1061
 - Filebrowser will hard fail when OIDC provider is not found #733
 - some API endpoints always return 404 #849

## v0.8.9-beta

> [!CAUTION]
> If you have indexing rules -- such as [maxWatchers](https://github.com/gtsteffaniak/filebrowser/blob/39514169f17ed9586d587dd496257588ded6e532/frontend/public/config.generated.yaml#L30), [neverWatchPaths](https://github.com/gtsteffaniak/filebrowser/blob/39514169f17ed9586d587dd496257588ded6e532/frontend/public/config.generated.yaml#L31C9-L31C24), [exclude](https://github.com/gtsteffaniak/filebrowser/blob/39514169f17ed9586d587dd496257588ded6e532/frontend/public/config.generated.yaml#L32C9-L32C16) rules, [include](https://github.com/gtsteffaniak/filebrowser/blob/39514169f17ed9586d587dd496257588ded6e532/frontend/public/config.generated.yaml#L43) rules, see [the migration guide](https://filebrowserquantum.com/en/docs/user-guides/general-configuration/exclusion-rules/) for help.

 **New Features**:
 - Playback Queue for media -- thanks @Kurami32 #1339
 - Ctrl+b Shortcut to toggle sidebar, open settings, search #1172
 - Add a "select all" toggle in the menu #974
 - Save on exit text editor -- thanks @srini-abhiram #1334 #1241
 - Disabled indexing for subfolders but allow browsing #783
 - Added more granular create/upload, delete, and download (includes viewing content) permission #1200 #1199
 - Upload settings can be adjusted directly in the upload prompt
 - Added paste confirmation prompt when using copy/paste keyboard shortcuts.
 - Updated shares
   - `disableDownload` to disable download and (content viewing).
   - `allowReplacements` to share permission -- prohibits replacing files on conflict #661
   - normal shares can create/upload files if given permission

 **Notes**:
 - Optimized font and icon loading.
 - the next/previous buttons will follow media queue order if playback mode has a queue.
 - All new exclude rules -- see [full config example](https://github.com/gtsteffaniak/filebrowser/wiki/Full-Config-Example)
 - The order of sources will strictly match order in config.yaml. Default landing page is root first source unless otherwise configured in profile settings.
 - Uploading changes #1371
   - Chunked uploading can no longer be explicitly disabled, defaults to 5MB chunks
   - Automatic chunked/non-chunked uploads based on size. If an item exceeds the chunk size, chunked uploading will occur. If its less than chunk size, no chunked upload.
   - To "disable" chunked upload, you an set your chunk size very large -- 0 chunk size will be reset to 5MB.

 **BugFixes**:
 - Remember the previous location after closing a preview #1336
 - fixed issue related to multi-config parsing #1267
 - Cannot save text files, missing button #1326 #1367
 - Files are being overwritten and/or content disappears #1312
 - moving between items using next/previous will reset the req and show a responsive loading spinner -- fixing several state related issues.
 - access management: child folders accessible stopped showing up #1332
 - Make source inaccessible if directory does not exist rather than exiting on start #1264
 - HTTP Proxy environment gets ignored since version v0.8.6-beta #1324
 - album artwork preview not showing on shares
 - fixed OIDC logout causing a loop #995 #1361
 - fixed checksum failure #1372
 - Copy, paste, cut shortcuts don't work and make the UI unusable #1375

## v0.8.8-beta

 **New Features**:
 - Ability to split configuration files via yaml anchoring #1267
   - see [How to: Use multiple config files](https://github.com/gtsteffaniak/filebrowser/wiki/How-to:-Use-multiple-config-files)
 - added [Init Script Help wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Init-Script-Help) to examples how you can bootstrap filebrowser quantum programatically
 - Add capability to edit files permissions #813
   - see updated full config for help using `server.filesystem`
 - better copy/move prompt "in progress" indicator
 - better login error message feedback
 - Add capability to disable certain video previews at server level. See full config for help.
 - Added ability to play media in same folder sequentially or shuffled.
 - upload/upload only share links. #661
 - Default landing directory if no other redirect is found (set profile settings) #781
 - share download limit feature can apply per user -- and disables anonymous download when enabled.
 - Docker health check #1292
 - embedded subtitle support is now **disabled** by default, can be enabled via `integrations.media.extractEmbeddedSubtitles`. Per share configuration is also possible via `extractEmbeddedSubtitles` toggle. This change is because reading subtitles requires processing the entire file, and multi GB video files can take 10-20 seconds.

 **Notes**:
 - Some standardized path format changes for access rules to ensure the rules apply. A migration check happens on startup, all rules should still be in effect without any issues, but double check.
 - moved `server.debugMedia` to `integrations.media.debug`
 - optimized thumbnail generation workflow
   - more concurrency changes for video thumbnails (now half of the numImageProcessors config)
   - 15s timeout added to preview api
   - preview operations are cancelled if api request is terminated.
   - reduced io overhead for video previews
   - you can now disable individual video file previews system wide at server config level (all enabled by default)

 **BugFixes**:
 - video plyr dark mode style issue where buttons are black instead of white
 - error when running without a config file #1280
 - Issue persist in 0.8.7 with Access Management #1282
 - all static resources go through /public/static url to fix logout loop #995
 - "realtime" Permission of API keys is not working #1141
 - "Path not found" when trying to share a file or folder inside a sub-directory #1139
 - logout path redirect not working
 - 2FA secret error #1305
 - console errors during login/logout
 - fix scoped user share creation issue #1309
 - share close button and navigation issues.
 - OnlyOffice source not found when opening shared file #1285
 - user scoped onlyoffice error saving files with Onlyoffice #1068

## v0.8.7-beta

 **New Features**:
 - json logging format support

 **Notes**:
 - increased overall file size limit to retrieve metadata from 50MB to 300MB #1234
 - document previews don't show up as folder previews.

 **BugFixes**:
 - Fix directories empty issue when disableIndexing is true #1249
 - OIDC to Authentik behind CloudFlare with Bot Fight turned on triggers a JS challenge #1165
 - OIDC Auth - loop with expired token #995
 - We lost the ability for download all? #1250
 - Directories are all empty when disableIndexing is true #1248
 - Access Management: issues on the shared folder when there's a sub folder #1208
 - user preferences for individual preview file types not getting respected (disable/enable office,image,video)
 - High server load when browsing folders with .mkv files #1259
 - 2FA can not be disabled with non-admin user #1244
 - 2FA not working on login #1243
 - Thumbnails of .m4a, .flac, and .opus are not displaying #1234
 - creating files/folders with same name overwrite each other #1242

## v0.8.6-beta

 **Notes**:
 - Please remove `indexAbumArt` config option from your config: it has been deprecated... I found a way to detect album art without impacting indexing performance -- so its default behavior.
 - Download button should be invisible before password input #1162
 - opus files / opus files detected wrongly #1164
 - changed autoplay behavior on plyr to not start playing if blocked instead of playing muted.
 - QR-Code disappearing / sliding out from shared link #1155
 - audio player matches dark/light mode better

 **BugFixes**:
 - Copy/Move index update changes to be more thorough and update album artwork #1220 #1219
 - fix preview related issues #1225 #1223
 - Scrollbar missing on user-edit-prompt in v0.8.5-beta #1221
 - copy share download link inconsistency #1207
 - fixed some onlyoffice related issues #1192 #1068
 - Creating a file, then a folder with the same name makes the folder unusable #1167
 - fix plyr issue switching from video to audio via next/previous button.

## v0.8.5-beta

 **New Features**:
 - backend media metadata processing
   - significantly improved performance
   - album artwork shows in listing view as preview thumbnail Album art thumbnail: display embedded covers of mp3/flac files #925
   - folders can show album art if enabled with indexing `server.sources.config.indexAlbumArt:true`
   - removed need for frontend dependancies.
 - optional thumbnail support for folders -- uses first child item with thumbnail as cover. Useful in combination with album metadata support to show album covers for folders with music.
 - Remember folder view type (and sorting) for each folder #966
 - heic is supported in viewer (via ffmpeg conversion) when on non-safari browser. This can be enabled via `integrations.ffmpeg.convert.impagePreview.heic: true` #1191
 - OnlyOffice: Add option to open documents in viewer mode (`integrations.office.viewOnly`) #1193
 - Exclude folders and file names 'starting with' or wildcard option #1054
 - added better info for failed uploads #1050
 - upload prompt "clear completed" can be changed to "clear all" in uploads & downloads settings to clear error and conflict states as well. #1128
 - share 'default view mode' option #1212
 - Universal Next/Previous buttons
   - available for all listing items (including office/markdown etc)
   - remembers sort order from parent directory
   - new "Quick Jump" prompt if you drag the next/previous icons. This shows other files available to "jump" directly to quickly.

 **Notes**:
 - Added more tests to ensure new features work in future.
 - caching for preview images is md5 based. Moved, renamed, or duplicate images don't get re-generated. Same album artwork shares cache.
 - Hide @eaDir folder by default #1212
 - defaults to hide "@eadir" folders (common for synology) #1212
 - After move/copy, ability to move to the destination folder #999

 **BugFixes**:
 - Access Management: issue with access settings #1195
 - fix shutdown panic related to sse connection
 - Custom theming not working in 0.8.4 beta #1204
 - Config Viewer not working #1189
 - "Path not found" when trying to share a file or folder inside a sub-directory #1139
 - files containing "+" in share issue #1089

## v0.8.4-beta

 **New Features**:
 - New media player styles and features
   - Custom Media Player: enhanced media player using plyr thanks @Kurami32 (see #1160)
   - Custom Media Player: also adds support for metadata
   - added embeded video subtitle support (for both native and custom player). @maxbin123 #1072 #1157
   - Users can disable the customer player and opt of native in profile settings.
 - Option to disable backend update check via `server.disableUpdateCheck` #1134
 - added `frontend.favicon` and `frontend.description` for html overrides
 - onlyoffice is now supported in shares. Both viewing and editing can be configured per-share.
 - Added only office debug view and wiki to assist with debugging issues #1068 #911 #1074
 - Dark mode enforcement possible for shared links #1029
 - added `System & Admin` section to settings
   - includes a new config viewer to see current running config (hides secrets) #838
 - added `server.minSearchLength` to allow adjusting the length requirement for search #1174

 **Notes**:
 - access management: specific folders/files with access are shown instead permission denied for parent folder
 - navigation no longer appends last location hash which should fix some unwanted navation behavior #1070
 - altered the context menu style and behavior.
 - documentation update: comma or Space separated extensions #1138
 - Files and folders can be created with "/" or "\" on the name #1126
 - Share management should not be allowed without authentication #1163
 - Question about customizing session timeout #1184

 **BugFixes**:
 - access management: delay showing rule changes in the list fixed. #1131
 - Color names are not localized #1159
 - rename issues #1170 #1171
 - some shortcuts not working #1056
 - Can't copy/paste text on mobile #1168
 - Can't change between images inside of the share image viewer. #1144
 - fixed and updated translations with variables always showing english.

## v0.8.3-beta

 **BugFixes**:
 - fixed search bar style bug in mobile #1147

## v0.8.2-beta

 **New Features**:
 - added `source.config.denyByDefault` configuration to enable a deny-by-default access rule. A source enabled with this will deny access unless an "allow" rule was specifically created. (Similar to creating a root-level denyAll rule)
 - allow oidc user source access and permission based on username and groups is fulfilled by denyByDefault source with access rules https://github.com/gtsteffaniak/filebrowser/issues/824
 - "open parent folder" in context menu and search results https://github.com/gtsteffaniak/filebrowser/issues/1121
 - added friendly "share not found" page.

 **Notes**:
 - 8.0 ffmpeg version bundled with docker
 - go 1.25 upgrade with green tea GC enabled
 - totp secrets accept non-secure strings, only throwing warning
 - adjusted download limit so it also counts viewing text "content" of files (like in editor). You can also "disable file viewing" to stop the editor from showing. lower quality file image previews are not counted as downloads.
 - updated invalid share message to be more clear https://github.com/gtsteffaniak/filebrowser/issues/1120

 **BugFixes**:
 - fixed /public/static routes issue
 - shares redirect to login - https://github.com/gtsteffaniak/filebrowser/issues/1109
 - some static assets not available to anonymous user - https://github.com/gtsteffaniak/filebrowser/issues/1102
 - more safari style issues https://github.com/gtsteffaniak/filebrowser/issues/1110
 - fix public share download issues https://github.com/gtsteffaniak/filebrowser/issues/1118 https://github.com/gtsteffaniak/filebrowser/issues/1089
 - fixed disable file viewer setting and enforced on backend

## v0.8.1-beta

 **New Features**:
 - api for generate download link (see swagger) https://github.com/gtsteffaniak/filebrowser/issues/1007
 - added `source.config.disabled` option to disable a source without removing it from config file.
 - added `source.config.private` option to designate as private -- currently just means no sharing permitted.
 - hide share card in share
 - download count for a share shows up on share management

 **Notes**:
 - updated description for indexingIntervalMinutes https://github.com/gtsteffaniak/filebrowser/issues/1067

 **BugFixes**:
 - fixed styling issues https://github.com/gtsteffaniak/filebrowser/issues/1086 https://github.com/gtsteffaniak/filebrowser/issues/1081 https://github.com/gtsteffaniak/filebrowser/issues/1082 https://github.com/gtsteffaniak/filebrowser/issues/1098
 - fix download limit issue https://github.com/gtsteffaniak/filebrowser/issues/1085
 - fixed oidc user defaults for new user https://github.com/gtsteffaniak/filebrowser/issues/1071
 - shares get updated when files moved in ui https://github.com/gtsteffaniak/filebrowser/issues/760
 - click listing behavior doesn't clear (introduced in 0.8.0) https://github.com/gtsteffaniak/filebrowser/issues/1101
 - show download count and limit in share list in settings https://github.com/gtsteffaniak/filebrowser/issues/1103
 - fix windows alt+arrow movement issue https://github.com/gtsteffaniak/filebrowser/issues/1094
 - nav memory issue for filenames with brackets https://github.com/gtsteffaniak/filebrowser/issues/1092
 - files with "+"" in name issue https://github.com/gtsteffaniak/filebrowser/issues/1089
 - fixed editor bug in share view https://github.com/gtsteffaniak/filebrowser/issues/1084
 - other share related issues https://github.com/gtsteffaniak/filebrowser/issues/1087 https://github.com/gtsteffaniak/filebrowser/issues/1064


## v0.8.0-beta

  This is a major release, new features and changes could introduce breaking behavior. Here are the known potentially breaking changes:

  - all public api and share url's get a `/public` prefix, making it easier to use with a reverse proxy. Any existing share link will still work but get redirected.
  - a small change to styling you may need to update your custom styling, for example the id `#input` was renamed `#search-input`

 **New Features**:
 - New access control system. You can add new allow / deny / denyAll rules for users/groups for specific paths on specific sources.
   - groups currently only works with provided oidc groups, but will add a full group management option for manual creation. https://github.com/gtsteffaniak/filebrowser/issues/545
 - share view changes -- now aligns with the standard listing view. This means files can be viewed and edited (if permission allows) just like a normal listing.
 - many share links customization enhancements
   - only share to certain authenticated users https://github.com/gtsteffaniak/filebrowser/issues/656 https://github.com/gtsteffaniak/filebrowser/issues/985
   - one-time download links
   - customize share theme https://github.com/gtsteffaniak/filebrowser/issues/827 https://github.com/gtsteffaniak/filebrowser/issues/1029
   - share link public changes https://github.com/gtsteffaniak/filebrowser/issues/473
   - shares can be modified/configured after creation.
   - download throttling for shares

 **Notes**:
 - hover effect on list/compact view https://github.com/gtsteffaniak/filebrowser/issues/1036

 **BugFixes**:
 - fix new file "true" content issue https://github.com/gtsteffaniak/filebrowser/issues/1048
 - editor allows device default popup https://github.com/gtsteffaniak/filebrowser/issues/1049

## v0.7.18-beta

 **Notes**:
 - desktop context menu "select multiple" enabled as optional user default (#1000)
 - onlyoffice readonly document types (".pages", ".numbers", ".key") list (#1018)
 - onlyoffice tweaks to make more consistent, added logging (#1015)

 **BugFixes**:
 - fix lightBackground issue (#1021)
 - fix user save issues (#1020, #1027)
 - fix image preview cache issue (#989)
 - fix file/folder count issue (#989)
 - only first file was upload on drag-n-drop (#1024)

## v0.7.17-beta

See an example of custom css styling that uses the reduce-rounded-corners.css by default and allows users to choose other themes. You can add your own themes as well that users can choose from in profile settings:

```
frontend:
  styling:
    lightBackground: "#f0f0f0"   # or names of css colors
    darkBackground: "#121212"
    customCSS: "custom.css"  # custom css file always applies first, then user themes on top of that.
    customThemes:
      "default": # if "default" is specified as the name, it will be the default option
        description: "Reduce rounded corners"
        css: "reduce-rounded-corners.css" # path to css file to use
      "original":
        description: "Original rounded theme"
        css: ""  # you could default to no styling changes this way.
```

 **New Features**:
 - more custom styling options (thanks @mordilloSan for #997)
   - background colors can be easily set in config
   - provided an example `reduce-rounded-corners.css` available by default in docker. (#986, #837)
   - added feature to specify multiple css themes that users can choose from in profile settings
 - swipe between photos on mobile (#825)

 **Notes**:
 - changed partition calculations on linux for total disk size (#982)
 - upload conflict detection for folders offers "replace all" if the folder already exists in target location.

 **BugFixes**:
 - TOTP prompt not showing generated code issue https://github.com/gtsteffaniak/filebrowser/issues/996
 - select mulitple deselect on mobile (#1002)
 - viewing svg images.

## v0.7.16-beta

 **Notes**:
 - more server logging for uploads when debug logging is enabled

 **BugFixes**:
 - fix onlyoffice integration viewing bug (#990)
 - fix uploading files with exec permissions (#984)
 - fix redirect on no source path (#989)
 - refresh file info on rename (#989)
 - listing refreshes when uploads finish (#989)
 - disable edit mode for certain onlyoffice files (#971)

## v0.7.15-beta

 **New Features**:
 - added userDefault `disableViewingExt`. The new properties apply to all files, not just office.
 - code blocks in markdown viewer have line numbers and each line is highlightable

 **Notes**:
 - replaced `disableOfficePreviewExt` with more generally applicable `disablePreviewExt` to disable preview for any specific file type.
 - more tooltip descriptions for settings options

 **BugFixes**:
 - fix chinese and other language error (#972, #969)
 - fix docker dockerfile for `docker run` (#973)
 - fix double slash href on single source (#968)
 - fix sources named "files" or "share" issue (#949, #574)
 - focus input field on popups (#976)
 - hopeful fix for size calculation (#982)
 - edit button is not working on .md files (#983)

## v0.7.14-beta

 **Notes**:
 - Updated translations https://github.com/gtsteffaniak/filebrowser/issues/957
 - enabled more doc types for onlyoffice https://github.com/gtsteffaniak/filebrowser/discussions/945

 **BugFixes**:
 - noauth user issue https://github.com/gtsteffaniak/filebrowser/issues/955
 - error 403 on source name with special characters https://github.com/gtsteffaniak/filebrowser/issues/952
 - delete pictures in previewer issue https://github.com/gtsteffaniak/filebrowser/issues/456
 - trailing slash source name issue https://github.com/gtsteffaniak/filebrowser/issues/920
 - image lazy loading issue causing all items to get previews at one time, not just whats in view.

## v0.7.13-beta

 **New Features**:
 - copy and Move files between sources https://github.com/gtsteffaniak/filebrowser/issues/689
 - new enhanced upload prompt
   - uses chunked uploads https://github.com/gtsteffaniak/filebrowser/issues/770
   - all or individual uploads can be paused/resumed
   - individual uploads can be retried
   - individual file upload progress https://github.com/gtsteffaniak/filebrowser/issues/871
   - keeps screen on https://github.com/gtsteffaniak/filebrowser/issues/900

 **Notes**:
 - lots of UI improvements
 - reworked a lot of the frontend path/source logic to be more consistent.
 - updated sort behavior to be natural sort https://github.com/gtsteffaniak/filebrowser/issues/551
 - optional quick save icon https://github.com/gtsteffaniak/filebrowser/issues/918
 - improved language support: zh-tw chinese traditional (tawain)

 **BugFixes**:
 - more accurate disk used calculation -- accounting for hard links and sparse files. https://github.com/gtsteffaniak/filebrowser/issues/921
 - fix api key revoking mechanism
 - fixed shift-select https://github.com/gtsteffaniak/filebrowser/issues/929
 - video preview images on safari https://github.com/gtsteffaniak/filebrowser/issues/932
 - sticky mode isn't sticky https://github.com/gtsteffaniak/filebrowser/issues/916

## v0.7.12-beta

Happy 4th of July!

The most noteworthy change is that no sources will be automatically enabled for any user. In order for a user to use a source, it needs to be added for that user. Or to keep a source available for all users, you can specify `defaultEnabled` in the source config to maintain the same behavior. See the wiki

 **New Features**:
 - setting added `deleteWithoutConfirming`, useful for quickly deleting files -- does not apply to folders.
 - more options for minimal UI https://github.com/gtsteffaniak/filebrowser/issues/745
 - dedicated section for sidebar customization in profile settings https://github.com/gtsteffaniak/filebrowser/issues/437

 **Notes**:
 - Filebrowser no longer requires a default source, users can be created without any sources.
 - Disables changing login type fallback behavior https://github.com/gtsteffaniak/filebrowser/issues/620
 - Uses calculated index size as "used" and total partition size as "total" https://github.com/gtsteffaniak/filebrowser/issues/875
 - Select multiple won't show up in context menu when using a desktop browser (with keyboard), opting for keyboard shortcuts
 - Updated translations that were not complete, such as simplified chinese https://github.com/gtsteffaniak/filebrowser/issues/895
 - larger min drop target size https://github.com/gtsteffaniak/filebrowser/issues/902
 - refresh page after file actions https://github.com/gtsteffaniak/filebrowser/issues/894
 - improved user PUT handler for easier user modification via API https://github.com/gtsteffaniak/filebrowser/issues/897
 - optional sidebar actions for upload/create https://github.com/gtsteffaniak/filebrowser/issues/885

 **BugFixes**:
 - fix delete in preview when moving between pictures. https://github.com/gtsteffaniak/filebrowser/issues/456
 - getting file info issue when indexing is disabled.
 - fixed initial sort order https://github.com/gtsteffaniak/filebrowser/issues/551
 - incorrect filename Drag and Drop fixes https://github.com/gtsteffaniak/filebrowser/issues/880
 - fix share duration always showing just now https://github.com/gtsteffaniak/filebrowser/issues/896

## v0.7.11-beta

 **Breaking Changes**:
  - `auth.resetAdminOnStart` has been removed. Instead, if you have `auth.adminPassword` set it will always be reset on startup. If you want to change your default admin password afterwards, make sure to unset `auth.adminPassword` so it doesn't get reset on startup.
  - renamed include/exclude rules see [updated example wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Configuration-And-Examples#example-advanced-source-config)!

 **New Features**:
 - more comprehensive exclude/include rules (see example wiki above).
   - include/exclude parts of folder names as well https://github.com/gtsteffaniak/filebrowser/issues/854
   - include/exclude file or folder names globally.
 - `source.config.neverWatchPaths` is now functional -- a list of paths that get indexed initially, but skips re-indexing. Useful for directories you don't expect to change ever, still show up in search but get don't contribute to indexing time after initial indexing.

 **Notes**:
 - updated swagger docs https://github.com/gtsteffaniak/filebrowser/issues/849

 **BugFixes**:
 - fix version update notification for binary https://github.com/gtsteffaniak/filebrowser/issues/836
 - ctrl-click cache issue https://github.com/gtsteffaniak/filebrowser/issues/735
 - fix admin user reset OIDC user https://github.com/gtsteffaniak/filebrowser/issues/811 https://github.com/gtsteffaniak/filebrowser/issues/851
 - fix windows and binary muPdf issue https://github.com/gtsteffaniak/filebrowser/issues/744
 - fix logout oidc issue https://github.com/gtsteffaniak/filebrowser/issues/829 https://github.com/gtsteffaniak/filebrowser/issues/662
 - file name upload bug https://github.com/gtsteffaniak/filebrowser/issues/662
 - could not create share with absolute timestamps enabled https://github.com/gtsteffaniak/filebrowser/issues/764
 - context menu off screen issue https://github.com/gtsteffaniak/filebrowser/issues/828

## v0.7.10-beta

 **OIDC change**: if you specify `oidc.userIdentifier: "username"`, originally this would map to `preferred_username` but now it maps to `username` explicitly. To maintain the same behavior update your config to `userIdentifier: "preferred_username"`. This was updated to allow for `username` to work as [some might need](https://github.com/gtsteffaniak/filebrowser/pull/789).

 **New Features**:
 - Added settings option to stop sidebar from automatically hiding on editor and previews. https://github.com/gtsteffaniak/filebrowser/issues/744
 - Added more secrets loadable from environment variables. https://github.com/gtsteffaniak/filebrowser/issues/790
 - Include/exclude files are checked for existence to assist with configuration, will show as warning if something is configured but doesn't exist.
 - Added open in new tab link for preview items to view the raw picture, pdf, etc. Especially helpful for safari viewing PDF documents. https://github.com/gtsteffaniak/filebrowser/issues/734
 - Added autoplay media toggle in user profile, to automatically play videos and audio.

 **Notes**:
 - Allowed to delete default admin user https://github.com/gtsteffaniak/filebrowser/issues/811 https://github.com/gtsteffaniak/filebrowser/issues/762
 - Better try/catch error handling for user feedback for shares https://github.com/gtsteffaniak/filebrowser/issues/732

 **BugFixes**:
 - Fix share scope creation issue https://github.com/gtsteffaniak/filebrowser/issues/809
 - Fix oidc token logout issue https://github.com/gtsteffaniak/filebrowser/issues/791
 - Non-admin users OTP issue https://github.com/gtsteffaniak/filebrowser/issues/815
 - Linewrap issue for a few cases https://github.com/gtsteffaniak/filebrowser/issues/810
 - BaseUrl redirect issue with proxies https://github.com/gtsteffaniak/filebrowser/issues/796
 - Fix exclude still shows up in ui issue https://github.com/gtsteffaniak/filebrowser/issues/797
 - Copy/move functions are async https://github.com/gtsteffaniak/filebrowser/issues/812
 - fix subtitle fetch issue https://github.com/gtsteffaniak/filebrowser/issues/766
 - fix location memory issue for url encoded file names

## v0.7.9-beta

 **New Features**:
 - Admin users will get a small notification banner for available update in sidebar with link to new release.

 **Notes**:
 - docker now defaults to ./data/databse.db as the database path allowing a simplified initial docker-compose.yaml. Existing configurations do not need updating.
 - oidc groups header updates admin permission of existing user (either add/remove if role exists)'
 - builds amd64 binary with musl for compatibility (glic error) https://github.com/gtsteffaniak/filebrowser/issues/755
 - renamed `server.sources.config.disabled` to `server.sources.config.disableIndexing`
 - better support for running with disabled index.
 - small indexing behavior tweaks.
 - markdown viewer hides sidebar https://github.com/gtsteffaniak/filebrowser/issues/744
 - quick download only applies to files

 **BugFixes**:
 - subtitles filename issue https://github.com/gtsteffaniak/filebrowser/issues/678
 - search result links not working with custom baseUrl https://github.com/gtsteffaniak/filebrowser/issues/746
 - preview error for office native preview https://github.com/gtsteffaniak/filebrowser/issues/744
 - more source name safety for special characters.
 - shares with special character errors https://github.com/gtsteffaniak/filebrowser/issues/753
 - backspace navigates back a page when typing https://github.com/gtsteffaniak/filebrowser/issues/663
 - markdown viewer scrolling https://github.com/gtsteffaniak/filebrowser/issues/767
 - fix user permissions updated when modifying api key permissions
 - fix language change issue https://github.com/gtsteffaniak/filebrowser/issues/768 https://github.com/gtsteffaniak/filebrowser/issues/487

## v0.7.8-beta

Note: if using oidc, please update from 0.7.7 to resolve invalid_grant issue. Also - oidc no longer creates users automatically by default -- must be enabled.

 **New Features**:
 - More oidc user creation options https://github.com/gtsteffaniak/filebrowser/issues/685
   - `auth.methods.oidc.createUser` must be true to automatically create user, defaults to false.
   - `auth.methods.oidc.adminGroup` allows using oidc provider group name to enable admin user creation.

 **BugFixes**:
 - fix save editor info sometimes saves wrong file. https://github.com/gtsteffaniak/filebrowser/issues/701
 - make ctrl select work on mac or windows. https://github.com/gtsteffaniak/filebrowser/issues/739
 - oidc login failures introduced in 0.7.6 https://github.com/gtsteffaniak/filebrowser/issues/731
 - oidc respects non-default baseURL

## v0.7.7-beta

  This release cleans up some of the native preview (image preview) feature logic. And adds simple docx and epub viewers as well. Going through all of this, I think I know how I can add full-fledge google doc and microsoft office viewer support (no edit). But, for now "onlyOffice" remains the most comprehensive solution with most compatibility and ability to fully edit. One day, I think I will be able to integrate a minimal license-free server into the docker image. But that's something for another time.

  Native preview (image preview) support is also available for linux arm64 and amd64 binaries, and windows exe.

 **New Features**:
 - since theres a wider kind of document preview types, a new disableOfficePreviewExt option has been added.
 - native (and simple) docx and epub viewers.
 - Other documents like xlsx get full size image preview when opened and no onlyoffice support.

 **Notes**:
 - all text mimetype files have preview support.
 - high-quality preview image sizes bumped from 512x512 to 640x640 to help make text previews readable.
 - no config is allowed and defaults to on source at current directory.

 **BugFixes**:
 - fix otp clearing on user save https://github.com/gtsteffaniak/filebrowser/issues/699
 - admin special characters and general login improvements https://github.com/gtsteffaniak/filebrowser/issues/594
 - updated editor caching behavior https://github.com/gtsteffaniak/filebrowser/issues/701
 - move/copy file path issue and overwrite https://github.com/gtsteffaniak/filebrowser/issues/687
 - fix popup preview loading on safari
 - `preview.highQuality` only affects gallery view mode. popop preview is always high quality, and icons are always low quality.

## v0.7.6-beta

 **New Features**:
 - native document preview generation enabled for certain document types on the regular docker image (no office integration needed)
   - supported native document preview types:
     - ".pdf",  // PDF
     - ".xps",  // XPS
     - ".epub", // EPUB
     - ".mobi", // MOBI
     - ".fb2",  // FB2
     - ".cbz",  // CBZ
     - ".svg",  // SVG
     - ".txt",  // TXT
     - ".docx", // DOCX
     - ".ppt",  // PPT
     - ".pptx", // PPTX
     - ".xlsx", // exel XLSX
     - ".hwp",  // HWP
     - ".hwp",  // HWPX
 - proxy logout redirectUrl support via `auth.methods.proxy.logoutRedirectUrl` https://github.com/gtsteffaniak/filebrowser/issues/684

 **Notes**:
 - image loading placeholders added and remain if image can't be loaded.
 - no more arm32 support on main image -- use a `slim` tagged image.

 **BugFixes**:
 - onlyoffice and other cache issues https://github.com/gtsteffaniak/filebrowser/issues/686
 - gallery size indicator centering https://github.com/gtsteffaniak/filebrowser/issues/652

## v0.7.5-beta

 **New Features**
 - new `./filebrowser.exe setup` command for creating a config.yaml on first run. https://github.com/gtsteffaniak/filebrowser/issues/675
 - new 2FA/OTP support for password-based users.
 - `auth.password.enforcedOtp` option to enforce 2FA usage for password users.

 **Notes**:
 - logging uses localtime, optional UTC config added https://github.com/gtsteffaniak/filebrowser/issues/665
 - generated config example now includes defaults https://github.com/gtsteffaniak/filebrowser/issues/590
 - `server.debugMedia` config option added to help debug ffmpeg issues in the future (don't enable unless debugging an issue)
 - more translations additions from english settings https://github.com/gtsteffaniak/filebrowser/issues/653
 - visual tweaks https://github.com/gtsteffaniak/filebrowser/issues/652
 - enhanced markdown viewer with code view spec

 **BugFixes**:
 - long video names ffmpeg issue fixed https://github.com/gtsteffaniak/filebrowser/issues/669
 - certain files not passing content https://github.com/gtsteffaniak/filebrowser/issues/657
 - https://github.com/gtsteffaniak/filebrowser/issues/668
 - allow edit markdown files
 - rename button doesn't close prompt https://github.com/gtsteffaniak/filebrowser/issues/664
 - webm video preview issue https://github.com/gtsteffaniak/filebrowser/issues/673
 - fix signup issue https://github.com/gtsteffaniak/filebrowser/issues/648
 - fix default source bug
 - https://github.com/gtsteffaniak/filebrowser/issues/666
 - fix 500 error for subtitle videos https://github.com/gtsteffaniak/filebrowser/issues/678
 - spaces and special characters in source name issue https://github.com/gtsteffaniak/filebrowser/issues/679

![image](https://github.com/user-attachments/assets/28e4e67e-31a1-4107-9294-0e715e87b558)

## v0.7.4-beta

 **Notes**:
 - Updated German translation. https://github.com/gtsteffaniak/filebrowser/pull/644

 **BugFixes**:
 - windows control click https://github.com/gtsteffaniak/filebrowser/issues/642
 - create user issue https://github.com/gtsteffaniak/filebrowser/issues/647

## v0.7.3-beta

Note: OIDC changes require config update.

 **New Features**
 - Added code highlights to text editor and enabled text editor for all asci files under 25MB
 - Motion previews for videos -- cycles screenshots of vidoes. https://github.com/gtsteffaniak/filebrowser/issues/588
 - Optionally reset default admin username/password on startup, to guarentee a username/password on startup if needed. Use by setting `auth.resetAdminOnStart` true https://github.com/gtsteffaniak/filebrowser/issues/625

 **Notes**:
 - Updated translations everywhere. https://github.com/gtsteffaniak/filebrowser/issues/627
 - Office viewer is now full-screen with floating close button. https://github.com/gtsteffaniak/filebrowser/issues/542
 - OIDC config additions
   - `issuerUrl` required now to get relevant oidc configurations.
   - `disableVerifyTLS` optionally, disable verifying HTTPS provider endpoints.
   - `logoutRedirectUrl` optionally, redirect the user to this URL on logout.
   - other URL config parameters are no longer accepted -- replace with issuerUrl.
 - Aadmins allowed to change user login methods in user settings when creating or updating users.
   - https://github.com/gtsteffaniak/filebrowser/issues/618
   - https://github.com/gtsteffaniak/filebrowser/issues/617
 - Hide header when showing only office https://github.com/gtsteffaniak/filebrowser/issues/542

 **BugFixes**:
 - Editor save shows notification
 - Preview settings resetting on startup
 - Not all languages show correctly https://github.com/gtsteffaniak/filebrowser/issues/623
 - scopes sometimes reset on startup https://github.com/gtsteffaniak/filebrowser/issues/636
 - Update save password option
   - https://github.com/gtsteffaniak/filebrowser/issues/587
   - https://github.com/gtsteffaniak/filebrowser/issues/619
   - https://github.com/gtsteffaniak/filebrowser/issues/615

## v0.7.2-beta

The `media` tags introduced in 0.7.0 have been removed -- all docker images have media enabled now.

  **Notes**:
  - Reverts enforced user login methods types -- until suitable methods to alter are available.
  - When updating a user, updating scope always sets to the exact scope specified on updated.
  - Redirect api messages are INFO instead of WARN
  - Settings has close button instead of back https://github.com/gtsteffaniak/filebrowser/issues/583

  **Bug Fixes**:
  - Hover bug when exact timestamp setting enabled https://github.com/gtsteffaniak/filebrowser/issues/585

## v0.7.1-beta

The `media` tags introduced in 0.7.0 have been removed -- all docker images have media enabled now.

  **Notes**:
  - changes to support jwks url needed for authelia - still needs testing to ensure it works https://github.com/gtsteffaniak/filebrowser/issues/575, added debug logs to help identify any further issues.
  - added apache license file back https://github.com/gtsteffaniak/filebrowser/discussions/599
  - updated toggle view icons to better match.
  - adjusted popup preview position on mobile.
  - updated createUserDir logic, https://github.com/gtsteffaniak/filebrowser/issues/541
    - it always creats user dir (even for admins)
    - scope path must exist if it doesn't end in username, and if it does, the parent dir must exist
    - enforced user login methods types -- can't be changed. a password user cannot login as oidc, etc.

  **Bug Fixes**:
  - right click context menu issue https://github.com/gtsteffaniak/filebrowser/issues/598
  - upload file issue https://github.com/gtsteffaniak/filebrowser/issues/597
  - defaultUserScope is not respected https://github.com/gtsteffaniak/filebrowser/issues/589
  - defaultEnabled is not respected https://github.com/gtsteffaniak/filebrowser/issues/603
  - user has weird navigation barhttps://github.com/gtsteffaniak/filebrowser/issues/593
  - fix multibutton state issue for close overlay https://github.com/gtsteffaniak/filebrowser/issues/596

## v0.7.0-beta

 **New Features**:
 - New authentication method: OIDC (OpenID Connect)
 - UI refresh
   - Refreshed icons and styles to provide more contrast https://github.com/gtsteffaniak/filebrowser/issues/493
   - New scrollbar which includes information about the listing https://github.com/gtsteffaniak/filebrowser/issues/304
   - User-configurable popup previewer and user can control preview size of images.
   - Enhanced user settings page with more toggle options.
   - Replaced checkboxes with toggles switches https://github.com/gtsteffaniak/filebrowser/issues/461
   - Refreshed Breadcrumbs style.
   - Main navbar icon is multipurpose menu, close, back and animates
   - Enhanced source info on the UI
     - User must have permission `realtime: true` property to get realtime events.
     - Sources shows status of the directory `ready`, `indexing`, and `unavailable`
   - Top-right overflow menu for deleting / editing files in peview https://github.com/gtsteffaniak/filebrowser/issues/456
   - Helpful UI animation for drag and drop files, to get feedback where the drop target is.
   - More consistent theme color https://github.com/gtsteffaniak/filebrowser/issues/538
 - New file preview types:
   - Video thumbnails available via new media integration (see configuration wiki for help) https://github.com/gtsteffaniak/filebrowser/issues/351
   - Office file previews if you have office integration enabled. https://github.com/gtsteffaniak/filebrowser/issues/460

  **Notes**:
  - sesssionId is now unique per window. Previously it was shared accross browser tabs.
  - DisableUsedPercentage is a backend property now, so users can't "hack" the information to be shown.
  - Updated documentation for resources api https://github.com/gtsteffaniak/filebrowser/issues/560
  - Updated placeholder for scopes https://github.com/gtsteffaniak/filebrowser/issues/475
  - When user's API permissions are removed, any api keys the user had will be revoked.
  - `server.enableThumbnails` moved to `server.disablePreviews` defaulting to false.
  - `server.resizePreview` moved to `server.resizePreviews` (with an "s" at the end)

  **Bug Fixes**:
  - Nil pointer error when source media is disconnected while running.
  - Source selection buggy https://github.com/gtsteffaniak/filebrowser/issues/537
  - Upload folder structure https://github.com/gtsteffaniak/filebrowser/issues/539
  - Editing files on multiple sources https://github.com/gtsteffaniak/filebrowser/issues/535
  - Prevent the user from changing the password https://github.com/gtsteffaniak/filebrowser/issues/550
  - Links in setting page does not navigate to correct location https://github.com/gtsteffaniak/filebrowser/issues/474
  - Url encoding issue https://github.com/gtsteffaniak/filebrowser/issues/530
  - Certain file types being treated as folders https://github.com/gtsteffaniak/filebrowser/issues/555
  - Source name with special characters https://github.com/gtsteffaniak/filebrowser/issues/557
  - Onlyoffice support on proxy auth https://github.com/gtsteffaniak/filebrowser/issues/559
  - Downloading with user scope https://github.com/gtsteffaniak/filebrowser/issues/564
  - User disableSettings property to be respected.
  - Non admin users updating admin settings.
  - Right click context issue on safari desktop.
  - office save file issue.

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
