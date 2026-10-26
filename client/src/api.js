const BASE = import.meta.env.VITE_API_URL || "http://localhost:8080";

// ---- session (token + user kept in localStorage) ----

export const session = {
  get token() {
    return localStorage.getItem("token");
  },
  get user() {
    try {
      return JSON.parse(localStorage.getItem("user"));
    } catch {
      return null;
    }
  },
  save(token, user) {
    localStorage.setItem("token", token);
    localStorage.setItem("user", JSON.stringify(user));
  },
  clear() {
    localStorage.removeItem("token");
    localStorage.removeItem("user");
  },
};

// If the server says our token is bad/expired, log out everywhere.
function handleUnauthorized(status) {
  if (status === 401 && session.token) {
    session.clear();
    window.dispatchEvent(new Event("logout"));
  }
}

async function request(path, { method = "GET", body } = {}) {
  const headers = {};
  if (body) headers["Content-Type"] = "application/json";
  if (session.token) headers.Authorization = `Bearer ${session.token}`;

  const res = await fetch(BASE + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  handleUnauthorized(res.status);
  return res;
}

async function requestJSON(path, options) {
  const res = await request(path, options);
  if (res.status === 204) return null;

  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Request failed (${res.status})`);
  return data;
}

// ---- auth ----

export async function register(name, email, password) {
  const data = await requestJSON("/register", {
    method: "POST",
    body: { name, email, password },
  });
  session.save(data.token, data.user);
  return data.user;
}

export async function login(email, password) {
  const data = await requestJSON("/login", {
    method: "POST",
    body: { email, password },
  });
  session.save(data.token, data.user);
  return data.user;
}

// ---- files ----

export async function listFiles() {
  const data = await requestJSON("/files");
  return data.files;
}

export function deleteFile(id) {
  return requestJSON(`/files/${id}`, { method: "DELETE" });
}

// Uses XHR instead of fetch so we get upload progress events.
// The server needs password and expires_at BEFORE the file part,
// so they are appended to the form first.
export function uploadFile(
  { file, password, expiresAt, uploaderName },
  onProgress,
) {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    form.append("password", password);
    form.append("expires_at", new Date(expiresAt).toISOString());
    if (uploaderName) form.append("uploader_name", uploaderName);
    form.append("file", file);

    const xhr = new XMLHttpRequest();
    xhr.open("POST", BASE + "/upload");
    xhr.setRequestHeader("Authorization", `Bearer ${session.token}`);

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable)
        onProgress(Math.round((e.loaded / e.total) * 100));
    };
    xhr.onerror = () => reject(new Error("Network error"));
    xhr.onload = () => {
      handleUnauthorized(xhr.status);
      let data = {};
      try {
        data = JSON.parse(xhr.responseText);
      } catch {}
      if (xhr.status >= 200 && xhr.status < 300) resolve(data);
      else reject(new Error(data.error || `Upload failed (${xhr.status})`));
    };

    xhr.send(form);
  });
}

// Checks the password, then hands the returned link to the browser, whose
// download manager streams it to disk and can resume it if interrupted.
// ---- downloads ----

// Step 1: check the password and get a short-lived download link.
export async function requestDownloadLink(fileId, password) {
  const data = await requestJSON("/download", {
    method: "POST",
    body: { file_id: fileId, password },
  });
  return { url: BASE + data.url, filename: data.filename, size: data.size };
}

// Option A: hand the link to the browser's own download manager.
export function saveWithBrowser(link) {
  const a = document.createElement("a");
  a.href = link.url;
  a.click();
}

// Option B: download inside the page, with pause / resume.
// Every (re)start asks the server for "Range: bytes=<received>-", which the
// server answers with 206 Partial Content.
export class ResumableDownload {
  constructor(link, onProgress) {
    this.link = link; // { url, filename, size }
    this.onProgress = onProgress; // called with 0..1
    this.received = 0;
    this.chunks = []; // only used when we can't write straight to disk
    this.writable = null; // file on disk (Chrome / Edge)
    this.controller = null;
  }

  // Call this from a click handler: the save dialog needs a user gesture.
  // Resolves to 'done', 'paused' or 'cancelled'; throws if the connection drops.
  async start() {
    if (window.showSaveFilePicker) {
      try {
        const handle = await window.showSaveFilePicker({
          suggestedName: this.link.filename,
        });
        this.writable = await handle.createWritable();
      } catch (err) {
        if (err.name === "AbortError") return "cancelled";
        // Any other problem: fall back to keeping the data in memory.
      }
    }
    return this.run();
  }

  resume() {
    return this.run();
  }

  pause() {
    this.controller?.abort();
  }

  async cancel() {
    this.controller?.abort();
    await this.writable?.abort().catch(() => {});
  }

  async run() {
    this.controller = new AbortController();

    try {
      if (this.link.size > 0) {
        const res = await fetch(this.link.url, {
          headers: { Range: `bytes=${this.received}-` },
          signal: this.controller.signal,
        });

        if (!res.ok) {
          const data = await res.json().catch(() => ({}));
          throw new Error(data.error || `Download failed (${res.status})`);
        }
        if (this.received > 0 && res.status !== 206) {
          throw new Error("The server did not accept the resume request");
        }

        const reader = res.body.getReader();
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;

          if (this.writable) await this.writable.write(value);
          else this.chunks.push(value);

          this.received += value.length;
          this.onProgress(this.received / this.link.size);
        }
      }
    } catch (err) {
      if (err.name === "AbortError") return "paused";
      if (err instanceof TypeError) throw new Error("Connection lost");
      throw err;
    }

    await this.finish();
    return "done";
  }

  async finish() {
    if (this.writable) {
      await this.writable.close();
      return;
    }

    const url = URL.createObjectURL(new Blob(this.chunks));
    const a = document.createElement("a");
    a.href = url;
    a.download = this.link.filename;
    a.click();
    URL.revokeObjectURL(url);
    this.chunks = [];
  }
}
