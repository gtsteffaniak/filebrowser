/**
 * Shared draft state for admin user-edit hub and breakout prompts.
 */
let activeSession = null;

function clone(value) {
  return JSON.parse(JSON.stringify(value ?? null));
}

export function createUserEditSession(payload = {}) {
  activeSession = {
    targetUsername: payload.targetUsername ?? null,
    user: payload.user ?? null,
    profileUser: payload.profileUser ?? null,
    selectedSources: payload.selectedSources ?? [],
    originalSnapshot: payload.originalSnapshot ?? null,
    listeners: new Set(),
  };
  return activeSession;
}

export function getUserEditSession() {
  return activeSession;
}

export function destroyUserEditSession() {
  activeSession = null;
}

export function updateUserEditSession(patch) {
  if (!activeSession) {
    return;
  }
  Object.assign(activeSession, patch);
  for (const listener of activeSession.listeners) {
    listener(activeSession);
  }
}

export function subscribeUserEditSession(listener) {
  if (!activeSession) {
    return () => {};
  }
  activeSession.listeners.add(listener);
  return () => {
    activeSession?.listeners.delete(listener);
  };
}

export function cloneUserEditSessionValue(value) {
  return clone(value);
}
