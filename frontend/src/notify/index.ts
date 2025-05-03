import * as messageFunctions from "./message.js";
import * as loadingSpinnerFunctions from "./loadingSpinner.js";
import * as events from "./events.js";

const notify = {
    ...messageFunctions,
    ...loadingSpinnerFunctions
};

export { notify, events };