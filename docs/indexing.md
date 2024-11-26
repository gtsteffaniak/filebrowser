# About Indexing on FileBrowser Quantum

The most significant feature is the index, this document intends to demystify how it works so you can understand how to ensure your index closely matches the current state of your filesystem.

## How does the index work?

The approach used by this repo includes filesystem watchers that periodically scan the directory tree for changes. By default, this uses a smart scan strategy, but you can also configure a set interval in your config file.

The `scan interval` is the break time between scans and does not include the time a scan takes. A typical scan can vary dramatically, but here are some expectations for SSD-based disks:

| # folders  | # files  | time to index  | memory usage (RAM) |
|---|---|---|---|
|  10,000 | 10,000 | ~ 0-5 seconds | 15 MB |
| 2,000  | 250,000 | ~ 0-5 seconds | 300 MB |
| 50,000  | 50,000 | ~ 5-30 seconds | 150 MB |
| 250,000  | 10,000 | ~ 2-5 minutes | 300 MB |
| 500,000  | 500,000 | ~ 5+ minutes | 500+ MB |

### Smart Scanning

1. There is a floating `smart scan interval` that ranges from **1 minute - 4 hours** depending on the complexity of your filesystem
2. The smart interval changes based on how often it discovers changed files:
  - ```
        // Schedule in minutes
        var scanSchedule = []time.Duration{
            5 * time.Minute, // 5 minute quick scan & 25 minutes for a full scan
            10 * time.Minute,
            20 * time.Minute, // [3] element is 20 minutes, reset anchor for full scan
            40 * time.Minute,
            1 * time.Hour,
            2 * time.Hour,
            3 * time.Hour,
            4 * time.Hour, // 4 hours for quick scan & 20 hours for a full scan
        }
    ```
3. The `smart scan interval` performs a `quick scan` 4 times in a row, followed by a 5th `full scan` which completely rebuilds the index.
   - A `quick scan` is limited to detecting directory changes, but is 10x faster than a full scan. Here is what a quick scan can see:
      1. New files or folders created.
      2. Files or folders deleted.
      3. Renaming of files or folders.
   - A quick scan **cannot** detect when a file has been updated, for example when you save a file and the size increases.
   - A `full scan` is a complete re-indexing. This is always more disk and computationally intense but will capture individual file changes.
4. The `smart scan interval` changes based on several things. A `simple` complexity enables scans every 1 minute if changes happen frequently and a maximum full scan interval of every 100 minutes. `high` complexity indicates a minimum scanning interval of 10 minutes.
   - **under 10,000 folders** or **Under 3 seconds** to index is awlays considered `simple` complexity.
   - **more than 500,000 folders** or **Over 2 minutes** to index is always considered `high` complexity.

### Manual Scanning Interval

If you don't like the behavior of smart scanning, you can configure set intervals instead by setting `indexingInterval` to a number greater than 0. This will make FileBrowser Quantum always scan at the given interval in minutes.

The scan behavior is still 4 quick scans at a given interval, followed by a 5th full scan.

### System requirements

You can expect FileBrowser Quantum to use 100 MB of RAM for a typical installation. If you have many files and folders then the requirement could climb to multiple Gigabytes. Please monitor your system on the first run to know your specific requirements.

### Why does FileBrowser Quantum index the way it does?

The smart indexing method uses filesystem scanners because it allows a low-footprint design that can cater to individual filesystem complexity. There are a few options for monitoring a filesystem for changes:

1. **Option 1**: Recursive Traversal with ReadDir
  - This is quite computationally intensive but creates an accurate record of the filesystem
  - Requires periodic scanning to remain updated
  - Low overhead and straightforward implementation.
2. **Option 2**: Use File System Monitoring (Real-Time or Periodic Check) such as `fsnotify`
  - This allows for event-based reactions to filesystem changes.
  - Requires extra overhead.
  - Relies on OS level features and behavior differs between OS's
  - Requires OS-level configuration to ulimits in order to properly watch a large filesystem.
