# Folder descriptions

Configure plain-text descriptions for exact folder paths within a source:

```yaml
server:
  sources:
    - name: Shared files
      path: /srv/shared
      config:
        folderDescriptions:
          /out: Upload queue for the cloud uploader
          /projects: Project working files
```

The desktop list and compact views display a Description column between Name and
Size. Only folders already visible to the user receive annotations; descriptions
do not grant access or add hidden folders to the listing. Unconfigured rows are
blank. Text is escaped rather than interpreted as HTML.

Paths are relative to the source root and must start with `/`. Descriptions do not
inherit into subfolders. Renaming the source's display name leaves these mappings
unchanged. If a folder moves, update its configured path and reload the server
configuration. No README or other file is created inside the described folder.
