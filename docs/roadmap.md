# Planned Roadmap

upcoming 0.3.x releases, ordered by priority:
  - more indexing flexability
    - option not to index hidden files/folders
    - options folders to include/exclude from indexing
    - implement more indexing runners for more efficienct filesystem watching
  - more filetype previews: eg. raw img, office, photoshop, vector, 3d files.
  - introduce jobs as replacement to runners.
    - Add Job status to the sidebar
    - index status.
    - Job status from users
    - upload status
  - opentelemetry metrics

Unplanned Future releases:
  - multiple sources https://github.com/filebrowser/filebrowser/issues/2514
  - Add tools to sidebar
    - duplicate file detector.
    - bulk rename https://github.com/filebrowser/filebrowser/issues/2473
    - metrics tracker - user access, file access, download count, last login, etc
  - support minio, s3, and backblaze sources https://github.com/filebrowser/filebrowser/issues/2544