3. **Option 3**: Directory Metadata Heuristics.
  - Using ModTime to determine when directory structures change.
  - Has minimal insight into actual file changes.
  - Much faster to scan for changes than Recursive transversal.

Ultimately, FileBrowser Quantum uses a combination of 1 and 3 to perform index updates. Using something like fsnotify is a non-starter for large filesystems, where it would require manual host OS tuning to work at all. Besides, I can essentially offer the same behavior by creating "watchers" for top-level folders (a feature to come in the future). However, right now there is a single root-level watcher that works over the entire index.

The main disadvantage of the approach is the delay caused by the scanning interval.

### How to manually refresh the index?

There is currently no way to manually trigger a new full indexing. This will come in a future release when the "jobs" functionality is added back.

However, if you want to force-refresh a certain directory, this happens every time you **view it** in the UI or via the resources API.

This also means the resources API is always up to date with the current status of the filesystem. When you "look" at a specific folder, you are causing the index to be refreshed at that location.

### What information does the index have?

You can see what the index looks like by using the resources API via the GET method, which returns individual directory information -- all of this information is stored in the index.

Here is an example:

```
{
    "name": "filebrowser",
    "size": 274467914,
    "modified": "2024-11-23T19:18:57.68013727-06:00",
    "type": "directory",
    "files": [
        {
            "name": ".dockerignore",
            "size": 73,
            "modified": "2024-11-20T18:14:44.91135413-06:00",
            "type": "blob"
        },
        {
            "name": ".DS_Store",
            "size": 6148,
            "modified": "2024-11-22T14:45:15.901211088-06:00",
            "type": "blob"
        },
        {
            "name": ".gitignore",
            "size": 455,
            "modified": "2024-11-23T19:18:57.616132373-06:00",
            "type": "blob"
        },
        {
            "name": "CHANGELOG.md",
            "size": 9325,
            "modified": "2024-11-23T19:18:57.616646332-06:00",
            "type": "text"
        },
        {
            "name": "Dockerfile",
            "size": 769,
            "modified": "2024-11-23T19:18:57.616941333-06:00",
            "type": "blob"
        },
        {
            "name": "Dockerfile.playwright",
            "size": 542,
            "modified": "2024-11-23T19:18:57.617151875-06:00",
            "type": "blob"
        },
        {
            "name": "makefile",
            "size": 1311,
            "modified": "2024-11-23T19:18:57.68017352-06:00",
            "type": "blob"
        },
        {
            "name": "README.md",
            "size": 10625,
            "modified": "2024-11-23T19:18:57.617464334-06:00",
            "type": "text"
        }
    ],
    "folders": [
        {
            "name": ".git",
            "size": 60075460,
            "modified": "2024-11-24T14:44:42.52180215-06:00",
            "type": "directory"
        },
        {
            "name": ".github",
            "size": 11584,
            "modified": "2024-11-20T18:14:44.911805335-06:00",
            "type": "directory"
        },
        {
            "name": "backend",
            "size": 29247172,
            "modified": "2024-11-23T19:18:57.667109624-06:00",
            "type": "directory"
        },
        {
            "name": "docs",
            "size": 14272,
            "modified": "2024-11-24T13:46:12.082024018-06:00",
            "type": "directory"
        },
        {
            "name": "frontend",
            "size": 185090178,
            "modified": "2024-11-24T14:44:39.880678934-06:00",
            "type": "directory"
        }
    ],
    "path": "/filebrowser"
}
```

### Can I disable the index and still use FileBrowser Quantum?

You can disable the index by setting `indexing: false` in your config file. You will still be able to browse your files, but the search will not work and you may run into issues as it's not intended to be used without indexing.

I'm not sure why you would run it like this, if you have a good reason please open an issue request on how you would like it to work -- and why you would run it without the index.
