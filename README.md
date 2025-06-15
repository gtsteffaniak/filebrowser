<div align="center">

  [![Go Report Card](https://goreportcard.com/badge/github.com/gtsteffaniak/filebrowser/backend)](https://goreportcard.com/report/github.com/gtsteffaniak/filebrowser/backend)
  [![Codacy Badge](https://app.codacy.com/project/badge/Grade/1c48cfb7646d4009aa8c6f71287670b8)](https://www.codacy.com/gh/gtsteffaniak/filebrowser/dashboard)
  [![latest version](https://img.shields.io/github/release/gtsteffaniak/filebrowser/all.svg)](https://github.com/gtsteffaniak/filebrowser/releases)
  [![DockerHub Pulls](https://img.shields.io/docker/pulls/gtstef/filebrowser?label=latest%20Docker%20pulls)](https://hub.docker.com/r/gtstef/filebrowser)
  [![Apache-2.0 License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)

  [![Donate](https://www.paypalobjects.com/en_US/i/btn/btn_donate_SM.gif)](https://www.paypal.com/donate/?business=W5XKNXHJM2WPE&no_recurring=0&currency_code=USD)

  <img width="150" src="https://github.com/user-attachments/assets/59986a2a-f960-4536-aa35-4a9a7c98ad48" title="Logo">
  <h3>FileBrowser Quantum</h3>
  The best free self-hosted web-based file manager.
  <br/><br/>
  <img width="800" src="https://github.com/user-attachments/assets/95af22fa-7760-4f7b-9c20-51fdcfe8f4ea" title="Main Screenshot">
</div>

> [!WARNING]
> There is no stable version -- :construction: planned for 2025. ([Read more](https://github.com/gtsteffaniak/filebrowser/discussions/628))

Poll: [how much RAM does your install use?](https://github.com/gtsteffaniak/filebrowser/discussions/787)

## About

FileBrowser Quantum provides an easy way to access and manage your files from the web. It has has a web page interface that allows you to create secure shared links, users with their own specific permissions and settings, and offers a great viewing experience for many file types.

This version is called "Quantum" because it packs tons of advanced features into a tiny easy to run file. Unlike the majority of alternative options, FileBrowser Quantum is simple to install and easy to configure.

The goal for this repo is to become the best open-source self-hosted file browsing application that exists -- **all for free**. This repo will always be free and open-source.

Ready to try it out? See [Getting Started Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Getting-Started).

## How its different

FileBrowser Quantum is a massive fork of the file browser open-source project with the following changes:

  1. ✅ Multiple sources support
  2. ✅ Login support for OIDC, password + 2FA, and proxy.
  3. ✅ Revamped UI
  4. ✅ Simplified configuration via `config.yaml` config file.
  5. ✅ Ultra-efficient [indexing](https://github.com/gtsteffaniak/filebrowser/wiki/Indexing) and real-time updates
     - Real-time search results as you type.
     - Real-time monitoring and updates in the UI.
     - Search supports file and folder sizes, along with various filters.
  6. ✅ Better listing browsing
     - More file type previews, such as **office** and **video** file previews
     - Instantly switches view modes and sort order without reloading data.
     - Folder sizes are displayed.
     - Navigating remembers the last scroll position.
  7. ✅ Developer API support
     - Ability to create long-lived API Tokens.
     - A helpful Swagger page is available at `/swagger` endpoint for API enabled users.

Notable features that this fork *does not* have (removed):

 - :construction: jobs are not supported yet.
 - :construction: rules are not supported yet.
 - ❌ shell commands are completely removed and will not be returned.

> [!WARNING]
> Every file and directory in the source gets indexed (by default). This enables powerful features such as instant search, but large source filesystems can increase your system requirements. [See indexing wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Indexing) for more info.

FileBrowser Quantum differs significantly from the original version. Many of these changes required a significant overhaul. Creating a fork was a necessary process to make the program better. There have been many growing pains, but a stable release is planned and coming soon.

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

## Install and Configuration

Check out the [Getting Started Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Getting-Started)

For help configuring your filebrowser see [Configuration Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Configuration-And-Examples)

## Command Line Usage

See the [CLI Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/CLI)

## API Usage

See the [API Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/API)

## Office File Support

See [Office Support Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Office-Support#adding-open-office-integration-for-docker) on how to enable office file editing and office-related features.

## Migration from the original filebrowser

See the [Migration Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Migration)

## Other Questions

For more, see the [Q&A Wiki](https://github.com/gtsteffaniak/filebrowser/wiki/Q&A)

## Comparison Chart

 Application Name | <img width="48" src="https://github.com/user-attachments/assets/59986a2a-f960-4536-aa35-4a9a7c98ad48" > Quantum | <img width="48" src="https://github.com/filebrowser/filebrowser/blob/master/frontend/public/img/logo.svg" > Filebrowser | <img width="48" src="https://github.com/mickael-kerjean/filestash/blob/master/public/assets/logo/app_icon.png?raw=true" > Filestash | <img width="48" src="https://avatars.githubusercontent.com/u/19211038?s=200&v=4" >  Nextcloud | <img width="48" src="https://upload.wikimedia.org/wikipedia/commons/thumb/d/da/Google_Drive_logo.png/480px-Google_Drive_logo.png" > Google_Drive | <img width="48" src="https://avatars.githubusercontent.com/u/6422152?v=4" > FileRun
--- | --- | --- | --- | --- | --- | --- |
Filesystem support            | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
Linux                         | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
Windows                       | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
Mac                           | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
Self hostable                 | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
Has Stable Release?           | :construction: | ✅ | ✅ | ✅ | ✅ | ✅ |
S3 support                    | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ |
webdav support                | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ |
FTP support                   | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ |
Dedicated docs site?          | :construction: | ✅ | ✅ | ✅ | ❌ | ✅ |
Multiple sources at once      | ✅ | ❌ | ✅ | ✅ | ❌ | ✅ |
Docker image size             | 200 MB (with ffmpeg) | 31 MB  | 240 MB (main image) | 250 MB | ❌ | > 2 GB |
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
