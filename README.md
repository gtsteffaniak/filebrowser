## Gtstef / filebrowser

**Note: Intended to be used in docker only.**

This fork makes the following significant changes to filebrowser for origin:

 1. [x] Improves search to use index instead of filesystem.
    - [x] Lightning fast
    - [x] Realtime results as you type
    - [x] Works with file type filter
 1. [ ] Preview enhancements
    - preview default view is constrained to files subwindow,
    which can be toggled to fullscreen.
 1. Improved and simplified GUI
    - 
 1. [x] Updated version and dependencies
    - [x] uses latest npm and node version
    - [x] removes deprecated npm packages
    - [x] Updates golang dependencies
 1. [ ] Moved all configurations to filebrowser.json. no more flags or binary operations to db

## About

Filebrowser provides a file managing interface within a specified directory and it can be used to upload, delete, preview, rename and edit your files. It allows the creation of multiple users and each user can have its own directory. It can be used as a standalone app.

## Install

Using docker:

1. docker run:

```
docker run -it -v /path/to/folder:/srv -p 8080:80 gtstef/filebrowser:0.1.0
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
      - '8080:80'
    image: gtstef/filebrowser:0.1.0
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
    image: gtstef/filebrowser:0.1.0
volumes:
  nas:
    driver_opts:
      type: cifs
      o: "username=myusername,password=mypassword,rw"
      device: "//fileshare/"
```

## Configuration

All configuration is now done via the filebrowser.json config file. This was chosen because it works best with a docker first use case.

Previously the primary way to configure filebrowser was via flags. But this quickly became cumbersome if you had many configurations to make

The other method to configure was via `filebrowser config` commands which would write configurations to a db if it existed already.
When considering