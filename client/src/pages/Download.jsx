import { useRef, useState } from "react";
import {
  requestDownloadLink,
  ResumableDownload,
  saveWithBrowser,
} from "../api.js";
import { humanSize } from "../util.js";

export default function Download() {
  const [fileId, setFileId] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  // idle | checking | downloading | paused | interrupted | done
  const [state, setState] = useState("idle");
  const [progress, setProgress] = useState(0);
  const [info, setInfo] = useState(null); // { filename, size }
  const download = useRef(null);

  // Runs start() / resume() and moves the UI to the matching state.
  async function run(action) {
    setError("");
    setState("downloading");
    try {
      const result = await action();
      if (result === "cancelled") {
        setState("idle");
      } else if (result === "done") {
        setProgress(1);
        setState("done");
      } else {
        setState("paused");
      }
    } catch (err) {
      setError(err.message);
      setState("interrupted"); // keeps what was received, Resume is available
    }
  }

  async function begin(e) {
    e.preventDefault();
    setError("");
    setState("checking");
    try {
      const link = await requestDownloadLink(fileId.trim(), password);
      setInfo(link);
      setProgress(0);
      setPassword("");
      download.current = new ResumableDownload(link, setProgress);
      await run(() => download.current.start());
    } catch (err) {
      setError(err.message);
      setState("idle");
    }
  }

  async function useBrowser() {
    setError("");
    try {
      saveWithBrowser(await requestDownloadLink(fileId.trim(), password));
    } catch (err) {
      setError(err.message);
    }
  }

  async function cancel() {
    await download.current?.cancel();
    setState("idle");
    setInfo(null);
    setProgress(0);
  }

  // ---- password form ----

  if (state === "idle" || state === "checking") {
    return (
      <form className="card" onSubmit={begin}>
        <h2>Download a file</h2>
        <p className="muted">
          No account needed. You only need the file ID and its password.
        </p>

        <label>
          File ID
          <input
            value={fileId}
            onChange={(e) => setFileId(e.target.value)}
            required
          />
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

        <button className="primary" disabled={state === "checking"}>
          {state === "checking" ? "Checking…" : "Download"}
        </button>

        <button
          type="button"
          className="link"
          disabled={!fileId || !password}
          onClick={useBrowser}
        >
          Or let the browser download it
        </button>
      </form>
    );
  }

  // ---- transfer card ----

  const percent = Math.round(progress * 100);
  const received = Math.round(progress * info.size);

  const statusText = {
    downloading: "Downloading…",
    paused: "Paused",
    interrupted: error || "Connection lost",
    done: "Download complete",
  }[state];

  return (
    <div className="card transfer">
      <div className="file-head">
        <span className="file-icon">📄</span>
        <div>
          <div className="file-name">{info.filename}</div>
          <div className="muted">{humanSize(info.size)}</div>
        </div>
      </div>

      <div className="progress-row">
        <div className={`bar ${state}`}>
          <div style={{ width: `${percent}%` }} />
        </div>
        <span className="percent">{percent}%</span>
      </div>

      <div className="muted center">
        {humanSize(received)} / {humanSize(info.size)}
      </div>

      <p className={`status ${state}`}>{statusText}</p>

      <div className="actions">
        {state === "downloading" && (
          <button className="primary" onClick={() => download.current.pause()}>
            Pause
          </button>
        )}

        {(state === "paused" || state === "interrupted") && (
          <button
            className="primary"
            onClick={() => run(() => download.current.resume())}
          >
            Resume
          </button>
        )}

        {state === "done" ? (
          <button className="link" onClick={cancel}>
            Download another file
          </button>
        ) : (
          <button className="link danger" onClick={cancel}>
            Cancel
          </button>
        )}
      </div>
    </div>
  );
}
