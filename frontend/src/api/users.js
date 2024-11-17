import { fetchURL, fetchJSON, getApiPath } from "@/api/utils";
import { notify } from "@/notify";  // Import notify for error handling

export async function getAllUsers() {
  try {
    return await fetchJSON(`api/users`, {});
  } catch (err) {
    notify.showError(err.message || "Failed to fetch users");
    throw err; // Re-throw to handle further if needed
  }
}


export async function get(id) {
  try {
    const apiPath = getApiPath("api/users", { id: id });
    return await fetchJSON(apiPath);
  } catch (err) {
    notify.showError(err.message || `Failed to fetch user with ID: ${id}`);
    throw err;
  }
}

export async function getApiKeys() {
  try {
    const apiPath = getApiPath("api/auth/tokens");
    return await fetchJSON(apiPath);
  } catch (err) {
    notify.showError(err.message || `Failed to get api keys`);
    throw err;
  }
}


export async function createApiKey(params) {
  try {
    const apiPath = getApiPath("api/auth/token", params);
    await fetchURL(apiPath, {
      method: "PUT",
    });
  } catch (err) {
    notify.showError(err.message || `Failed to create API key`);
    throw err;
  }
}

export function deleteApiKey(params) {
  try {
    const apiPath = getApiPath("api/auth/token", params);
    fetchURL(apiPath, {
      method: "DELETE",
    });
  } catch (err) {
    notify.showError(err.message || `Failed to delete API key`);
    throw err;
  }
}

export async function create(user) {
  try {
    const res = await fetchURL(`api/users`, {
      method: "POST",
      body: JSON.stringify({
        what: "user",
        which: [],
        data: user,
      }),
    });

    if (res.status === 201) {
      return res.headers.get("Location");
    } else {
      throw new Error("Failed to create user");
    }
  } catch (err) {
    notify.showError(err.message || "Error creating user");
    throw err;
  }
}

export async function update(user, which = ["all"]) {
  try {
    // List of keys to exclude from the "which" array
    const excludeKeys = ["id", "name"];
    // Filter out the keys from "which"
    which = which.filter(item => !excludeKeys.includes(item));
    if (user.username === "publicUser") {
      return;
    }
    const apiPath = getApiPath("api/users", { id: user.id });
    await fetchURL(apiPath, {
      method: "PUT",
      body: JSON.stringify({
        what: "user",
        which: which,
        data: user,
      }),
    });
  } catch (err) {
    notify.showError(err.message || `Failed to update user with ID: ${user.id}`);
    throw err;
  }
}

export async function remove(id) {
  try {
    const apiPath = getApiPath("api/users", { id: id });
    await fetchURL(apiPath, {
      method: "DELETE",
    });
  } catch (err) {
    notify.showError(err.message || `Failed to delete user with ID: ${id}`);
    throw err;
  }
}
