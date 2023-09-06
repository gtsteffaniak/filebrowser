<p align="center"> 
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/license-MIT-green.svg?color=3F51B5&style=for-the-badge&label=License&logoColor=000000&labelColor=ececec" alt="License: MIT"></a>
</p>
<p align="center">
<img src="frontend/public/img/icons/favicon-32x32.png" width="100" title="Login With Custom URL">
</p>
<h3 align="center">Filebrowser - A modern file manager for the web</h3>
<p align="center">
  <img width="500" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/459937ef-3f14-408d-aef5-899cde4cf3a1" title="Main Screenshot">
</p>

> **NOTE**
Intended for docker use only

> **Warning**
Starting with v0.2.0, *ALL* configuration is done via `filebrowser.yaml` configuration file. `.filebrowser.json` and any flags other than `-c` and `-config` during execution WILL NO LONGER WORK. This is by design, in order to use the v0.2.0 You can mount your directory and initialize a new DB with a new default `filebrowser.yaml` which you can tweak and use in the future. Or you can copy and paste the default startup `filebrowser.yaml` below.

This fork makes the following significant changes to filebrowser for origin:

 1. [x] Improves search to use index instead of filesystem.
    - [x] Lightning fast
    - [x] Realtime results as you type
    - [x] Works with file type filter
    - [x] better desktop search view
 1. [x] Preview enhancements
    - Preview default view is constrained to files subwindow,
    which can be toggled to fullscreen.
 1. [x] Improved and simplified GUI
    - Moved all action buttons to file action bar except for switch-view
    - Simplified navbar to 3 main actions: settings,search, and switch-view
    - New search view on desktop
 1. [x] Updated version and dependencies
    - [x] Uses latest npm and node version
    - [x] Removes deprecated npm packages
    - [x] Updates golang dependencies
    - [x] Remove all unnecessary packages, replaces with generic functions.
 1. [x] **IMPORTANT** Moved all configurations to `filebrowser.yaml`. no more flags or binary operations to db

## About

Filebrowser provides a file managing interface within a specified directory
and it can be used to upload, delete, preview, rename and edit your files.
It allows the creation of multiple users and each user can have its own
directory. It can be used as a standalone app.

This repository is a fork, a collection of changes that make this program
work better in terms of asthetics and performance. Improved search,
 simplified ui (without removing features) and more secure and up-to-date
 build are just a few examples.

There are a few more changes needed to get it to a stable status where it
will only recieve security updates. These changes are mentioned above.
Once this is fully complete, the only updates to th

## Look
<p align="center">
  <img width="500" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/35cdeb3b-ab79-4b04-8001-8f51f6ea06bb" title="Dark mode">
  <img width="500" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/8d426356-26cf-407b-b078-bf58f198e799" title="Dark mode2">
  <img width="300" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/37e8f03b-4f5a-4689-aa6c-5cd858a858e9" title="Dark mode">
  <img width="300" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/b04d3c1f-154b-45ba-927c-2112926ad3a9" title="Dark mode">
</p>

## Performance

Search Performance - 100x faster search. However, this will be at expense of RAM. if you have < 1 million
files and folders in the given scope, the RAM usage should be less than 200MB total. RAM requirements
should scale based on the number of directories.

Also , the approx. time to fully index will vary widely based on performance. A sufficiently performant
system should fully index within the first 5 minutes, potentially within the first few seconds.

For example, a low end 11th gen i5 with SSD indexes 86K files within 1 second:

```
2023/08/01 00:08:29 Using config file: /.filebrowser.json
2023/08/01 00:08:29 Indexing files...
2023/08/01 00:08:29 Listening on [::]:8080
2023/08/01 00:08:30 Successfully indexed files.
2023/08/01 00:08:30 Files found       : 85310
2023/08/01 00:08:30 Directories found : 1711
2023/08/01 00:08:30 Indexing scheduler will run every 5 minutes
```

## Install

Using docker:

1. docker run:

```
docker run -it -v /path/to/folder:/srv -p 8080:8080 gtstef/filebrowser
```

1. docker-compose:

  - with local storage

```
version: '3.7'
services:
  filebrowser:
    volumes:
      - '/path/to/folder:/srv'
      #- './database/:/database/'
      - './config.json:/.filebrowser.json'
    ports:
      - '8080:8080'
    image: gtstef/filebrowser
```

  - with network share

```
version: '3.7'
services:
  filebrowser:
    volumes:
      - 'nas:/srv'
      #- './database/:/database/'
      #- './config.json:/.filebrowser.json'
    ports:
      - '8080:80'
    image: gtstef/filebrowser
volumes:
  nas:
    driver_opts:
      type: cifs
      o: "username=myusername,password=mypassword,rw"
      device: "//fileshare/"
```

## Configuration

All configuration is now done via a single configuration file: `filebrowser.yaml`, here is an example [configuration file](./backend/filebrowser.yaml).
### background

The original project filebrowser/filebrowser used multiple different ways to configure the server.
This was confusing and difficult to work with from a user and from a developer's perspective.
So I completely redesigned the program to use one single human-readable config file.
