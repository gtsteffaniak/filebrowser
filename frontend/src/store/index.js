import Vue from "vue";
import Vuex from "vuex";
import mutations from "./mutations";
import getters from "./getters";
import upload from "./modules/upload";

Vue.use(Vuex);

const state = {
  editor: null,
  user: null,
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

export default new Vuex.Store({
  strict: true,
  state,
  getters,
  mutations,
  modules: { upload },
});
