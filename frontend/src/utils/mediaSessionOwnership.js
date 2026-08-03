/** True when this viewer instance currently owns the global Media Session slot. */
export function ownsMediaSession(ownerId, globalOwner) {
  return ownerId > 0 && ownerId === globalOwner;
}
