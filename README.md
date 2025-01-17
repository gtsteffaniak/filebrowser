<p align="center">
  <a href="https://opensource.org/license/apache-2-0/"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License: Apache-2.0"></a>
</p>
<p align="center">
  <img src="frontend/public/img/icons/favicon-256x256.png" width="100" title="Login With Custom URL">
</p>
<h3 align="center">FileBrowser Quantum - A modern web-based file manager</h3>
<p align="center">
  <img width="800" src="https://github.com/user-attachments/assets/b16acd67-0292-437a-a06c-bc83f95758e6" title="Main Screenshot">
</p>

> [!WARNING]
> There is no stable version yet. 
> (planned for later this year after these are complete: multiple sources support, initial onboarding page, official automated docs website)

FileBrowser Quantum is a fork of the file browser open-source project with the following changes:

  1. ✅ Indexes files efficiently. (See [indexing Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Indexing) for more info.)
     - Real-time search results as you type
     - Search supports file/folder sizes and many file type filters.
     - Enhanced interactive results that show file/folder sizes.
  2. ✅ Revamped and simplified GUI navbar and sidebar menu.
     - Additional compact view mode as well as refreshed view mode styles.
     - Many graphical and user experience improvements.
     - right-click context menu
  3. ✅ Revamped and simplified configuration via `config.yaml` config file.
  4. ✅ Better listing browsing
     - Instantly Switches view modes and sort order without reloading data.
     - Folder sizes are displayed
     - Navigating remembers the scroll position, navigating back keeps the last scroll position.
  5. ✅ Developer API support
     - Ability to create long-live API Tokens.
     - Helpful Swagger page available at `/swagger` endpoint.

Notable features that this fork *does not* have (removed):

 - ❌ jobs/runners are not supported yet (planned).
 - ❌ per-user rules are not supported yet (planned).
 - ❌ pagination for directory items for extremely large directories.
 - ❌ shell commands are completely removed and will not be returned.
 - see feature matrix below for more.

## About

FileBrowser Quantum provides a file-managing interface within a specified directory
and can be used to upload, delete, preview, rename, and edit your files.
It allows the creation of multiple users and each user can have its directory.

This repository is a fork of the original [filebrowser](https://github.com/filebrowser/filebrowser)
 with a collection of changes that make this program work better in terms of
 aesthetics and performance. Improved search, simplified UI
 (without removing features) and more secure and up-to-date
 build are just a few examples.

FileBrowser Quantum differs significantly from the original.
There are hundreds of thousands of lines changed and they are generally
no longer compatible with each other. This has been intentional -- the
focus of this fork is on a few key principles:
  - Simplicity and improved user experience
  - Improving performance and faster feedback when making changes.
  - Minimize external dependencies and standard library usage.
  - Of course -- adding much-needed features.

For more, see the [Q&A Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Q&A)

## Look

The UI has a simple three-component navigation system :

  1. (Left) The slide-out action panel button
  2. (Middle) The powerful search bar.
  3. (Right) The view change toggle.

All other functions are moved either into the action menu or popup menus.
If the action does not depend on context, it will exist in the slide-out
action panel. If the action is available based on context, it will show up as
a popup menu.

<p align="center">
  <img width="800" src="https://github.com/user-attachments/assets/2be7a6c5-0f95-4d9f-bc05-484ee71246d8" title="Search GIF">
  <img width="800" src="https://github.com/user-attachments/assets/f55a6f1f-b930-4399-98b5-94da6e90527a" title="Navigation GIF">
  <img width="800" src="https://github.com/user-attachments/assets/93b019de-d38f-4aaa-bde3-3ba4e99ecd25" title="Main Screenshot">
</p>

## Install and Configuration

See the [Configuration Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Configuration)

## Command Line Usage

See the [CLI Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/CLI)

## API Usage

See the [API Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/API)

## Configuration

Configuration is done via the `config.yaml`, see the [Configuration Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Configuration) for available configuration options and other help.


## Migration from the original filebrowser

See the [Migration Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Migration)

## Comparison Chart

 Application Name | <img width="48" src="frontend/public/img/icons/favicon-256x256.png" > Quantum | <img width="48" src="https://github.com/filebrowser/filebrowser/blob/master/frontend/public/img/logo.svg" > Filebrowser | <img width="48" src="https://github.com/mickael-kerjean/filestash/blob/master/public/assets/logo/app_icon.png?raw=true" > Filestash | <img width="48" src="https://avatars.githubusercontent.com/u/19211038?s=200&v=4" >  Nextcloud | <img width="48" src="https://upload.wikimedia.org/wikipedia/commons/thumb/d/da/Google_Drive_logo.png/480px-Google_Drive_logo.png" > Google_Drive | <img width="48" src="https://avatars.githubusercontent.com/u/6422152?v=4" > FileRun
--- | --- | --- | --- | --- | --- | --- |
Filesystem support            | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
Linux                         | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
Windows                       | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
Mac                           | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
Self hostable                 | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
Has Stable Release?           | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
S3 support                    | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ |
webdav support                | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ |
ftp support                   | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ |
Dedicated docs site?          | ❌ | ✅ | ✅ | ✅ | ❌ | ✅ |
Multiple sources at once      | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ |
Docker image size             | 31 MB  | 31 MB  | 240 MB (main image) | 250 MB | ❌ | > 2 GB |
Min. Memory Requirements      | 128 MB | 128 MB | 128 MB (main image) | 128 MB | ❌ | 4 GB   |
has standalone binary         | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
price                         | free | free | free | free tier | free tier | $99+ |
rich media preview            | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
upload files from the web?    | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
Advanced Search?              | ✅ | ❌ | ❌ | configurable | ✅ | ✅ |
Indexed Search?               | ✅ | ❌ | ❌ | configurable | ✅ | ✅ |
Content-aware search?         | ❌ | ❌ | ❌ | configurable | ✅ | ✅ |
Custom job support            | ❌ | ✅ | ❌ | ✅ | ❌ | ✅ |
Multiple users                | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
Single sign-on support        | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
LDAP sign-on support          | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
2FA sign-on support           | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
Long-live API key support     | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ |
API documentation page        | ✅ | ❌ | ✅ | ✅ | ❌ | ✅ |
Mobile App                    | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ |
open source?                  | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
tags support                  | ❌ | ❌ | ❌ | ✅ | ❌ | ✅ |
sharable web links?           | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
Event-based notifications     | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
Metrics                       | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
file space quotas             | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
text-based files editor       | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
office file support           | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ |
Themes                        | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
Branding support              | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
activity log                  | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
Comments support              | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
collaboration on same file    | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
trash support                 | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
Starred/pinned files          | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
Content preview icons         | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ |
Plugins support               | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ |
Chromecast support            | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ |
Share collections of files    | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
Can archive selected files    | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ |
Can browse archive files      | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ 