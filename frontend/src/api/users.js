import { fetchURL, fetchJSON } from "@/api/utils";

export async function getAllUsers() {
  return await fetchJSON(`/api/users`, {});
}

export async function get(id) {
  return fetchJSON(`/api/users/${id}`, {});
}

export async function create(user) {
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
  }
}

export async function update(user, which = ["all"]) {
  if (which[0] != "password") {
    user.password = "";
  }
  console.log("updating user",user,which)
  await fetchURL(`/api/users/${user.id}`, {
    method: "PUT",
    body: JSON.stringify({
      what: "user",
      which: which,
      data: user,
    }),
  });
}

export async function remove(id) {
  await fetchURL(`/api/users/${id}`, {
    method: "DELETE",
  });
}
