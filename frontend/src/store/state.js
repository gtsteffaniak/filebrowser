// state.js
export const state = {
    editor: null,
    user: {
        perm: {},
        rules: [], // Default to an empty array
        permissions: {}, // Default to an empty object for permissions
        darkMode: false, // Default to false, assuming this is a boolean
        profile: { // Example of additional user properties
            username: '', // Default to an empty string
            email: '', // Default to an empty string
            avatarUrl: '' // Default to an empty string
        }
    },
    req: {
        sorting: {
            by: 'name', // Initial sorting field
            asc: true,  // Initial sorting order
        },
    },
    oldReq: {},
    clipboard: {
        key: "",
        items: [],
    },
    jwt: "",
    progress: 0,
    loading: false,
    reload: false,
    selected: [],
    multiple: false,
    upload: {
        progress: [], // Array of progress values
        sizes: [],    // Array of sizes
    },
    prompts: [],
    show: null,
    showShell: false,
    showConfirm: null,
};
