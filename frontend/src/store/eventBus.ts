type EventCallback = (data?: unknown) => void;

class EventBus extends EventTarget {
  private listeners = new Map<EventCallback, Map<string, EventListener>>();

  emit(event: string, data?: unknown) {
    this.dispatchEvent(new CustomEvent(event, { detail: data }));
  }

  on(event: string, callback: EventCallback) {
    const wrapper = (e: Event) => callback((e as CustomEvent).detail);

    // Store the wrapper so we can remove it later
    if (!this.listeners.has(callback)) {
      this.listeners.set(callback, new Map());
    }
    this.listeners.get(callback)!.set(event, wrapper);

    this.addEventListener(event, wrapper);
  }

  off(event: string, callback: EventCallback) {
    const eventMap = this.listeners.get(callback);
    if (eventMap?.has(event)) {
      const wrapper = eventMap.get(event)!;
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
