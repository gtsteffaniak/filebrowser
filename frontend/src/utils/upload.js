import { state } from "@/store";
import url from "@/utils/url.js";
import { filesApi } from "@/api";

export function checkConflict(files, items) {
  console.log("testing",files)

  if (typeof items === "undefined" || items === null) {
    items = [];
  }

  let folder_upload = files[0].fullPath !== undefined;

  let conflict = false;
  for (let i = 0; i < files.length; i++) {
    let file = files[i];
    let name = file.name;

    if (folder_upload) {
      let dirs = file.fullPath.split("/");
      if (dirs.length > 1) {
        name = dirs[0];
      }
    }

    let res = items.findIndex(function hasConflict(element) {
      return element.name === this;
    }, name);

    if (res >= 0) {
      conflict = true;
      break;
    }
  }

  return conflict;
}

export function scanFiles(dt) {
  return new Promise((resolve) => {
    let reading = 0;
    const contents = [];

    if (dt.items !== undefined) {
      for (let item of dt.items) {
        if (
          item.kind === "file" &&
          typeof item.webkitGetAsEntry === "function"
        ) {
          const entry = item.webkitGetAsEntry();
          readEntry(entry);
        }
      }
    } else {
      resolve(dt.files);
    }

    function readEntry(entry, directory = "") {
      if (entry.isFile) {
        reading++;
        entry.file((file) => {
          reading--;

          file.fullPath = `${directory}${file.name}`;
          contents.push(file);

          if (reading === 0) {
            resolve(contents);
          }
        });
      } else if (entry.isDirectory) {
        const dir = {
          isDir: true,
          size: 0,
          fullPath: `${directory}${entry.name}`,
          name: entry.name,
        };

        contents.push(dir);

        readReaderContent(entry.createReader(), `${directory}${entry.name}`);
      }
    }

    function readReaderContent(reader, directory) {
      reading++;

      reader.readEntries(function (entries) {
        reading--;
        if (entries.length > 0) {
          for (const entry of entries) {
            readEntry(entry, `${directory}/`);
          }

          readReaderContent(reader, `${directory}/`);
        }

        if (reading === 0) {
          resolve(contents);
        }
      });
    }
  });
}

export async function handleFiles(files, base, overwrite = false) {
  for (const file of files) {
    const id = state.upload.id;
    let path = base;

    if (file.fullPath !== undefined) {
      path += url.encodePath(file.fullPath);
    } else {
      path += url.encodeRFC5987ValueChars(file.name);
    }

    if (file.type == "directory") {
      path += "/";
    }

    const item = {
      id,
      path,
      file: file.file, // Ensure `file.file` is the Blob or File
      overwrite,
    };

    await filesApi.post(item.path, item.file, item.overwrite, (event) => {
      console.log(`Upload progress: ${Math.round((event.loaded / event.total) * 100)}%`);
    })
    .then(response => {
      console.log("Upload successful:", response);
    })
    .catch(error => {
      console.error("Upload error:", error);
    });
  }
}