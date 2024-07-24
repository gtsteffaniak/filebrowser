// eventBus.ts
class EventBus extends EventTarget {
  emit(event, data) {
    this.dispatchEvent(new CustomEvent(event, { detail: data }));
  }

  on(event, callback) {
    this.addEventListener(event, (e) => callback(e.detail));
  }
}

export const eventBus = new EventBus();

export function emitStateChanged() {
  eventBus.emit('stateChanged');
}
