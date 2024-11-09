import { fetchURL, fetchJSON, createURL } from "@/api/utils";
import { notify } from "@/notify";  // Import notify for error handling

export async function getAllUsers() {
  try {
    return await fetchJSON(`/api/users`, {});
  } catch (err) {
    notify.showError(err.message || "Failed to fetch users");
    throw err; // Re-throw to handle further if needed
  }
}


export async function get(id) {
  try {
    return await fetchJSON(`/api/users?id=${id}`, {});
  } catch (err) {
    notify.showError(err.message || `Failed to fetch user with ID: ${id}`);
    throw err;
  }
}

export async function getApiKeys(key="") {

  try {
    let buildUrl =   "api/auth/tokens"
    if (key != "") {
      buildUrl = buildUrl + "?key="+key
    }
    const url = createURL(buildUrl)
    return await fetchJSON(url);
  } catch (err) {
    notify.showError(err.message || `Failed to get api keys`);
    throw err;
  }
}


export async function createApiKey(params) {
  try {
    const url = createURL(`api/auth/token`, params)
    await fetchURL(url, {
      method: "PUT",
    });  } catch (err) {
    notify.showError(err.message || `Failed to create API key`);
    throw err;
  }
}

export async function create(user) {
  try {
    const res = await fetchURL(`/api/users`, {
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

    await fetchURL(`/api/users?id=${user.id}`, {
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
    await fetchURL(`/api/users?id=${id}`, {
      method: "DELETE",
    });
  } catch (err) {
    notify.showError(err.message || `Failed to delete user with ID: ${id}`);
    throw err;
  }
}
