import Vue from 'vue';

export const EventBus = new Vue();

export function emitStateChanged() {
  EventBus.$emit('stateChanged');
}
