import { useState } from 'react';
import { uploadFile } from '../api.js';
import { humanSize, nowForInput } from '../util.js';

export default function Upload() {
  const [file, setFile] = useState(null);
  const [password, setPassword] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [uploaderName, setUploaderName] = useState('');
  const [progress, setProgress] = useState(null); // null = not uploading
  const [error, setError] = useState('');
  const [result, setResult] = useState(null);
  const [fileInputKey, setFileInputKey] = useState(0); // changing the key clears the file input

  async function submit(e) {
    e.preventDefault();
    setError('');
    setResult(null);
    setProgress(0);
    try {
      const data = await uploadFile(
        { file, password, expiresAt, uploaderName },
        setProgress
      );
      setResult(data);
      setFile(null);
      setPassword('');
      setExpiresAt('');
      setFileInputKey((k) => k + 1);
    } catch (err) {
      setError(err.message);
    } finally {
      setProgress(null);
    }
  }

  const uploading = progress !== null;

  return (
    <form className="card" onSubmit={submit}>
      <h2>Upload a file</h2>

      <label>
        File
        <input
          key={fileInputKey}
          type="file"
          onChange={(e) => setFile(e.target.files[0] || null)}
          required
        />
        {file && <small>{humanSize(file.size)}</small>}
      </label>

      <label>
        Link password
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          required
        />
        <small>Whoever downloads the file will need this.</small>
      </label>

      <label>
        Expires at
        <input
          type="datetime-local"
          min={nowForInput()}
          value={expiresAt}
          onChange={(e) => setExpiresAt(e.target.value)}
          required
        />
      </label>

      <label>
        Uploader name <small>(optional)</small>
        <input value={uploaderName} onChange={(e) => setUploaderName(e.target.value)} />
      </label>

      {uploading && (
        <div className="bar">
          <div style={{ width: `${progress}%` }} />
        </div>
      )}

      {error && <p className="error">{error}</p>}

      <button className="primary" disabled={uploading || !file}>
        {uploading ? `Uploading… ${progress}%` : 'Upload'}
      </button>

      {result && (
        <div className="success">
          <p>
            <strong>{result.filename}</strong> uploaded ({humanSize(result.size)}).
            It is being virus-scanned and can be downloaded once it is clean.
          </p>
          <p>
            File ID: <code>{result.id}</code>{' '}
            <button
              type="button"
              className="link"
              onClick={() => navigator.clipboard.writeText(result.id)}
            >
              Copy
            </button>
          </p>
          <p>Share the ID and the password with the recipient.</p>
        </div>
      )}
    </form>
  );
}
