# Planned Roadmap

Next version (v0.2.7) :

- Replace http routes for gorilla/mux
- Replace vue-router with simple vanilla js
- Theme configuration from settings
- Replace afero requests with std library
- Add Job status to the sidebar
  - index status.
  - new jobs as they come

Future releases:

 - Allow multiple volumes to show up in the same filebrowser container. https://github.com/filebrowser/filebrowser/issues/2514
 - enable/disable indexing for certain mounts
 - Add tools to sidebar
   - duplicate file detector.
   - bulk rename https://github.com/filebrowser/filebrowser/issues/2473
   - job manager - folder sync, copy, lifecycle operations
   - metrics tracker - user access, file access, download count, last login, etc
 - support minio s3 and backblaze sources https://github.com/filebrowser/filebrowser/issues/2544
