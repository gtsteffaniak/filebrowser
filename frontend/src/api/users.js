import { fetchURL, fetchJSON } from '@/api/utils'
import { getApiPath, getPublicApiPath } from '@/utils/url.js'
import { notify } from '@/notify'

// GET /api/users (list all)
export async function getAllUsers() {
  try {
    const apiPath = getApiPath('api/users')
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
    const apiPath = getApiPath('api/users')
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
export async function update(user, which = ['all']) {
  const excludeKeys = ['id', 'name']
  which = which.filter(item => !excludeKeys.includes(item))
  if (user.username === 'anonymous') {
    return
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

  const apiPath = getApiPath('api/users', { id: user.id })
  await fetchURL(apiPath, {
    method: 'PUT',
    body: JSON.stringify({
      which: which,
      data: userData
    })
  })
}

// DELETE /api/users (remove user)
export async function remove(id) {
  try {
    const apiPath = getApiPath('api/users', { id: id })
    await fetchURL(apiPath, {
      method: 'DELETE'
    })
  } catch (err) {
    notify.showError(err.message || `Failed to delete user with ID: ${id}`)
    throw err
  }
}

