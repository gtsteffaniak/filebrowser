<p align="center">
  <a href="https://opensource.org/license/apache-2-0/"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License: Apache-2.0"></a>
</p>
<p align="center">
<img src="frontend/public/img/icons/favicon-256x256.png" width="100" title="Login With Custom URL">
</p>
<h3 align="center">Filebrowser - A modern file manager for the web</h3>
<p align="center">
  <img width="500" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/b45683b0-bd55-4430-9831-650fe0d21eb8" title="Main Screenshot">
</p>

> [!NOTE]  
> Only intended to be used with docker.

> [!WARNING]
> Starting with v0.2.0, *ALL* configuration is done via `filebrowser.yaml` configuration file.

This fork makes the following significant changes to filebrowser for origin:

 1. [x] Better search.
    - Lightning fast
    - realtime results as you type
    - Works with more type filters
    - interactive results page.
 1. [x] Revamped and simplified GUI navbar and sidebar menu.
 1. [x] **IMPORTANT** Revamped configuration via `filebrowser.yml` config file.
 1. [x] More configurations possible at a per-user level
    - <img width="450" alt="image" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/625bd7c4-5ee9-4011-aaae-2a388ab0813b">
 1. [x] Additional compact view mode as well as refreshed view mode styles.
    
## About

Filebrowser provides a file managing interface within a specified directory
and it can be used to upload, delete, preview, rename and edit your files.
It allows the creation of multiple users and each user can have its own
directory. It can be used as a standalone app.

This repository is a fork, a collection of changes that make this program
work better in terms of asthetics and performance. Improved search,
 simplified ui (without removing features) and more secure and up-to-date
 build are just a few examples.

## Look
<p align="center">
  <img width="500" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/35cdeb3b-ab79-4b04-8001-8f51f6ea06bb" title="Dark mode">
<img width="500" alt="image" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/55fa4f5c-440e-4a97-b711-96139208a163">
<img width="500" alt="image" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/c76f4100-949b-4e17-a3e6-e410fb8ec08f">
<img width="500" alt="image" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/0bde26f3-fa90-411e-bd0b-abaa47506d62">
<img width="560" alt="image" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/71d8f2b8-6fe6-4fdc-8aac-503d08c28d86">


</p>


## Install

Using docker:

1. docker run (no persistent db):

```
docker run -it -v /path/to/folder:/srv -p 80:8080 gtstef/filebrowser
```

1. docker-compose:

  - with local storage

```
version: '3.7'
services:
  filebrowser:
    volumes:
      - '/path/to/folder:/srv' # required (for now not configurable)
      - './database:/database'  # optional if you want db to persist - configure a path under "database" dir in config file.
      - './filebrowser.yaml:/filebrowser.yaml' # required
    ports:
      - '80:8080'
    image: gtstef/filebrowser
    restart: always
```

  - with network share

```
version: '3.7'
services:
  filebrowser:
    volumes:
      - 'storage:/srv' # required (for now not configurable)
      - './database:/database'  # optional if you want db to persist - configure a path under "database" dir in config file.
      - './filebrowser.yaml:/filebrowser.yaml' # required
    ports:
      - '80:8080'
    image: gtstef/filebrowser
    restart: always
volumes:
  storage:
    driver_opts:
      type: cifs
      o: "username=admin,password=password,rw" # enter valid info here
      device: "//192.168.1.100/share/"         # enter valid hinfo here

```

## Configuration

All configuration is now done via a single configuration file: `filebrowser.yaml`, here is an example minimal [configuration file](./backend/filebrowser.yaml).

View the [Configuration Help Page](./configuration.md) for available configuration options and other help.

### background & help

The original project filebrowser/filebrowser used multiple different ways to configure the server.
This was confusing and difficult to work with from a user and from a developer's perspective.
So I completely redesigned the program to use one single human-readable config file.

I understand many coming from the original fork may notice differences which make using this improved version more difficult. If you notice issues that you believe should be fixed, please open an issue here and it will very likely be addressed with a PR within a few weeks.

This version of filebrowser is going through a configuration overhaul as mentioned above. Certain features related to rules and commands may not work as they do on the original filebrowser. The purpose of this is to create a more consistent experience where configuration is done via files rather than running commands, so that it's very clear what the current state of the configuration is. When running commands its not clear what the configuration is.

## Roadmap

see [Roadmap Page](./roadmap.md)

