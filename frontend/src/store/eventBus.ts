type EventCallback = (data?: unknown) => void;

class EventBus extends EventTarget {
  private listeners = new Map<EventCallback, Map<string, EventListener>>();

  emit(event: string, data?: unknown) {
    this.dispatchEvent(new CustomEvent(event, { detail: data }));
  }

  on<T = unknown>(event: string, typedCallback: (data: T) => void) {
    const callback = typedCallback as EventCallback;
    if (this.listeners.get(callback)?.has(event)) {
      return;
    }

    const wrapper = (e: Event) => callback((e as CustomEvent).detail);

    // Store the wrapper so we can remove it later
    let eventMap = this.listeners.get(callback);
    if (!eventMap) {
      eventMap = new Map();
      this.listeners.set(callback, eventMap);
    }
    eventMap.set(event, wrapper);

    this.addEventListener(event, wrapper);
  }

  off<T = unknown>(event: string, typedCallback: (data: T) => void) {
    const callback = typedCallback as EventCallback;
    const eventMap = this.listeners.get(callback);
    const wrapper = eventMap?.get(event);
    if (eventMap && wrapper) {
      this.removeEventListener(event, wrapper);
      eventMap.delete(event);

      // Clean up if no more events for this callback
      if (eventMap.size === 0) {
        this.listeners.delete(callback);
      }
    }
  }
}

export const eventBus = new EventBus();

export function emitStateChanged() {
  eventBus.emit('stateChanged');
}
