import { useEffect, useState } from 'react';
import { session } from './api.js';
import Auth from './pages/Auth.jsx';
import Upload from './pages/Upload.jsx';
import MyFiles from './pages/MyFiles.jsx';
import Download from './pages/Download.jsx';

export default function App() {
  const [user, setUser] = useState(session.user);
  const [tab, setTab] = useState('download');

  // api.js fires this when the server rejects our token.
  useEffect(() => {
    const onLogout = () => {
      setUser(null);
      setTab('login');
    };
    window.addEventListener('logout', onLogout);
    return () => window.removeEventListener('logout', onLogout);
  }, []);

  function handleLogin(u) {
    setUser(u);
    setTab('upload');
  }

  function handleLogout() {
    session.clear();
    setUser(null);
    setTab('download');
  }

  const tabs = user
    ? [
        ['upload', 'Upload'],
        ['files', 'My files'],
        ['download', 'Download'],
      ]
    : [
        ['download', 'Download'],
        ['login', 'Log in / Sign up'],
      ];

  return (
    <div className="app">
      <header>
        <h1>Parcel</h1>
        <nav>
          {tabs.map(([id, label]) => (
            <button
              key={id}
              className={tab === id ? 'tab active' : 'tab'}
              onClick={() => setTab(id)}
            >
              {label}
            </button>
          ))}
        </nav>
        {user && (
          <div className="who">
            <span>{user.name}</span>
            <button className="link" onClick={handleLogout}>
              Log out
            </button>
          </div>
        )}
      </header>

      <main>
        {tab === 'login' && !user && <Auth onSuccess={handleLogin} />}
        {tab === 'upload' && user && <Upload />}
        {tab === 'files' && user && <MyFiles />}
        {tab === 'download' && <Download />}
      </main>
    </div>
  );
}
