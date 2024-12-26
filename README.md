<p align="center">
  <a href="https://opensource.org/license/apache-2-0/"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License: Apache-2.0"></a>
</p>
<p align="center">
  <img src="frontend/public/img/icons/favicon-256x256.png" width="100" title="Login With Custom URL">
</p>
<h3 align="center">FileBrowser Quantum - A modern web-based file manager</h3>
<p align="center">
  <img width="800" src="https://github.com/user-attachments/assets/e4a47229-66f8-4838-9575-dd2413596688" title="Main Screenshot">
</p>

> [!Note]
> Starting with v0.3.3, configuration file mapping is different to support non-root user. Now, the default config file name is `config.yaml` and in docker the path is `/home/filebrowser/config.yaml` and `/home/filebrowser/<database_file>`. Please read the usage below to properly update your config to point the new config location. (open an issue for any help needed)

> [!WARNING]
> - There is no stable version yet. Always check release notes for bug fixes on functionality that may have been changed. If you notice any unexpected behavior -- please open an issue to have it fixed soon.

FileBrowser Quantum is a fork of the file browser opensource project with the following changes:

  1. [x] Indexes files efficiently. (See [indexing readme](./docs/indexing.md) for more info.)
     - Real-time search results as you type!
     - Search supports file/folder sizes and many file type filters.
     - Enhanced interactive results that show file/folder sizes.
  2. [x] Revamped and simplified GUI navbar and sidebar menu.
     - Additional compact view mode as well as refreshed view mode
       styles.
     - Many graphical and user experience improvements.
     - right-click context menu
  3. [x] Revamped and simplified configuration via `config.yaml` config file.
  4. [x] Better listing browsing
     - Switching view modes is instant
     - Folder sizes are shown as well
     - Changing Sort order is instant
     - The entire directory is loaded in 1/3 the time
  5. [x] Developer API support
     - Ability to create long-live API Tokens.
     - Helpful Swagger page available at `/swagger` endpoint.

Notable features that this fork *does not* have (removed):

 - jobs/runners are not supported yet (planned).
 - shell commands are completely removed and will not be returned.
 - Themes and branding are not fully supported yet (planned).
 - see feature matrix below for more.
 - pagination for directory items, so large directories with more than 100,000 items may be slow to load or not load at all.

## About

FileBrowser Quantum provides a file-managing interface within a specified directory
and can be used to upload, delete, preview, rename, and edit your files.
It allows the creation of multiple users and each user can have its 
directory.

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

For more questions, see the [Q&A Readme](./docs/questions.md)

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
  <img width="800" src="https://github.com/user-attachments/assets/75226dc4-9802-46f0-9e3c-e4403d3275da" title="Main Screenshot">
</p>

## Install

Using docker:

1. docker run (no persistent db):

```
docker run -it -v /path/to/folder:/srv -v $(pwd)/config.yaml:/home/filebrowser/config.yaml -p 80:80 gtstef/filebrowser
```

or optionally, as non-root filebrowser user:

```
docker run -u filebrowser -it -v $(pwd)/config.yaml:/home/filebrowser/config.yaml -v /path/to/folder:/srv -p 80:80 gtstef/filebrowser
```

1. docker compose:

  - with local storage

```
services:
  filebrowser:
    volumes:
      - '/path/to/folder:/srv' # required (for now not configurable)
      # optional if you want db to persist - configure a path under "database" dir in config file.
      - './database:/home/filebrowser/database'
      - './config.yaml:/home/filebrowser/config.yaml'
    ports:
      - '80:80'
    image: gtstef/filebrowser
    # optionally run as non-root filebrowser user
    #user: filebrowser
    restart: always
```

  - with network share

```
services:
  filebrowser:
    volumes:
      - 'storage:/srv' # required (for now not configurable)
      # optional if you want db to persist - configure a path under "database" dir in config file.
      - './database:/home/filebrowser/database'
      - './config.yaml:/home/filebrowser/config.yaml'
    ports:
      - '80:80'
    image: gtstef/filebrowser
    restart: always
volumes:
  storage:
    driver_opts:
      type: cifs
      o: "username=admin,password=password,rw" # enter valid info here
      device: "//192.168.1.100/share/"         # enter valid info here

```

Not using docker (not recommended), download your binary from releases and run with your custom config file:

```
./filebrowser -c <config.yaml or other /path/to/config.yaml>
```

## Command Line Usage

There are very few commands available. There are 3 actions done via the command line:

1. Running the program, as shown in the install step. The only argument used is the config file if you choose to override the default "config.yaml"
2. Checking the version info via `./filebrowser version`
3. Updating the DB, which currently only supports adding users via `./filebrowser set -u username,password [-a] [-s "example/scope"]`

## API Usage

API tokens can be created to perform actions, access file information, and update user settings just like what can be done from the UI. You can create API tokens from the settings page via "API Management" section. This section will only show up if the user has "API" permissions, which can be granted by editing the user in user management.

Regardless of whether a user has API permissions, anyone can visit the swagger page which is found at `/swagger`. This swagger page uses a short-live token (2-hour exp) that the UI uses, but allows for quick access to all the API's and their described usage and requirements:

![image](https://github.com/user-attachments/assets/12abd1f6-21d3-4437-98ed-9b0da6cf2c73)

When using the API outside of swagger, you will need to set the API token as a bearer token authentication type. This means the authorization header will look like `Authorization: Bearer <token>`. For example in Postman:

Successful Request:

<p align="center"><img width="500" alt="image" src="https://github.com/user-attachments/assets/4f18fa8a-8d87-4f40-9dc7-3d4407769b59"></p>

Failed Request

<p align="center"><img width="500" alt="image" src="https://github.com/user-attachments/assets/4da0deae-f93d-4d94-83b1-68806afb343a"></p>


## Configuration

All configuration is now done via a single configuration file:
`config.yaml`, here is an example of minimal [configuration
file](./backend/config.yaml).

View the [Configuration Help Page](./docs/configuration.md) for available
configuration options and other help.


## Migration from the original filebrowser

I would recommend that you start fresh without reusing the database. However, 
If you want to migrate your existing database to FileBrowser Quantum, 
visit the [migration 
readme](./docs/migration.md)

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
Branding support              | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ |
activity log                  | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
Comments support              | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
collaboration on same file    | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
trash support                 | ❌ | ❌ | ❌ | ✅ | ✅ | ✅ |
Starred/pinned files          | ❌ | ❌ | ❌ | ❌ | ✅ | ✅ |
Content preview icons         | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ |
Plugins support               | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ |
Chromecast support            | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ |

## Roadmap

see [Roadmap Page](./docs/roadmap.md)
