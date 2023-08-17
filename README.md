## Filebrowser

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
    - New search view on desktop
 1. [x] Updated version and dependencies
    - [x] Uses latest npm and node version
    - [x] Removes deprecated npm packages
    - [x] Updates golang dependencies
    - [ ] Remove all unnecessary packages, replaces with generic functions.
 1. [ ] Moved all configurations to filebrowser.json.
  no more flags or binary operations to db
 1. [ ] File browsing uses index first for better performance
    - file details shown only when toggled or needed

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

General UI desktop (dark mode):
![image](https://github.com/gtsteffaniak/filebrowser/assets/42989099/11346953-f3eb-4f2f-a833-1d615e0e38bc)

General UI mobile (dark mode):
![image](https://github.com/gtsteffaniak/filebrowser/assets/42989099/634d3ba6-7ac0-425b-8a83-419743e92fec)

This is how desktop search looks in 0.1.4:
![image](https://github.com/gtsteffaniak/filebrowser/assets/42989099/c8dc8af1-6869-4736-9092-c47f735bbdc0)

However, mobile search still appears very similar to filebrowser/filebrowsers original implementation:
![image](https://github.com/gtsteffaniak/filebrowser/assets/42989099/e179b821-f4e2-4568-b895-4e00de371637)

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

Note: still a WIP migrating configuration to json.

All configuration is now done via the filebrowser.json config file.
This was chosen because it works best with a docker first use case.

Previously the primary way to configure filebrowser was via flags.
But this quickly became cumbersome if you had many configurations to make

The other method to configure was via `filebrowser config` commands which
would write configurations to a db if it existed already.
When considering
