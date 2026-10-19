# Changelog

All notable changes to this project will be documented in this file. For commit guidelines, please refer to [Standard Version](https://github.com/conventional-changelog/standard-version).

## v2.1.3

 **Security**:
 - [Moderate] Moving or renaming a folder updates only shares whose paths are under the moved prefix, preserving per-file share roots instead of retargeting unrelated substring matches (GHSA-26fh-r34c-qxh6) reporters @jayhemnani9910 @dylanzonix
 - [Moderate] WebDAV PUT now requires Create permission when the target path does not yet exist, so Modify-only users cannot create new files (GHSA-8655-2ph3-q7q7) reporter @levanduy181
 - [Low] Failed-login lockout and auth rate limits now use the rightmost `X-Forwarded-For` hop when proxy headers are trusted, so a client cannot spoof the client IP with a leftmost forwarded value (GHSA-wjp8-g49g-xrhx) reporter @LECOQQ
 - [Low] Duplicate finder results are cached per requesting user so filtered file names from one account are not served to another (GHSA-4fhx-8pf6-5gmr) reporter @Nazarami
 - [Moderate] Authenticated `GET /api/resources/view` no longer accepts view tokens minted for a public share when the caller lacks view permission on the requested source (GHSA-wxj5-g3qp-g732) reporter @BlueN0r
 - [Low] SSE source-update delivery no longer iterates the client registry without holding its lock, preventing a concurrent map iteration/write crash when Realtime clients connect or disconnect (GHSA-p9wm-f5jj-75pj) reporter @tao0845
 - [High] Incomplete symlink-containment remediation: an authenticated non-admin user could create a relative symlink via archive extraction, relocate it with WebDAV MOVE, and write through it to arbitrary host paths with the service account's privileges (GHSA-3j46-5w28-f8p2) reporter @Saku0512
 - [Moderate] Source-root containment bypass: create, write, delete, and move operations now resolve symlinks uncached and enforce the caller's scope, closing escapes through pre-existing out-of-source symlinks on upload, PUT, PATCH, delete, and WebDAV endpoints (GHSA-j764-qg3m-rhv7) reporter @chaoming0625
 - [Moderate] Preview reads resolve symlinks within the caller's scope so authenticated preview cannot follow an out-of-scope symlink target (GHSA-g2q5-r6ch-v25f) reporter @tao0845
 - [Moderate] Archive downloads and extraction no longer read or write through symlinks that resolve outside the caller's share or user scope; escaping archive-walk entries are skipped (GHSA-qqf3-2wvq-q4w9) reporter @SmokinCaterpillar
 - [Moderate] Share directory-archive downloads now apply the share owner's access rules to every entry, closing a bypass where anonymous share visitors could download admin-denied content (GHSA-9fcr-gm58-p839) reporter @Tan-JunWei

 **New Features**:
 - chunked uploads for large files like rclone `--owncloud-chunk-size` (#3112)

 **Bug Fixes**:
 - perf(indexing): use one statx syscall per directory entry on Linux for listings instead of readdir lstat plus birth-time statx (#1890)
 - keep requested permissions when creating a user (#3130)
 - add missing file modify logging to activity viewer (#3152)
 - fix(frontend): restore upload prompt close, tab-reload, and in-app navigation guards while uploads are active (including paused uploads)
 - fix(frontend): load the default source when browsing `/files/` with a single configured source ([#3178](https://github.com/gtsteffaniak/filebrowser/issues/3178)) (related: [#3143](https://github.com/gtsteffaniak/filebrowser/issues/3143), [#3144](https://github.com/gtsteffaniak/filebrowser/issues/3144))

## v2.1.2

PLACEHOLDER_TRUNCATED_FOR_UPLOAD