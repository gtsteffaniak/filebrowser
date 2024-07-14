import { state } from "./state.js";

export const getters = {
  isLogged: () => state.user !== null,
  
  isFiles: () => !state.loading && state.route.name === "Files",
  
  isListing: (getters) => getters.isFiles && state.req.isDir,

  selectedCount: () => {
    // Ensure state.selected is an array
    return Array.isArray(state.selected) ? state.selected.length : 0;
  },
  
  progress: () => {
    // Check if state.upload is defined and valid
    if (!state.upload || !Array.isArray(state.upload.progress) || !Array.isArray(state.upload.sizes)) {
      return 0;
    }
    
    // Handle cases where progress or sizes arrays might be empty
    if (state.upload.progress.length === 0 || state.upload.sizes.length === 0) {
      return 0;
    }

    // Calculate totalSize
    let totalSize = state.upload.sizes.reduce((a, b) => a + b, 0);

    // Calculate sum of progress
    let sum = state.upload.progress.reduce((acc, val) => acc + val, 0);

    // Return progress as a percentage
    return Math.ceil((sum / totalSize) * 100);
  },
  
  filesInUploadCount: () => {
    // Ensure state.upload.uploads is an object and state.upload.queue is an array
    const uploadsCount = typeof state.upload.uploads === 'object' ? Object.keys(state.upload.uploads).length : 0;
    const queueCount = Array.isArray(state.upload.queue) ? state.upload.queue.length : 0;

    return uploadsCount + queueCount;
  },
  
  currentPrompt: () => {
    // Ensure state.prompts is an array
    return Array.isArray(state.prompts) && state.prompts.length > 0
      ? state.prompts[state.prompts.length - 1]
      : null;
  },
  
  currentPromptName: (getters) => {
    return getters.currentPrompt?.prompt || null;
  },
  
  filesInUpload: () => {
    // Ensure state.upload.uploads is an object and state.upload.sizes is an array
    if (typeof state.upload.uploads !== 'object' || !Array.isArray(state.upload.sizes)) {
      return [];
    }

    let files = [];

    for (let index in state.upload.uploads) {
      let upload = state.upload.uploads[index];
      let id = upload.id;
      let type = upload.type;
      let name = upload.file.name;
      let size = state.upload.sizes[id] || 0; // Default to 0 if size is undefined
      let isDir = upload.file.isDir;
      let progress = isDir
        ? 100
        : Math.ceil((state.upload.progress[id] || 0 / size) * 100); // Default to 0 if progress is undefined

      files.push({
        id,
        name,
        progress,
        type,
        isDir,
      });
    }

    return files.sort((a, b) => a.progress - b.progress);
  },
};
