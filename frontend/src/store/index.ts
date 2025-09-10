// store/index.ts
import { state } from "./state.js";
import { getters } from "./getters.js";
import { mutations } from "./mutations.js";
import type { StoreState } from "./types";

// Type assertion to tell TypeScript about the actual structure
const typedState = state as StoreState;

export {
  typedState as state,
  getters,
  mutations
};