import { fetchURL, fetchJSON } from '@/api/utils'
import { getApiPath, getPublicApiPath } from '@/utils/url.js'
import { notify } from '@/notify'

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

// GET /public/api/users?username= (single user by login name)
export async function get(username) {
  try {
    let apiPath = getPublicApiPath('users', { username })
    return await fetchJSON(apiPath)
  } catch (err) {
    notify.showError(err.message || `Failed to fetch user: ${username}`)
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

// PUT /api/users?username= (update user; target login name is always the query key)
export async function update(user, which = ['all']) {
  const excludeKeys = ['id', 'name']
  which = which.filter(item => !excludeKeys.includes(item))
  if (user.username === 'anonymous') {
    return
  }
  if (!user.username) {
    notify.showError('username is required to update a user')
    throw new Error('username is required')
  }

  let userData = user
  if (which.length !== 1 || which[0] !== 'all') {
    userData = {}
    which.forEach(key => {
      if (key in user) {
        userData[key] = user[key]
      }
    })
  }

  const apiPath = getApiPath('users', { username: user.username })
  await fetchURL(apiPath, {
    method: 'PUT',
    body: JSON.stringify({
      which: which,
      data: userData
    })
  })
}

// DELETE /api/users?username=
export async function remove(username) {
  try {
    const apiPath = getApiPath('users', { username })
    await fetchURL(apiPath, {
      method: 'DELETE'
    })
  } catch (err) {
    notify.showError(err.message || `Failed to delete user: ${username}`)
    throw err
  }
}

