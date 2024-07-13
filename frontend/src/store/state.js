// state.js
const state = {
    editor: null,
    user: {
        rules: [],
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
    prompts: [],
    show: null,
    showShell: false,
    showConfirm: null,
};

export default state;
