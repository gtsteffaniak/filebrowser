# Planned Roadmap

next 0.2.x release:

- Theme configuration from settings
- Better media and file viewer support


initial 0.3.0 release :

- drop in replace backend db with pocketbas
- Add Job status to the sidebar
  - index status.
  - Job status from users

Future releases:
  - Replace http routes for gorilla/mux with pocketbase
  - Allow multiple volumes to show up in the same filebrowser container. https://github.com/filebrowser/filebrowser/issues/2514
  - enable/disable indexing for certain mounts
  - Add tools to sidebar
    - duplicate file detector.
    - bulk rename https://github.com/filebrowser/filebrowser/issues/2473
    - job manager - folder sync, copy, lifecycle operations
    - metrics tracker - user access, file access, download count, last login, etc
  - support minio s3 and backblaze sources https://github.com/filebrowser/filebrowser/issues/2544
