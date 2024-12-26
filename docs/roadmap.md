# Planned Roadmap

Upcoming 0.3.x releases, ordered by priority:
  - Bring Themes and Branding back.
  - openoffice support https://github.com/filebrowser/filebrowser/pull/2954
  - More filetype previews: eg. raw img, office, photoshop, vector, 3d files.
  - Introduce jobs as replacement to runners.
    - Add Job status to the sidebar
    - index status.
    - Job status from users
    - upload status
  - Opentelemetry metrics
    - user access,
    - file access
    - download count
    - last login
  - more sign in support
    - LDAP
    - 2FA
    - SSO

Upcoming 0.4.x release:
  - Support for multiple filesystem sources https://github.com/filebrowser/filebrowser/issues/2514
  - Onboarding process to add sources and configure them on first run.
  - More indexing flexability
    - option not to index hidden files/folders
    - options folders to include/exclude from indexing
    - implement more indexing runners for more efficienct filesystem watching
  - tags support

Stable release (v1.0.0) - Planned 2025:
  - Once under the hood changes for things like multiple sources, jobs support, etc
  - More robust backend and frontend testing
  - Currently a stable release does not exist primarily because things are still changing, configuration changes are happening frequently and will for the next
  - Rebrand to QuantumX App suite umbrella branding and github repo change.

Unplanned Future releases:
  - Add tools to sidebar
    - duplicate file detector.
    - bulk rename https://github.com/filebrowser/filebrowser/issues/2473
  - support more source types such as minio, s3, and backblaze sources https://github.com/filebrowser/filebrowser/issues/2544
  - Activity Log
  - Comments support
  - Trash Support
  - starred/pinned files
  - event based notifications
