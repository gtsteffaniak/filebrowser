# Configuration Help

This document covers the available configuration options, their defaults, and how they affect the functionality of filebrowser.

## All possible configurations

Here is an expanded config file which includes all possible configurations:

```
server:
  indexingInterval: 5
  numImageProcessors: 4
  socket: ""
  tlsKey: ""
  tlsCert: ""
  enableThumbnails: false
  resizePreview: true
  typeDetectionByHeader: true
  port: 8080
  baseURL: "/"
  address: ""
  log: "stdout"
  database: "/database/database.db"
  root: "/srv"
auth:
  recaptcha:
    host: ""
    key: ""
    secret: ""
  header: ""
  method: json
  command: ""
  signup: false
  shell: ""
frontend:
  name: ""
  disableExternal: false
  disableUsedPercentage: true
  files: ""
  theme: ""
  color: ""
userDefaults:
  scope: ""
  locale: ""
  viewMode: ""
  singleClick: true
  sorting:
    by: ""
    asc: true
  permissions:
    admin: true
    execute: true
    create: true
    rename: true
    modify: true
    delete: true
    share: true
    download: true
  commands: []
  hideDotfiles: false
  dateFormat: false
```

Here are the defaults if nothing is set:

```
Signup: true
AdminUsername: admin
AdminPassword: admin
Server:
  EnableThumbnails: true
  EnableExec: false
  IndexingInterval: 5
  Port: 8080
  NumImageProcessors: 4
  BaseURL: ""
  Database: database.db
  Log: stdout
  Root: /srv
Auth:
  Method: password
  Recaptcha:
    Host: ""
UserDefaults:
  Scope: "."
  LockPassword: false
  HideDotfiles: true
  Permissions:
    Create: true
    Rename: true
    Modify: true
    Delete: true
    Share: true
    Download: true
```

## About each configuration

### Server configuration settings

  - `indexingInterval`: This is the time in minutes the system waits before checking for filesystem changes. (used in search only)
  - `numImageProcessors`: 
  socket: ""
  tlsKey: ""
  tlsCert: ""
  enableThumbnails: false
  resizePreview: true
  typeDetectionByHeader: true
  port: 8080
  baseURL: "/"
  address: ""
  log: "stdout"
  - `database`: This is the database file path + filename that will be created if it does not already exist. If it exists, it will use the existing file.
  - `root`: "/srv"