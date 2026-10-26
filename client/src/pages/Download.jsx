import { useState } from 'react';
import { downloadFile } from '../api.js';

export default function Download() {
  const [fileId, setFileId] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [done, setDone] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(e) {
    e.preventDefault();
    setError('');
    setDone('');
    setBusy(true);
    try {
      const name = await downloadFile(fileId.trim(), password);
      setDone(`Downloaded ${name}`);
    } catch (err) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="card" onSubmit={submit}>
      <h2>Download a file</h2>
      <p className="muted">No account needed. You only need the file ID and its password.</p>

      <label>
        File ID
        <input value={fileId} onChange={(e) => setFileId(e.target.value)} required />
      </label>

      <label>
        Password
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
      </label>

      {error && <p className="error">{error}</p>}
      {done && <p className="ok">{done}</p>}

      <button className="primary" disabled={busy}>
        {busy ? 'Downloading…' : 'Download'}
      </button>
    </form>
  );
}
