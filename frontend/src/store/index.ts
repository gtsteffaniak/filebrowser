// store/index.ts
import { getters } from "./getters.js";
import { mutations } from "./mutations.js";
import { state } from "./state.js";

import type { StoreState } from "./types";

// Type assertion to tell TypeScript about the actual structure
const typedState = state as StoreState;

export {
  getters,
  mutations,
  typedState as state,
};
