<div align="center">

  [![Go Report Card](https://goreportcard.com/badge/github.com/gtsteffaniak/filebrowser/backend)](https://goreportcard.com/report/github.com/gtsteffaniak/filebrowser/backend)
  [![Codacy Badge](https://app.codacy.com/project/badge/Grade/1c48cfb7646d4009aa8c6f71287670b8)](https://www.codacy.com/gh/gtsteffaniak/filebrowser/dashboard)
  [![latest version](https://img.shields.io/github/release/gtsteffaniak/filebrowser/all.svg)](https://github.com/gtsteffaniak/filebrowser/releases)
  [![DockerHub Pulls](https://img.shields.io/docker/pulls/gtstef/filebrowser?label=latest%20Docker%20pulls)](https://hub.docker.com/r/gtstef/filebrowser)
  [![Apache-2.0 License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)

  [![Donate](https://www.paypalobjects.com/en_US/i/btn/btn_donate_SM.gif)](https://github.com/gtsteffaniak/filebrowser/wiki/Q&A#is-there-a-way-to-donate-or-support-this-project)

  <img width="150" src="https://github.com/user-attachments/assets/c40b22c9-33da-47b7-bc4c-ce69bb5cc174" title="Logo">
  <h3>FileBrowser Quantum</h3>
  The best free self-hosted web-based file manager.
  <br/><br/>
  <img width="800" src="https://github.com/user-attachments/assets/162d7a95-33b7-49bd-976c-dd6822c0d22b">
</div>

## Pinned

:loudspeaker: [What's Coming Soon](https://github.com/gtsteffaniak/filebrowser/discussions/1622)
:pushpin: [Read The Official Docs](https://filebrowserquantum.com/) (currently english-only)

## About

FileBrowser Quantum provides an easy way to access and manage your files from the web. It has a modern responsive interface that has many advanced features to manage users, access, sharing, and file preview and editing.

This version is called "Quantum" because it packs tons of advanced features into a tiny easy to run file. Unlike the majority of alternative options, FileBrowser Quantum is simple to install and easy to configure.

The goal for this repo is to become the best open-source self-hosted file browsing application that exists -- **all for free**. This repo will always be free and open-source.

Ready to try it out? See [Getting Started Docs](https://filebrowserquantum.com/en/docs/getting-started/).

## How its different

FileBrowser Quantum is a massive fork of the file browser open-source project with the following changes:

  1. ✅ Add and configure multiple sources
  2. ✅ Login support for OIDC, password + 2FA, and proxy.
  3. ✅ Beautiful, Responsive, and Customizable user interface.
  4. ✅ Streamlined configuration via `config.yaml` config file.
  5. ✅ Ultra-efficient [indexing](https://github.com/gtsteffaniak/filebrowser/wiki/Indexing) and real-time updates
     - Real-time search results as you type.
     - Real-time monitoring and updates in the UI.
     - Search supports file and folder sizes, along with various filters.
  6. ✅ Better listing browsing
     - Better thumbnail support including **office**, **video**, and **album artwork**
     - Faster and more responsive views with animations.
     - Folder sizes are displayed and support for thumbnails
     - Navigating remembers the last scroll position.
  7. ✅ Highly configurable and customizable sharing options
     - share expiration time
     - users who can access share (including anonymous)
     - styling and themes
     - file viewing, editing, and uploading permissions
  8. ✅ Directory-level access control that can be scoped to user or group.
  9. ✅ Developer API support
     - Ability to create long-lived API Tokens.
     - A helpful Swagger page is available at `/swagger` endpoint for API enabled users.

Notable features that this fork *does not* have (removed):

 - :construction: jobs are not supported yet.
 - ❌ shell commands are completely removed and will not be returned.

FileBrowser Quantum differs significantly from the original version. Many of these changes required a significant overhaul. Creating a fork was a necessary process to make the program better. There have been many growing pains, but a stable release is planned and coming soon.

## System Requirements

> [!WARNING]
> Every file and directory in the source gets indexed (by default). This enables powerful features such as instant search, but large source filesystems can increase your system requirements. [See indexing wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Indexing) for more info.

 - **Memory**: depends on configured source complexity. See [how much RAM does it require?](https://github.com/gtsteffaniak/filebrowser/discussions/787)
 - **GPU**: Not currently used (planned)

## The UI

The UI has a simple three-component navigation system:

  1. (Left) Multi-action button with slide-out panel.
  2. (Middle) The powerful search bar.
  3. (Right) The view change toggle.

All other functions are moved either into the action menu or pop-up menus.
If the action does not depend on context, it will exist in the slide-out
action panel. If the action is available based on context, it will show up as
a pop-up menu.

<p align="center">
  <img width="1000" src="https://github.com/user-attachments/assets/aa32b05c-f917-47bb-b07f-857edc5e47f7" title="Search GIF">
</p>

## Official Docs

See the [Official Docs](https://filebrowserquantum.com/). Contributions are welcome and encouraged! See [FilebrowserDocs Github](https://github.com/quantumx-apps/filebrowserDocs).

## Comparison Chart
Application Name | <img width="48" src="https://github.com/user-attachments/assets/c40b22c9-33da-47b7-bc4c-ce69bb5cc174" > Quantum | <img width="48" src="https://github.com/filebrowser/filebrowser/blob/master/frontend/public/img/logo.svg" > Filebrowser | <img width="48" src="https://github.com/mickael-kerjean/filestash/blob/master/public/assets/logo/app_icon.png?raw=true" > Filestash | <img width="48" src="https://avatars.githubusercontent.com/u/19211038?s=200&v=4" >  Nextcloud | <img width="48" src="https://upload.wikimedia.org/wikipedia/commons/thumb/d/da/Google_Drive_logo.png/480px-Google_Drive_logo.png" > Google_Drive | <img width="48" src="https://avatars.githubusercontent.com/u/6422152?v=4" > FileRun
--- | --- | --- | --- | --- | --- | --- |
Filesystem support            | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
Linux                         | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
Windows                       | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
Mac                           | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
Self hostable                 | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
Has Stable Release?           | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
S3 support                    | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ |
webdav support                | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ |
FTP support                   | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ |
Dedicated docs site?          | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
Multiple sources at once      | ✅ | ❌ | ✅ | ✅ | ❌ | ✅ |
Docker image size             | 180 MB (with ffmpeg) | 31 MB  | 240 MB (main image) | 250 MB | ❌ | > 2 GB |
Min. Memory Requirements      | 256 MB | 128 MB | 128 MB (main image) | 512 MB | ❌ | 512 MB   |
has standalone binary         | ✅ | ✅ | ❌ | ❌ | ❌ | ❌ |
price                         | free | free | free | free tier | free tier | $99+ |
rich media preview            | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
Upload files from the web?    | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
Advanced Search?              | ✅ | ❌ | ❌ | configurable | ✅ | ✅ |
Indexed Search?               | ✅ | ❌ | ❌ | configurable | ✅ | ✅ |
Content-aware search?         | ❌ | ❌ | ❌ | configurable | ✅ | ✅ |
Custom job support            | :construction: | ✅ | ❌ | ✅ | ❌ | ✅ |
Multiple users                | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
Single sign-on support        | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ |
LDAP sign-on support          | :construction: | ❌ | ❌ | ✅ | ❌ | ✅ |
Long-live API key support     | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ |
API documentation page        | ✅ | ❌ | ✅ | ✅ | ❌ | ✅ |
Mobile App                    | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ |
open source?                  | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
tags support                  | :construction: | ❌ | ❌ | ✅ | ❌ | ✅ |
shareable web links?          | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
Event-based notifications     | :construction: | ❌ | ❌ | ❌ | ❌ | ✅ |
Metrics                       | :construction: | ❌ | ❌ | ❌ | ❌ | ❌ |
file space quotas             | :construction: | ❌ | ❌ | ❌ | ✅ | ✅ |
text-based files editor       | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
Office file support           | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ |
Office file previews          | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ |
Themes                        | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
Branding support              | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
activity log                  | :construction: | ❌ | ❌ | ✅ | ✅ | ✅ |
Comments support              | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
trash support                 | :construction: | ❌ | ❌ | ✅ | ✅ | ✅ |
Starred/pinned files          | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
Chromecast support            | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ |
Share collections of files    | :construction: | ❌ | ❌ | ❌ | ❌ | ✅ |
Can archive selected files    | :construction: | ❌ | ❌ | ❌ | ❌ | ✅ |
Can browse archive files      | :construction: | ❌ | ❌ | ❌ | ❌ | ✅ |
Can convert documents         | :construction: | ❌ | ❌ | ❌ | ❌ | ✅ |
Can convert videos            | :construction: | ❌ | ❌ | ❌ | ❌ | ❌ |
Can convert photos            | :construction: | ❌ | ❌ | ❌ | ❌ | ❌ |
