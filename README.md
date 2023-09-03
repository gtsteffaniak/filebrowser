## Gtstef / filebrowser

> **NOTE**
Intended for docker use only

> **Warning**
Starting with v0.2.0, *ALL* configuration is done via `filebrowser.yaml` configuration file. `.filebrowser.json` and any flags used during execution WILL NO LONGER WORK. This is by design, in order to use the v0.2.0 You can mount your directory and initialize a new DB with a new default `filebrowser.yaml` which you can tweak and use in the future. Or you can copy and paste the default startup `filebrowser.yaml` below.

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

This is how desktop search looks in 0.1.3, the styling will be further refined in the next version.
![image](https://github.com/gtsteffaniak/filebrowser/assets/42989099/761f2a08-cafb-4f79-90fe-48fa50679f48)

However mobile search still appears very similar to filebrowser/filebrowsers original implementation:
![image](https://github.com/gtsteffaniak/filebrowser/assets/42989099/03af7760-73a0-4a5d-ab32-84815e455245)

search categories are improved:

![image](https://github.com/gtsteffaniak/filebrowser/assets/42989099/5572ef20-3047-43b9-92f8-95c4ce6f12b5)

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
