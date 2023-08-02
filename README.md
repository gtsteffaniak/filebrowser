## Gtstef / filebrowser

**Note: Intended to be used in docker only.**

This fork makes the following significant changes to filebrowser for origin:

 1. [x] Improves search to use index instead of filesystem.
    - [x] Lightning fast
    - [x] Realtime results as you type
    - [x] Works with file type filter
    - [x] better desktop search view
 1. [ ] Preview enhancements
    - Preview default view is constrained to files subwindow,
    which can be toggled to fullscreen.
 1. [x] Improved and simplified GUI
    - Moved all action buttons to file action bar except for switch-view
    - Simplified navbar to 3 main actions: settings,search, and switch-view
 1. [x] Updated version and dependencies
    - [x] Uses latest npm and node version
    - [x] Removes deprecated npm packages
    - [x] Updates golang dependencies
 1. [ ] Moved all configurations to filebrowser.json.
  no more flags or binary operations to db

## About

Filebrowser provides a file managing interface within a specified directory 
and it can be used to upload, delete, preview, rename and edit your files. 
It allows the creation of multiple users and each user can have its own 
directory. It can be used as a standalone app.

## Look

This is how desktop search looks in 0.1.3, the styling will be further refined in the next version" 
![image](https://github.com/gtsteffaniak/filebrowser/assets/42989099/761f2a08-cafb-4f79-90fe-48fa50679f48)

However mobile search still appears very similar to filebrowser/filebrowsers original implementation:
![image](https://github.com/gtsteffaniak/filebrowser/assets/42989099/03af7760-73a0-4a5d-ab32-84815e455245)

search categories are improved:

![image](https://github.com/gtsteffaniak/filebrowser/assets/42989099/5572ef20-3047-43b9-92f8-95c4ce6f12b5)

## Install

Using docker:

1. docker run:

```
docker run -it -v /path/to/folder:/srv -p 8080:80 gtstef/filebrowser:0.1.3
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
    image: gtstef/filebrowser:0.1.3
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
    image: gtstef/filebrowser:0.1.3
volumes:
  nas:
    driver_opts:
      type: cifs
      o: "username=myusername,password=mypassword,rw"
      device: "//fileshare/"
```

## Configuration

All configuration is now done via the filebrowser.json config file. 
This was chosen because it works best with a docker first use case.

Previously the primary way to configure filebrowser was via flags. 
But this quickly became cumbersome if you had many configurations to make

The other method to configure was via `filebrowser config` commands which 
would write configurations to a db if it existed already.
When considering
