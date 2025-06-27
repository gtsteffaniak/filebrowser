import { state } from '@/store'
import { filesApi } from '@/api'
import { notify } from '@/notify'

export function checkConflict (files, items) {
  if (typeof items === 'undefined' || items === null) {
    items = []
  }

  let folder_upload = files[0].path !== undefined

  let conflict = false
  for (let i = 0; i < files.length; i++) {
    let file = files[i]
    let name = file.name

    if (folder_upload) {
      let dirs = file.path.split('/')
      if (dirs.length > 1) {
        name = dirs[0]
      }
    }

    let res = items.findIndex(function hasConflict (element) {
      return element.name === this
    }, name)

    if (res >= 0) {
      conflict = true
      break
    }
  }

  return conflict
}

export function scanFiles (dt) {
  return new Promise(resolve => {
    let reading = 0
    const contents = []

    if (dt.items !== undefined) {
      for (let item of dt.items) {
        if (
          item.kind === 'file' &&
          typeof item.webkitGetAsEntry === 'function'
        ) {
          const entry = item.webkitGetAsEntry()
          readEntry(entry)
        }
      }
    } else {
      resolve(dt.files)
    }

    function readEntry (entry, directory = '') {
      if (entry.isFile) {
        reading++
        entry.file(file => {
          reading--

          file.fullPath = directory + file.name
          contents.push(file)

          if (reading === 0) {
            resolve(contents)
          }
        })
      } else if (entry.isDirectory) {
        const dir = {
          isDir: true,
          size: 0,
          fullPath: directory + entry.name + '/',
          name: entry.name
        }
        contents.push(dir)

        readReaderContent(entry.createReader(), directory + entry.name + '/')
      }
    }

    function readReaderContent (reader, directory) {
      reading++

      reader.readEntries(function (entries) {
        reading--
        if (entries.length > 0) {
          for (const entry of entries) {
            readEntry(entry, `${directory}/`)
          }

          readReaderContent(reader, `${directory}/`)
        }

        if (reading === 0) {
          resolve(contents)
        }
      })
    }
  })
}

export async function handleFiles (files, base, overwrite = false) {
  let blockUpdates = false
  let c = 0
  const count = files.length

  for (const file of files) {
    c += 1
    const id = state.upload.id

    // Inside handleFiles
    let basePath = base
    let relativePath = file.fullPath || file.webkitRelativePath || file.name

    if (basePath.endsWith('/')) basePath = basePath.slice(0, -1)
    if (relativePath.startsWith('/')) relativePath = relativePath.slice(1)

    const path = basePath + '/' + relativePath

    // If it's a directory entry (from scanFiles)
    const isDirectory = file.type === 'directory' || file.isDir

    const item = {
      id,
      path: isDirectory ? path + '/' : path,
      file: isDirectory ? new Blob([]) : file.file || file, // support both wrapped and raw files
      overwrite
    }

    let last = 0
    notify.showPopup(
      'success',
      `(${c} of ${count}) Uploading ${relativePath}`,
      false
    )

    await filesApi
      .post(item.path, item.file, item.overwrite, percentComplete => {
        if (blockUpdates) return
        blockUpdates = true
        notify.startLoading(last, percentComplete)
        last = percentComplete
        setTimeout(() => {
          blockUpdates = false
        }, 250)
      })
      .then(response => {
        const spinner = document.querySelector('.notification-spinner')
        if (spinner) spinner.classList.add('hidden')
        console.log('Upload successful!', response)
        notify.showSuccess('Upload successful!')
      })
      .catch(error => {
        const spinner = document.querySelector('.notification-spinner')
        if (spinner) spinner.classList.add('hidden')
        notify.showError('Error uploading file: ' + error)
        throw error
      })
  }
}
