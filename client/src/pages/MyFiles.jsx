import { useEffect, useState } from 'react';
import { deleteFile, listFiles } from '../api.js';
import { formatDate, humanSize } from '../util.js';

export default function MyFiles() {
  const [files, setFiles] = useState(null); // null = loading
  const [error, setError] = useState('');

  async function load() {
    try {
      setFiles(await listFiles());
    } catch (err) {
      setError(err.message);
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function remove(file) {
    if (!window.confirm(`Delete "${file.filename}"? This cannot be undone.`)) return;
    setError('');
    try {
      await deleteFile(file.id);
      setFiles((current) => current.filter((f) => f.id !== file.id));
    } catch (err) {
      setError(err.message);
    }
  }

  return (
    <div className="card wide">
      <div className="row">
        <h2>My files</h2>
        <button className="link" onClick={load}>
          Refresh
        </button>
      </div>

      {error && <p className="error">{error}</p>}
      {files === null && !error && <p>Loading…</p>}
      {files && files.length === 0 && <p>You haven't uploaded anything yet.</p>}

      {files && files.length > 0 && (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Size</th>
                <th>Status</th>
                <th>Expires</th>
                <th>ID</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {files.map((f) => (
                <tr key={f.id}>
                  <td>{f.filename}</td>
                  <td>{humanSize(f.size)}</td>
                  <td>
                    <span className={`badge ${f.status}`}>{f.status}</span>
                  </td>
                  <td>{formatDate(f.expires_at)}</td>
                  <td>
                    <button
                      className="link"
                      title={f.id}
                      onClick={() => navigator.clipboard.writeText(f.id)}
                    >
                      Copy ID
                    </button>
                  </td>
                  <td>
                    <button className="link danger" onClick={() => remove(f)}>
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
