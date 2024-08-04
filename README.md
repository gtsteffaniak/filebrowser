<p align="center">
  <a href="https://opensource.org/license/apache-2-0/"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="License: Apache-2.0"></a>
</p>
<p align="center">
  <img src="frontend/public/img/icons/favicon-256x256.png" width="100" title="Login With Custom URL">
</p>
<h3 align="center">Filebrowser - A modern web-based file manager</h3>
<p align="center">
  <img width="800" src="https://github.com/user-attachments/assets/8ba93582-aba2-4996-8ac3-25f763a2e596" title="Main Screenshot">
</p>

> [!WARNING]
> Starting with v0.2.0, *ALL* configuration is done via `filebrowser.yaml`
> Configuration file.
> Starting with v0.2.4 *ALL* share links need to be re-created (due to
> security fix).

This fork makes the following significant changes to filebrowser for
origin:

  1. [x] Better search
     - Lightning fast
     - real-time results as you type
     - Works with more type filters
     - interactive results page.
  2. [x] Revamped and simplified GUI navbar and sidebar menu.
     - Additional compact view mode as well as refreshed view mode
       styles.
  3. [x] Revamped configuration via `filebrowser.yml` config file.
     - More configurations possible at a per-user level

## About

Filebrowser provides a file managing interface within a specified directory
and can be used to upload, delete, preview, rename, and edit your files.
It allows the creation of multiple users and each user can have its 
directory.

This repository is a fork, a collection of changes that make this program
work better in terms of aesthetics and performance. Improved search,
simplified ui (without removing features) and more secure and up-to-date
build are just a few examples.

This Implementation of filebrowser differs significantly to the original.
There are hundreds of thousands of lines changed and they are generally
no longer compatible with each other. This has been intentional -- the
focus of this fork is on a few key principles:
  - Simplicity and improved user experience
  - Efficiency of operations and performance
  - Minimize external dependencies and standard library usage.
  - Of course -- adding much-needed features.

## Look

One way you can observe the improved user experience is how I changed
the UI. The Navbar is simplified to a three-component system :

  1. (Left) The slide-out action panel button
  2. (Middle) The powerful search bar.
  3. (Right) The view change toggle.

All other functions are moved either into the action menu or popup menus.
If the action does not depend on context, it will exist in the slide-out
action panel. If the action is available based on context, it will show up as
a popup menu.

<p align="center">
  <img width="800" src="https://github.com/gtsteffaniak/filebrowser/assets/42989099/899152cf-3e69-4179-aa82-752af2df3fc6" title="Main Screenshot">
    <img width="800" src="https://github.com/user-attachments/assets/18c02d03-5c60-4e15-9c32-3cfe058a0c49" title="Main Screenshot">
      <img width="800" src="https://github.com/user-attachments/assets/75226dc4-9802-46f0-9e3c-e4403d3275da" title="Main Screenshot">

</p>

## Install

Using docker:

1. docker run (no persistent db):

```
docker run -it -v /path/to/folder:/srv -p 80:80 gtstef/filebrowser
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
      - '80:80'
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
      - '80:80'
    image: gtstef/filebrowser
    restart: always
volumes:
  storage:
    driver_opts:
      type: cifs
      o: "username=admin,password=password,rw" # enter valid info here
      device: "//192.168.1.100/share/"         # enter valid hinfo here

```

Not using docker (not recommended), download your binary from releases and run with your custom config file:

```
./filebrowser -c <filebrowser.yml or other /path/to/config.yaml>
```

## Configuration

All configuration is now done via a single configuration file:
`filebrowser.yaml`, here is an example of minimal [configuration
file](./backend/filebrowser.yaml).

View the [Configuration Help Page](./configuration.md) for available
configuration options and other help.


## Migration from filebrowser/filebrowser

If you currently use filebrowser from the filebrowser/filebrowser
repo but want to try using this. I recommend you start fresh without
reusing the database, but there are a few things you'll need to do if you
must migrate:

1. Create a configuration file as mentioned above.
2. Copy your database file from the original filebrowser to the path of
   the new one.
3. Update the configuration file to use the database (under server in
   filebrowser.yml)
4. If you are using docker, update the docker-compose file or docker run
   command to use the config file as described in the install section
   above.
5. If you are not using docker, just make sure you run filebrowser -c
   filebrowser.yml and have a valid filebrowser config.


The filebrowser application should run with the same user and rules that
you have from the original. But keep in mind the differences that are
mentioned at the top of this readme.

## Roadmap

see [Roadmap Page](./roadmap.md)
