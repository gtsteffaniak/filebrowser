import { fetchURL, fetchJSON } from "@/api/utils";
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
    return await fetchJSON(`/api/users/${id}`, {});
  } catch (err) {
    notify.showError(err.message || `Failed to fetch user with ID: ${id}`);
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
    if (which.length > 0 && which.includes("password")) {
      user.password = "";
    }
    if (user.username === "publicUser") {
      return;
    }
    console.log("putting",which,user)
    await fetchURL(`/api/users/${user.id}`, {
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
    await fetchURL(`/api/users/${id}`, {
      method: "DELETE",
    });
  } catch (err) {
    notify.showError(err.message || `Failed to delete user with ID: ${id}`);
    throw err;
  }
}
