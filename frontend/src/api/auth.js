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
      credentials: 'same-origin',
      headers: {
        'X-Password': encodeURIComponent(password),
      }
    })
    const body = await res.json().catch(() => ({}))
    if (!res.ok) {
      const msg = body.message || 'Failed to generate OTP'
      throw new Error(msg)
    }
    return body
  } catch (error) {
    notify.showError(error.message || error || 'Failed to generate OTP')
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
      credentials: 'same-origin',
      headers: {
        'X-Password': encodeURIComponent(password),
        'X-Secret': otp,
      }
    })
    if (res.status != 200) {
      let msg = 'Failed to verify OTP'
      try {
        const errBody = await res.json()
        if (errBody.message) {
          msg = errBody.message
        }
      } catch {
        // keep default message
      }
      throw new Error(msg)
    }
  } catch (error) {
    notify.showError(error.message || error || 'Failed to verify OTP')
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

function arrayBufferToBase64(buffer) {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  bytes.forEach(b => binary += String.fromCharCode(b))
  return btoa(binary).replace(/=/g, '').replace(/\+/g, '-').replace(/\//g, '_')
}

function base64ToArrayBuffer(base64url) {
  // Convert URL-safe base64 to standard base64
  let base64 = base64url.replace(/-/g, '+').replace(/_/g, '/')
  while (base64.length % 4) base64 += '='
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes.buffer
}

function preprocessPublicKeyOptions(options) {
  const opts = { ...options }
  if (typeof opts.challenge === 'string') {
    opts.challenge = base64ToArrayBuffer(opts.challenge)
  }
  if (opts.user && typeof opts.user.id === 'string') {
    opts.user = { ...opts.user, id: base64ToArrayBuffer(opts.user.id) }
  }
  if (opts.allowCredentials && Array.isArray(opts.allowCredentials)) {
    opts.allowCredentials = opts.allowCredentials.map(c => ({
      ...c,
      id: typeof c.id === 'string' ? base64ToArrayBuffer(c.id) : c.id,
    }))
  }
  if (opts.excludeCredentials && Array.isArray(opts.excludeCredentials)) {
    opts.excludeCredentials = opts.excludeCredentials.map(c => ({
      ...c,
      id: typeof c.id === 'string' ? base64ToArrayBuffer(c.id) : c.id,
    }))
  }
  return opts
}

function formatCredentialResponse(cred) {
  return {
    id: cred.id,
    rawId: arrayBufferToBase64(cred.rawId),
    type: cred.type,
    response: {
      clientDataJSON: arrayBufferToBase64(cred.response.clientDataJSON),
      authenticatorData: arrayBufferToBase64(cred.response.authenticatorData),
      signature: arrayBufferToBase64(cred.response.signature),
      userHandle: cred.response.userHandle ? arrayBufferToBase64(cred.response.userHandle) : null,
    },
  }
}

function formatAttestationResponse(cred) {
  return {
    id: cred.id,
    rawId: arrayBufferToBase64(cred.rawId),
    type: cred.type,
    response: {
      clientDataJSON: arrayBufferToBase64(cred.response.clientDataJSON),
      attestationObject: arrayBufferToBase64(cred.response.attestationObject),
      transports: cred.response.getTransports ? cred.response.getTransports() : [],
    },
  }
}

export async function beginPasskeyLogin(username, password) {
  const apiPath = getApiPath('auth/webauthn/begin-login', { username })
  const res = await fetch(apiPath, {
    method: 'POST',
    credentials: 'same-origin',
    headers: {
      'X-Password': encodeURIComponent(password),
    },
  })

  const body = await res.json().catch(() => ({}))
  if (res.status !== 200) {
    const msg = body.message || 'Passkey login failed'
    throw new Error(msg)
  }

  const credential = await navigator.credentials.get({ publicKey: preprocessPublicKeyOptions(body.publicKey) })
  const formatted = formatCredentialResponse(credential)

  const finishPath = getApiPath('auth/webauthn/finish-login', { session_id: body.sessionID })
  const finishRes = await fetch(finishPath, {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(formatted),
  })

  if (finishRes.status !== 200) {
    const errBody = await finishRes.json().catch(() => ({}))
    throw new Error(errBody.message || 'Passkey login failed')
  }
}

export async function beginPasskeyRegistration() {
  const apiPath = getApiPath('auth/webauthn/begin-register')
  const res = await fetch(apiPath, {
    method: 'POST',
    credentials: 'same-origin',
  })

  const body = await res.json().catch(() => ({}))
  if (res.status !== 200) {
    const msg = body.message || 'Failed to begin passkey registration'
    throw new Error(msg)
  }

  const credential = await navigator.credentials.create({ publicKey: preprocessPublicKeyOptions(body.publicKey) })
  const formatted = formatAttestationResponse(credential)

  const name = prompt('Name for this passkey (e.g. "iPhone", "YubiKey"):') || 'Passkey'
  const finishPath = getApiPath('auth/webauthn/finish-register', { session_id: body.sessionID, name })
  const finishRes = await fetch(finishPath, {
    method: 'POST',
    credentials: 'same-origin',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(formatted),
  })

  if (finishRes.status !== 200) {
    const errBody = await finishRes.json().catch(() => ({}))
    throw new Error(errBody.message || 'Failed to register passkey')
  }
}

export async function deletePasskeyCredential(credentialId) {
  const apiPath = getApiPath(`auth/webauthn/${encodeURIComponent(credentialId)}`)
  const res = await fetch(apiPath, {
    method: 'DELETE',
    credentials: 'same-origin',
  })

  if (res.status !== 200) {
    const errBody = await res.json().catch(() => ({}))
    throw new Error(errBody.message || 'Failed to delete passkey')
  }
}
