package icons

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/moasq/nanowave/internal/terminal"
)

// iconUploadPage is the HTML for the drag-and-drop icon upload page.
const iconUploadPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>App Icon — Nanowave</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'SF Pro', system-ui, sans-serif;
    background: #0a0a0a; color: #e5e5e5;
    display: flex; align-items: center; justify-content: center;
    min-height: 100vh; padding: 2rem;
  }
  .container { max-width: 480px; width: 100%; text-align: center; }
  h1 { font-size: 1.5rem; font-weight: 600; margin-bottom: 0.5rem; }
  .subtitle { color: #888; font-size: 0.9rem; margin-bottom: 2rem; }
  .drop-zone {
    border: 2px dashed #333; border-radius: 24px;
    padding: 3rem 2rem; cursor: pointer;
    transition: border-color 0.2s, background 0.2s;
    position: relative;
  }
  .drop-zone.active { border-color: #0a84ff; background: rgba(10,132,255,0.05); }
  .drop-zone.has-icon { border-style: solid; border-color: #333; padding: 1.5rem; }
  .drop-text { color: #666; font-size: 0.95rem; }
  .drop-text strong { color: #e5e5e5; }
  .icon-preview {
    width: 200px; height: 200px; border-radius: 44px;
    object-fit: cover; display: none; margin: 0 auto 1rem;
  }
  .drop-zone.has-icon .icon-preview { display: block; }
  .drop-zone.has-icon .drop-text { display: none; }
  .info { color: #666; font-size: 0.8rem; margin-top: 0.5rem; }
  .btn {
    display: inline-block; margin-top: 1.5rem; padding: 0.75rem 2rem;
    background: #0a84ff; color: #fff; border: none; border-radius: 12px;
    font-size: 1rem; font-weight: 500; cursor: pointer;
    transition: opacity 0.2s; opacity: 0;
    pointer-events: none;
  }
  .btn.visible { opacity: 1; pointer-events: auto; }
  .btn:hover { opacity: 0.85; }
  .btn:disabled { opacity: 0.4; cursor: not-allowed; }
  .skip { color: #555; font-size: 0.85rem; margin-top: 1rem; cursor: pointer; }
  .skip:hover { color: #888; }
  .status { margin-top: 1rem; font-size: 0.9rem; }
  .status.error { color: #ff453a; }
  .status.success { color: #30d158; }
  .done-msg { margin-top: 2rem; }
  .done-msg h2 { color: #30d158; font-size: 1.2rem; margin-bottom: 0.5rem; }
  .done-msg p { color: #888; }
  input[type=file] { display: none; }
</style>
</head>
<body>
<div class="container">
  <h1>App Icon</h1>
  <p class="subtitle">1024x1024 PNG required for App Store</p>
  <div class="drop-zone" id="dropZone">
    <img class="icon-preview" id="preview">
    <div class="drop-text">
      <p><strong>Drop your icon here</strong></p>
      <p>or click to browse</p>
    </div>
    <p class="info">PNG, 1024x1024 pixels</p>
  </div>
  <input type="file" id="fileInput" accept="image/png,image/jpeg">
  <button class="btn" id="uploadBtn">Set Icon</button>
  <p class="skip" id="skipBtn">Skip — let Claude handle it</p>
  <div class="status" id="status"></div>
  <div class="done-msg" id="done" style="display:none">
    <h2>Icon set</h2>
    <p>You can close this tab and return to the terminal.</p>
  </div>
</div>
<script>
const dropZone = document.getElementById('dropZone');
const preview = document.getElementById('preview');
const fileInput = document.getElementById('fileInput');
const uploadBtn = document.getElementById('uploadBtn');
const skipBtn = document.getElementById('skipBtn');
const status = document.getElementById('status');
const done = document.getElementById('done');
let selectedFile = null;

dropZone.addEventListener('click', () => fileInput.click());
dropZone.addEventListener('dragover', e => { e.preventDefault(); dropZone.classList.add('active'); });
dropZone.addEventListener('dragleave', () => dropZone.classList.remove('active'));
dropZone.addEventListener('drop', e => {
  e.preventDefault(); dropZone.classList.remove('active');
  if (e.dataTransfer.files.length) handleFile(e.dataTransfer.files[0]);
});
fileInput.addEventListener('change', () => { if (fileInput.files.length) handleFile(fileInput.files[0]); });

function handleFile(file) {
  if (!file.type.startsWith('image/')) { setStatus('Please select an image file', true); return; }
  selectedFile = file;
  const url = URL.createObjectURL(file);
  preview.src = url;
  dropZone.classList.add('has-icon');
  uploadBtn.classList.add('visible');
  status.textContent = '';
}

uploadBtn.addEventListener('click', async () => {
  if (!selectedFile) return;
  uploadBtn.disabled = true;
  uploadBtn.textContent = 'Setting icon...';
  status.textContent = '';
  const form = new FormData();
  form.append('icon', selectedFile);
  try {
    const res = await fetch('/upload', { method: 'POST', body: form });
    const data = await res.json();
    if (data.ok) {
      dropZone.style.display = 'none';
      uploadBtn.style.display = 'none';
      skipBtn.style.display = 'none';
      done.style.display = 'block';
    } else {
      setStatus(data.error || 'Upload failed', true);
      uploadBtn.disabled = false; uploadBtn.textContent = 'Set Icon';
    }
  } catch (e) {
    setStatus('Connection error', true);
    uploadBtn.disabled = false; uploadBtn.textContent = 'Set Icon';
  }
});

skipBtn.addEventListener('click', async () => {
  await fetch('/skip', { method: 'POST' });
  dropZone.style.display = 'none';
  uploadBtn.style.display = 'none';
  skipBtn.style.display = 'none';
  done.querySelector('h2').textContent = 'Skipped';
  done.querySelector('h2').style.color = '#888';
  done.querySelector('p').textContent = 'You can close this tab.';
  done.style.display = 'block';
});

function setStatus(msg, isError) {
  status.textContent = msg;
  status.className = 'status' + (isError ? ' error' : '');
}
</script>
</body>
</html>`

// RunUploadServer starts a temporary local HTTP server for icon upload.
// Opens the browser, waits for upload or skip, then shuts down.
// Returns true if an icon was set.
func RunUploadServer(ctx context.Context, appIconDir, platform string) bool {
	done := make(chan bool, 1)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(iconUploadPage))
	})

	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		file, header, err := r.FormFile("icon")
		if err != nil {
			w.Write([]byte(`{"ok":false,"error":"no file received"}`))
			return
		}
		defer file.Close()

		// Determine filename
		ext := filepath.Ext(header.Filename)
		if ext == "" {
			ext = ".png"
		}
		iconFilename := "AppIcon" + ext

		// Save to AppIcon.appiconset
		destPath := filepath.Join(appIconDir, iconFilename)
		out, err := os.Create(destPath)
		if err != nil {
			w.Write([]byte(fmt.Sprintf(`{"ok":false,"error":"failed to save: %s"}`, err.Error())))
			return
		}
		if _, err := io.Copy(out, file); err != nil {
			out.Close()
			w.Write([]byte(fmt.Sprintf(`{"ok":false,"error":"failed to write: %s"}`, err.Error())))
			return
		}
		out.Close()

		// Update Contents.json
		if err := UpdateContentsJSON(appIconDir, iconFilename, platform); err != nil {
			log.Printf("[asc] icon Contents.json update failed: %v", err)
			w.Write([]byte(fmt.Sprintf(`{"ok":false,"error":"icon saved but Contents.json update failed: %s"}`, err.Error())))
			return
		}

		log.Printf("[asc] icon uploaded: %s -> %s", header.Filename, destPath)
		w.Write([]byte(`{"ok":true}`))

		// Signal done
		select {
		case done <- true:
		default:
		}
	})

	mux.HandleFunc("/skip", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
		select {
		case done <- false:
		default:
		}
	})

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Printf("[asc] failed to start icon server: %v", err)
		return false
	}
	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	// Open browser
	log.Printf("[asc] icon upload server at %s", url)
	terminal.Info(fmt.Sprintf("Opening browser for icon upload: %s", url))
	_ = exec.Command("open", url).Start()

	// Wait for upload, skip, or context cancellation
	select {
	case iconSet := <-done:
		server.Shutdown(context.Background())
		if iconSet {
			terminal.Success("App icon set")
			return true
		}
		terminal.Info("Icon upload skipped")
		return false
	case <-ctx.Done():
		server.Shutdown(context.Background())
		return false
	}
}
