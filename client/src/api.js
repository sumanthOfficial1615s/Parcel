const BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080';

// ---- session (token + user kept in localStorage) ----

export const session = {
  get token() {
    return localStorage.getItem('token');
  },
  get user() {
    try {
      return JSON.parse(localStorage.getItem('user'));
    } catch {
      return null;
    }
  },
  save(token, user) {
    localStorage.setItem('token', token);
    localStorage.setItem('user', JSON.stringify(user));
  },
  clear() {
    localStorage.removeItem('token');
    localStorage.removeItem('user');
  },
};

// If the server says our token is bad/expired, log out everywhere.
function handleUnauthorized(status) {
  if (status === 401 && session.token) {
    session.clear();
    window.dispatchEvent(new Event('logout'));
  }
}

async function request(path, { method = 'GET', body } = {}) {
  const headers = {};
  if (body) headers['Content-Type'] = 'application/json';
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
  const data = await requestJSON('/register', {
    method: 'POST',
    body: { name, email, password },
  });
  session.save(data.token, data.user);
  return data.user;
}

export async function login(email, password) {
  const data = await requestJSON('/login', {
    method: 'POST',
    body: { email, password },
  });
  session.save(data.token, data.user);
  return data.user;
}

// ---- files ----

export async function listFiles() {
  const data = await requestJSON('/files');
  return data.files;
}

export function deleteFile(id) {
  return requestJSON(`/files/${id}`, { method: 'DELETE' });
}

// Uses XHR instead of fetch so we get upload progress events.
// The server needs password and expires_at BEFORE the file part,
// so they are appended to the form first.
export function uploadFile({ file, password, expiresAt, uploaderName }, onProgress) {
  return new Promise((resolve, reject) => {
    const form = new FormData();
    form.append('password', password);
    form.append('expires_at', new Date(expiresAt).toISOString());
    if (uploaderName) form.append('uploader_name', uploaderName);
    form.append('file', file);

    const xhr = new XMLHttpRequest();
    xhr.open('POST', BASE + '/upload');
    xhr.setRequestHeader('Authorization', `Bearer ${session.token}`);

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100));
    };
    xhr.onerror = () => reject(new Error('Network error'));
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

// Downloads the file through the browser's normal "save" flow.
export async function downloadFile(fileId, password) {
  const res = await request('/download', {
    method: 'POST',
    body: { file_id: fileId, password },
  });

  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new Error(data.error || `Download failed (${res.status})`);
  }

  // Needs Access-Control-Expose-Headers: Content-Disposition on the server.
  const disposition = res.headers.get('Content-Disposition') || '';
  const match = disposition.match(/filename="?([^"]+)"?/);
  const filename = match ? match[1] : fileId;

  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);

  return filename;
}
