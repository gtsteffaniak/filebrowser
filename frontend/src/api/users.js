import { fetchURL, fetchJSON } from '@/api/utils'
import { getApiPath, getPublicApiPath } from '@/utils/url.js'
import { notify } from '@/notify' // Import notify for error handling
import { setNewToken } from '@/utils/auth.js' // Import setNewToken for token management
export async function getAllUsers() {
  try {
    const apiPath = getApiPath('api/users')
    return await fetchJSON(apiPath)
  } catch (err) {
    notify.showError(err.message || 'Failed to fetch users')
    throw err // Re-throw to handle further if needed
  }
}

export async function generateOTP(username, password) {
  const params = { username }
  try {
    let apiPath = getApiPath('api/auth/otp/generate', params)
    const res = await fetch(apiPath, {
      method: 'POST',
      headers: {
        'X-Password': password,
      }
    })
    return await res.json()
  } catch (error) {
    notify.showError(error || 'Failed to generate OTP')
    throw error
  }
}

export async function verifyOtp(username, password, otp) {
  const params = { username }
  try {
    let apiPath = getApiPath('api/auth/otp/verify', params)
    const res = await fetch(apiPath, {
      method: 'POST',
      headers: {
        'X-Password': password,
        'X-Secret': otp,
      }
    })
    if (res.status != 200) {
      throw new Error('Failed to verify OTP')
    }
  } catch (error) {
    notify.showError(error || 'Failed to generate OTP')
    throw error
  }
}

export async function login(username, password, recaptcha, otp) {
  if (!otp) {
    otp = ''
  }
  if (!recaptcha) {
    recaptcha = ''
  }
  if (!password) {
    password = ''
  }

  const params = { username, recaptcha };
  let apiPath = getApiPath('api/auth/login', params);
  const res = await fetch(apiPath, {
    method: 'POST',
    credentials: 'same-origin', // Ensure cookies can be set during login
    headers: {
      'X-Password': password,
      'X-Secret': otp,
    }
  });

  const bodyText = await res.text();
  let body;

  try {
    body = JSON.parse(bodyText);
  } catch {
    body = { message: bodyText };
  }

  if (res.status === 200) {
    await setNewToken(bodyText);
  } else if (res.status === 403) {
    const msg = body.message || 'Forbidden';
    throw new Error(msg);
  } else {
    throw new Error(body.message || bodyText || 'Failed to login');
  }
}
export async function get(id) {
  try {
    let apiPath = getPublicApiPath('users', { id: id })
    return await fetchJSON(apiPath)
  } catch (err) {
    notify.showError(err.message || `Failed to fetch user with ID: ${id}`)
    throw err
  }
}

export async function getApiKeys () {
  try {
    const apiPath = getApiPath('api/auth/tokens')
    return await fetchJSON(apiPath)
  } catch (err) {
    notify.showError(err.message || `Failed to get api keys`)
    throw err
  }
}

export async function signupLogin (username, password, otp) {
  const params = { username, password, otp }
  let apiPath = getApiPath('api/auth/signup', params)
  const res = await fetch(apiPath, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    }
  })

  if (res.status !== 201) {
    throw new Error(res.status)
  }
}

export async function createApiKey (params) {
  try {
    const apiPath = getApiPath('api/auth/token', params)
    await fetchURL(apiPath, {
      method: 'PUT'
    })
  } catch (err) {
    notify.showError(err.message || `Failed to create API key`)
    throw err
  }
}

export function deleteApiKey (params) {
  try {
    const apiPath = getApiPath('api/auth/token', params)
    fetchURL(apiPath, {
      method: 'DELETE'
    })
  } catch (err) {
    notify.showError(err.message || `Failed to delete API key`)
    throw err
  }
}

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

export async function update(user, which = ['all']) {
  // List of keys to exclude from the "which" array
  const excludeKeys = ['id', 'name']
  // Filter out the keys from "which"
  which = which.filter(item => !excludeKeys.includes(item))
  if (user.username === 'anonymous') {
    return
  }
  const apiPath = getApiPath('api/users', { id: user.id })
  await fetchURL(apiPath, {
    method: 'PUT',
    body: JSON.stringify({
      which: which,
      data: user
    })
  })
}

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
