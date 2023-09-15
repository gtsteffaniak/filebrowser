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

# Configuration Settings Documentation

## About each configuration

- `Signup`: This boolean value indicates whether user signup is enabled.

- `AdminUsername`: This is the username of the admin user.

- `AdminPassword`: This is the password of the admin user.

### Server configuration settings

- `indexingInterval`: This is the time in minutes the system waits before checking for filesystem changes (used in search only).

- `numImageProcessors`: This is the number of image processors available.

- `socket`: This is the socket configuration.

- `tlsKey`: This is the TLS key configuration.

- `tlsCert`: This is the TLS certificate configuration.

- `enableThumbnails`: This boolean value determines whether thumbnails are enabled.

- `resizePreview`: This boolean value determines whether preview resizing is enabled.

- `typeDetectionByHeader`: This boolean value determines whether type detection is based on headers.

- `port`: This is the port number on which the server is running.

- `baseURL`: This is the base URL for the server.

- `address`: This is the server address configuration.

- `log`: This specifies the log destination (e.g., "stdout" for standard output).

- `database`: This is the database file path + filename that will be created if it does not already exist. If it exists, it will use the existing file.

- `root`: This is the root directory path.

### Auth configuration settings

- `recaptcha`:

  - `host`: This is the host for reCAPTCHA.

  - `key`: This is the reCAPTCHA key.

  - `secret`: This is the reCAPTCHA secret.

- `header`: This is the authentication header.

- `method`: This is the authentication method used (e.g., "json"). Possible values:
  - password - username and password
  - hook - hook authentication
  - proxy - proxy authentication
  - oath - oath authentication

- `command`: This is the authentication command.

- `signup`: This boolean value indicates whether user signup is enabled.

- `shell`: This is the shell configuration.
  
### Frontend configuration settings

- `name`: This is the name of the frontend.

- `disableExternal`: This boolean value determines whether external access is disabled.

- `disableUsedPercentage`: This boolean value determines whether used percentage is disabled.

- `files`: This is the files configuration.

- `theme`: This is the theme configuration.

- `color`: This is the color configuration.
  
### UserDefaults configuration settings

- `scope`: This is a scope of the permissions, "." or "./" means all directories, "./downloads" would mean only the downloads folder.

- `locale`: This is the locale configuration.

- `viewMode`: This is the view mode configuration.

- `singleClick`: This boolean value determines whether single-click is enabled.

- `sorting`:

  - `by`: This is the sorting method used (e.g., "asc").

  - `asc`: This boolean value determines the sorting order.

- `permissions`:

  - `admin`: This boolean value determines whether admin permissions are granted.

  - `execute`: This boolean value determines whether execute permissions are granted.

  - `create`: This boolean value determines whether create permissions are granted.

  - `rename`: This boolean value determines whether rename permissions are granted.

  - `modify`: This boolean value determines whether modify permissions are granted.

  - `delete`: This boolean value determines whether delete permissions are granted.

  - `share`: This boolean value determines whether share permissions are granted.

  - `download`: This boolean value determines whether download permissions are granted.

- `commands`: This is a list of commands.

- `hideDotfiles`: This boolean value determines whether dotfiles are hidden.

- `dateFormat`: This boolean value determines whether date formatting is enabled.

