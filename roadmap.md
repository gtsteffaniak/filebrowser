# Planned Roadmap

next 0.2.x release:

- Theme configuration from settings
- File syncronization improvements
- right-click context menu

initial 0.3.0 release :

- database changes
- introduce jobs as replacement to runners.
- Add Job status to the sidebar
  - index status.
  - Job status from users
  - upload status

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
