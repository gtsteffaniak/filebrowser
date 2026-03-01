import { fetchURL, fetchJSON } from '@/api/utils'
import { getApiPath } from '@/utils/url.js'
import { notify } from '@/notify'

// POST /api/auth/login
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
  let apiPath = getApiPath('auth/login', params);
  const res = await fetch(apiPath, {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      'X-Password': encodeURIComponent(password),
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

  if (res.status != 200) {
    const msg = body.message || 'Forbidden';
    throw new Error(msg);
  }
}

// POST /api/auth/logout
export async function logout() {
  try {
    const apiPath = getApiPath('auth/logout')
    await fetchURL(apiPath, { method: 'POST' })
  } catch (err) {
    notify.showError(err.message || 'Failed to logout')
    throw err
  }
}

// POST /api/auth/signup
export async function signup(username, password, otp) {
  const params = { username, password, otp }
  let apiPath = getApiPath('auth/signup', params)
  const res = await fetch(apiPath, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    }
  })

  if (res.status !== 201) {
    let errorMessage = res.status
    try {
      const errorData = await res.json()
      if (errorData.message) {
        errorMessage = errorData.message
      }
    } catch (parseError) {
      // If parsing fails, keep the status code as error message
    }
    throw new Error(errorMessage)
  }
}

// POST /api/auth/otp/generate
export async function generateOTP(username, password) {
  const params = { username }
  try {
    let apiPath = getApiPath('auth/otp/generate', params)
    const res = await fetch(apiPath, {
      method: 'POST',
      headers: {
        'X-Password': encodeURIComponent(password),
      }
    })
    return await res.json()
  } catch (error) {
    notify.showError(error || 'Failed to generate OTP')
    throw error
  }
}

// POST /api/auth/otp/verify
export async function verifyOTP(username, password, otp) {
  const params = { username }
  try {
    let apiPath = getApiPath('auth/otp/verify', params)
    const res = await fetch(apiPath, {
      method: 'POST',
      headers: {
        'X-Password': encodeURIComponent(password),
        'X-Secret': otp,
      }
    })
    if (res.status != 200) {
      throw new Error('Failed to verify OTP')
    }
  } catch (error) {
    notify.showError(error || 'Failed to verify OTP')
    throw error
  }
}

// POST /api/auth/renew
export async function renew() {
  try {
    const apiPath = getApiPath('auth/renew')
    await fetchURL(apiPath, { method: 'POST' })
  } catch (err) {
    notify.showError(err.message || 'Failed to renew token')
    throw err
  }
}

// GET /api/auth/token/list
export async function getApiKeys() {
  try {
    const apiPath = getApiPath('auth/token/list')
    return await fetchJSON(apiPath)
  } catch (err) {
    // ignore 404 errors
    if (err.status !== 404) {
      notify.showError(err.message || 'Failed to get API tokens')
      throw err
    }
    throw err
  }
}

// PUT /api/auth/token
export async function createApiKey(params) {
  try {
    const apiPath = getApiPath('auth/token', params)
    await fetchURL(apiPath, {
      method: 'POST'
    })
  } catch (err) {
    notify.showError(err.message || 'Failed to create API token')
    throw err
  }
}

// DELETE /api/auth/token
export function deleteApiKey(params) {
  try {
    const apiPath = getApiPath('auth/token', params)
    fetchURL(apiPath, {
      method: 'DELETE'
    })
  } catch (err) {
    notify.showError(err.message || 'Failed to delete API token')
    throw err
  }
}
