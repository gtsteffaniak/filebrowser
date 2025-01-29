import * as messageFunctions from "./message.js";
import * as loadingSpinnerFunctions from "./loadingSpinner.js";

const notify = {
    ...messageFunctions,
    ...loadingSpinnerFunctions
};

export { notify };