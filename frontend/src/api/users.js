import { fetchURL, fetchJSON } from '@/api/utils'
import { getApiPath, getPublicApiPath } from '@/utils/url.js'
import { notify } from '@/notify'
import { state } from '@/store/state.js'
import { mutations } from '@/store/mutations.js'
import i18n from '@/i18n'

// GET /api/users (list all)
export async function getAllUsers() {
  try {
    const apiPath = getApiPath('users')
    return await fetchJSON(apiPath)
  } catch (err) {
    notify.showError(err.message || 'Failed to fetch users')
    throw err
  }
}

// GET /api/users or /public/api/users (get single user)
export async function get(id) {
  try {
    let apiPath = getPublicApiPath('users', { id: id })
    return await fetchJSON(apiPath)
  } catch (err) {
    notify.showError(err.message || `Failed to fetch user with ID: ${id}`)
    throw err
  }
}

// POST /api/users (create user)
export async function create(user) {
  try {
    const apiPath = getApiPath('users')
    const res = await fetchURL(apiPath, {
      method: 'POST',
      body: JSON.stringify({
        which: [],
        data: user
      })
    })

    if (res.status === 201) {
      return res.headers.get('Location')
    } else {
      throw new Error('Failed to create user')
    }
  } catch (err) {
    notify.showError(err.message || 'Error creating user')
    throw err
  }
}

// PUT /api/users (update user)
// Password-login: tries without X-Password first; on 401 requiring X-Password, opens the prompt and retries.
// options.skipActorPasswordConfirm / pre-set X-Password skip that flow.
// options.actorPasswordPromptI18nKey — optional vue-i18n key (default: confirmPasswordToSaveUser).
export async function update(user, which = ['all'], options = {}) {
  const excludeKeys = ['id', 'name']
  which = which.filter(item => !excludeKeys.includes(item))
  if (user.username === 'anonymous') {
    return
  }

  const mergedHeaders = { ...(options.headers || {}) }

  let userData = user
  if (which.length !== 1 || which[0] !== 'all') {
    userData = {}
    which.forEach(key => {
      if (key in user) {
        userData[key] = user[key]
      }
    })
  }

  const apiPath = getApiPath('users', { id: user.id })
  const body = JSON.stringify({
    which: which,
    data: userData
  })

  const needsActorPasswordRetry = (err) =>
    state.user?.loginMethod === 'password' &&
    options.skipActorPasswordConfirm !== true &&
    mergedHeaders['X-Password'] === undefined &&
    err &&
    err.status === 401 &&
    typeof err.message === 'string' &&
    err.message.includes('X-Password')

  try {
    await fetchURL(apiPath, {
      method: 'PUT',
      body,
      headers: mergedHeaders,
    })
  } catch (e) {
    if (!needsActorPasswordRetry(e)) {
      throw e
    }
    const promptKey =
      options.actorPasswordPromptI18nKey || 'prompts.confirmPasswordToSaveUser'
    return new Promise((resolve, reject) => {
      mutations.showPrompt({
        name: 'password',
        props: {
          infoText: i18n.global.t(promptKey),
          submitLabel: i18n.global.t('general.confirm'),
          submitCallback: async (actorPassword) => {
            try {
              await update(user, which, {
                ...options,
                headers: {
                  ...mergedHeaders,
                  'X-Password': encodeURIComponent(actorPassword),
                },
                skipActorPasswordConfirm: true,
              })
              resolve(undefined)
            } catch (err) {
              reject(err)
            }
          },
        },
      })
    })
  }
}

// DELETE /api/users (remove user)
export async function remove(id) {
  try {
    const apiPath = getApiPath('users', { id: id })
    await fetchURL(apiPath, {
      method: 'DELETE'
    })
  } catch (err) {
    notify.showError(err.message || `Failed to delete user with ID: ${id}`)
    throw err
  }
}

