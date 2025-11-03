// eventBus.ts
class EventBus extends EventTarget {
  constructor() {
    super();
    this.listeners = new Map();
  }

  emit(event, data) {
    this.dispatchEvent(new CustomEvent(event, { detail: data }));
  }

  on(event, callback) {
    const wrapper = (e) => callback(e.detail);
    
    // Store the wrapper so we can remove it later
    if (!this.listeners.has(callback)) {
      this.listeners.set(callback, new Map());
    }
    this.listeners.get(callback).set(event, wrapper);
    
    this.addEventListener(event, wrapper);
  }

  off(event, callback) {
    if (this.listeners.has(callback)) {
      const eventMap = this.listeners.get(callback);
      if (eventMap.has(event)) {
        const wrapper = eventMap.get(event);
        this.removeEventListener(event, wrapper);
        eventMap.delete(event);
        
        // Clean up if no more events for this callback
        if (eventMap.size === 0) {
          this.listeners.delete(callback);
        }
      }
    }
  }
}

export const eventBus = new EventBus();

export function emitStateChanged() {
  eventBus.emit('stateChanged');
}
