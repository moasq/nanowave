package screenshots

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/moasq/nanowave/internal/terminal"
)

// RunUploadServer starts a temporary local HTTP server for screenshot upload.
// Opens the browser, waits for done or skip, then shuts down.
// Returns true if screenshots were uploaded.
func RunUploadServer(ctx context.Context, screenshotDir string, reqs ScreenshotRequirements) bool {
	done := make(chan bool, 1)

	// Build requirements JSON for embedding in the page
	type acceptedDim struct {
		Width  int    `json:"w"`
		Height int    `json:"h"`
		Device string `json:"device"`
	}
	type reqsPayload struct {
		Required   []string     `json:"required"`
		Accepted   []acceptedDim `json:"accepted"`
	}

	var accepted []acceptedDim
	for dim, dt := range dimensionToDevice {
		accepted = append(accepted, acceptedDim{Width: dim[0], Height: dim[1], Device: dt})
	}
	reqsData := reqsPayload{Required: reqs.Required, Accepted: accepted}
	reqsJSON, _ := json.Marshal(reqsData)

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := strings.Replace(screenshotUploadPage, "/*REQUIREMENTS_JSON*/", string(reqsJSON), 1)
		w.Write([]byte(page))
	})

	mux.HandleFunc("/requirements", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(reqsJSON)
	})

	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", 405)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		if err := r.ParseMultipartForm(100 << 20); err != nil {
			json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "failed to parse form"})
			return
		}

		files := r.MultipartForm.File["screenshots"]
		if len(files) == 0 {
			json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "no files received"})
			return
		}

		var saved []map[string]string
		for _, fh := range files {
			src, err := fh.Open()
			if err != nil {
				log.Printf("[screenshots] failed to open uploaded file %s: %v", fh.Filename, err)
				continue
			}

			destPath := filepath.Join(screenshotDir, fh.Filename)
			out, err := os.Create(destPath)
			if err != nil {
				src.Close()
				log.Printf("[screenshots] failed to create %s: %v", destPath, err)
				continue
			}
			if _, err := io.Copy(out, src); err != nil {
				out.Close()
				src.Close()
				log.Printf("[screenshots] failed to write %s: %v", destPath, err)
				continue
			}
			out.Close()
			src.Close()

			dt := detectDeviceType(destPath)
			if dt == "" {
				dt = "UNKNOWN"
			}
			log.Printf("[screenshots] uploaded: %s -> %s (device: %s)", fh.Filename, destPath, dt)
			saved = append(saved, map[string]string{
				"filename":   fh.Filename,
				"deviceType": dt,
			})
		}

		// Check fulfillment
		fulfilled, missing := ValidateScreenshots(screenshotDir, reqs)

		resp := map[string]any{
			"ok":          true,
			"count":       len(saved),
			"screenshots": saved,
			"fulfilled":   fulfilled,
			"missing":     missing,
		}
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/done", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
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

	mux.HandleFunc("/screenshots", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		screenshots := ListScreenshots(screenshotDir)
		var items []map[string]string
		for _, s := range screenshots {
			items = append(items, map[string]string{
				"filename":   s.Filename,
				"deviceType": s.DeviceType,
				"url":        fmt.Sprintf("/screenshots/%s", s.Filename),
			})
		}
		json.NewEncoder(w).Encode(items)
	})

	mux.HandleFunc("/screenshots/", func(w http.ResponseWriter, r *http.Request) {
		filename := filepath.Base(r.URL.Path)
		filePath := filepath.Join(screenshotDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			http.Error(w, "not found", 404)
			return
		}
		http.ServeFile(w, r, filePath)
	})

	// Find a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Printf("[screenshots] failed to start server: %v", err)
		return false
	}
	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	server := &http.Server{Handler: mux}
	go server.Serve(listener)

	// Open browser
	log.Printf("[screenshots] upload server at %s", url)
	terminal.Info(fmt.Sprintf("Opening browser for screenshot upload: %s", url))
	_ = exec.Command("open", url).Start()

	// Wait for done, skip, or context cancellation
	select {
	case uploaded := <-done:
		server.Shutdown(context.Background())
		if uploaded {
			terminal.Success("Screenshots uploaded")
			return true
		}
		terminal.Info("Screenshot upload skipped")
		return false
	case <-ctx.Done():
		server.Shutdown(context.Background())
		return false
	}
}

const screenshotUploadPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>App Screenshots — Nanowave</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'SF Pro', system-ui, sans-serif;
    background: #0a0a0a; color: #e5e5e5;
    display: flex; align-items: center; justify-content: center;
    min-height: 100vh; padding: 2rem;
  }
  .container { max-width: 780px; width: 100%; text-align: center; }
  h1 { font-size: 1.5rem; font-weight: 600; margin-bottom: 0.5rem; }
  .subtitle { color: #888; font-size: 0.9rem; margin-bottom: 1.5rem; }

  /* Requirements checklist */
  .requirements {
    background: #111; border-radius: 16px; padding: 1.25rem 1.5rem;
    margin-bottom: 1.5rem; text-align: left;
  }
  .requirements h2 { font-size: 0.85rem; color: #888; font-weight: 500; margin-bottom: 0.75rem; text-transform: uppercase; letter-spacing: 0.05em; }
  .req-item {
    display: flex; align-items: center; gap: 0.6rem;
    padding: 0.4rem 0; font-size: 0.9rem;
  }
  .req-check {
    width: 20px; height: 20px; border-radius: 50%;
    border: 2px solid #333; display: flex; align-items: center; justify-content: center;
    font-size: 0.7rem; flex-shrink: 0; transition: all 0.3s;
  }
  .req-check.done { border-color: #30d158; background: #1c3a1c; color: #30d158; }
  .req-label { color: #ccc; }
  .req-dims { color: #666; font-size: 0.8rem; margin-left: auto; }
  .req-summary {
    margin-top: 0.75rem; padding-top: 0.75rem; border-top: 1px solid #222;
    font-size: 0.85rem; color: #888;
  }
  .req-summary.complete { color: #30d158; }

  .drop-zone {
    border: 2px dashed #333; border-radius: 24px;
    padding: 3rem 2rem; cursor: pointer;
    transition: border-color 0.2s, background 0.2s;
  }
  .drop-zone.active { border-color: #0a84ff; background: rgba(10,132,255,0.05); }
  .drop-text { color: #666; font-size: 0.95rem; }
  .drop-text strong { color: #e5e5e5; }
  .grid {
    display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 1rem; margin-top: 1.5rem;
  }
  .thumb {
    background: #1a1a1a; border-radius: 12px; padding: 0.5rem;
    text-align: center; overflow: hidden; border: 2px solid transparent;
    transition: border-color 0.3s;
  }
  .thumb.invalid { border-color: #ff453a; }
  .thumb img {
    width: 100%; border-radius: 8px; aspect-ratio: 9/19.5;
    object-fit: cover; margin-bottom: 0.4rem;
  }
  .thumb .name { font-size: 0.7rem; color: #888; word-break: break-all; }
  .thumb .badge {
    display: inline-block; margin-top: 0.3rem; padding: 0.15rem 0.5rem;
    background: #1c3a1c; color: #30d158; border-radius: 6px;
    font-size: 0.65rem; font-weight: 500;
  }
  .thumb .badge.invalid { background: #3a1c1c; color: #ff453a; }
  .thumb .badge.unknown { background: #3a2a1c; color: #ff9f0a; }
  .thumb .warning { font-size: 0.65rem; color: #ff9f0a; margin-top: 0.2rem; }

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
  .btn-secondary {
    display: none; margin-top: 0.5rem; padding: 0.5rem 1.25rem;
    background: transparent; color: #888; border: 1px solid #333; border-radius: 10px;
    font-size: 0.8rem; cursor: pointer; transition: opacity 0.2s;
  }
  .btn-secondary.visible { display: inline-block; }
  .btn-secondary:hover { color: #ccc; border-color: #555; }
  .btn-row { display: flex; gap: 1rem; justify-content: center; flex-wrap: wrap; }
  .btn-done {
    display: inline-block; margin-top: 1rem; padding: 0.6rem 1.5rem;
    background: #30d158; color: #fff; border: none; border-radius: 12px;
    font-size: 0.9rem; font-weight: 500; cursor: pointer;
  }
  .btn-done:hover { opacity: 0.85; }
  .skip { color: #555; font-size: 0.85rem; margin-top: 1rem; cursor: pointer; }
  .skip:hover { color: #888; }
  .status { margin-top: 1rem; font-size: 0.9rem; }
  .status.error { color: #ff453a; }
  .status.success { color: #30d158; }
  input[type=file] { display: none; }
  .hidden { display: none; }
</style>
</head>
<body>
<div class="container">
  <h1>App Screenshots</h1>
  <p class="subtitle">PNG or JPEG, matching required device dimensions</p>

  <div class="requirements" id="reqsSection"></div>

  <div class="drop-zone" id="dropZone">
    <div class="drop-text">
      <p><strong>Drop your screenshots here</strong></p>
      <p>or click to browse</p>
    </div>
  </div>
  <input type="file" id="fileInput" accept="image/png,image/jpeg" multiple>
  <div class="grid" id="grid"></div>
  <div>
    <button class="btn" id="uploadBtn">Upload Screenshots</button>
  </div>
  <div>
    <button class="btn-secondary" id="uploadAnywayBtn">Upload Anyway</button>
  </div>
  <div class="status" id="status"></div>
  <div class="hidden" id="postUpload">
    <div class="btn-row">
      <button class="btn visible" id="addMoreBtn" style="background:#333">Add More</button>
      <button class="btn-done" id="doneBtn">Done</button>
    </div>
  </div>
  <p class="skip" id="skipBtn">Skip — continue without screenshots</p>
</div>
<script>
const REQS = /*REQUIREMENTS_JSON*/;
const dropZone = document.getElementById('dropZone');
const fileInput = document.getElementById('fileInput');
const uploadBtn = document.getElementById('uploadBtn');
const uploadAnywayBtn = document.getElementById('uploadAnywayBtn');
const skipBtn = document.getElementById('skipBtn');
const grid = document.getElementById('grid');
const status = document.getElementById('status');
const postUpload = document.getElementById('postUpload');
const doneBtn = document.getElementById('doneBtn');
const addMoreBtn = document.getElementById('addMoreBtn');
const reqsSection = document.getElementById('reqsSection');

// Build dimension lookup from server-provided data
const deviceDims = {};
REQS.accepted.forEach(a => {
  deviceDims[a.w + 'x' + a.h] = a.device;
});

const deviceLabels = {
  'IPHONE_69': 'iPhone 6.9"',
  'IPHONE_65': 'iPhone 6.5"',
  'IPHONE_63': 'iPhone 6.3"',
  'IPAD_PRO_13': 'iPad 13"'
};

const deviceDimDescriptions = {
  'IPHONE_69': '1320x2868, 1290x2796, or 1260x2736',
  'IPHONE_65': '1284x2778 or 1242x2688',
  'IPAD_PRO_13': '2064x2752 or 2048x2732'
};

let selectedFiles = [];
let fulfilledTypes = new Set();

// Render requirements checklist
function renderRequirements() {
  let html = '<h2>Required Screenshots</h2>';
  let fulfilledCount = 0;
  REQS.required.forEach(req => {
    // For IPHONE_69, also accept IPHONE_65
    let isFulfilled = fulfilledTypes.has(req);
    if (req === 'IPHONE_69' && fulfilledTypes.has('IPHONE_65')) isFulfilled = true;
    if (isFulfilled) fulfilledCount++;

    const label = deviceLabels[req] || req;
    const dims = deviceDimDescriptions[req] || '';
    html += '<div class="req-item">';
    html += '<div class="req-check' + (isFulfilled ? ' done' : '') + '">' + (isFulfilled ? '&#10003;' : '') + '</div>';
    html += '<span class="req-label">' + label + '</span>';
    if (dims) html += '<span class="req-dims">' + dims + '</span>';
    html += '</div>';
  });
  const isComplete = fulfilledCount === REQS.required.length;
  html += '<div class="req-summary' + (isComplete ? ' complete' : '') + '">';
  html += fulfilledCount + '/' + REQS.required.length + ' required types covered';
  if (!isComplete) {
    const missingLabels = REQS.required
      .filter(r => {
        if (fulfilledTypes.has(r)) return false;
        if (r === 'IPHONE_69' && fulfilledTypes.has('IPHONE_65')) return false;
        return true;
      })
      .map(r => deviceLabels[r] || r);
    if (missingLabels.length > 0) html += ' — ' + missingLabels.join(', ') + ' still needed';
  }
  html += '</div>';
  reqsSection.innerHTML = html;
  return isComplete;
}

renderRequirements();

dropZone.addEventListener('click', () => fileInput.click());
dropZone.addEventListener('dragover', e => { e.preventDefault(); dropZone.classList.add('active'); });
dropZone.addEventListener('dragleave', () => dropZone.classList.remove('active'));
dropZone.addEventListener('drop', e => {
  e.preventDefault(); dropZone.classList.remove('active');
  addFiles(Array.from(e.dataTransfer.files));
});
fileInput.addEventListener('change', () => {
  addFiles(Array.from(fileInput.files));
  fileInput.value = '';
});

function addFiles(files) {
  const imgs = files.filter(f => f.type.startsWith('image/'));
  if (!imgs.length) { setStatus('Please select image files', true); return; }
  selectedFiles.push(...imgs);
  status.textContent = '';
  renderPreviews();
}

function renderPreviews() {
  grid.innerHTML = '';
  let pendingChecks = selectedFiles.length;
  const previewTypes = new Set();

  selectedFiles.forEach(file => {
    const thumb = document.createElement('div');
    thumb.className = 'thumb';
    const img = document.createElement('img');
    img.src = URL.createObjectURL(file);
    const name = document.createElement('div');
    name.className = 'name';
    name.textContent = file.name;
    const badge = document.createElement('div');

    img.onload = function() {
      const key = this.naturalWidth + 'x' + this.naturalHeight;
      const dt = deviceDims[key];
      if (dt) {
        badge.className = 'badge';
        badge.textContent = deviceLabels[dt] || dt;
        previewTypes.add(dt);
      } else {
        badge.className = 'badge invalid';
        badge.textContent = 'Invalid size (' + this.naturalWidth + 'x' + this.naturalHeight + ')';
        thumb.classList.add('invalid');
        const warn = document.createElement('div');
        warn.className = 'warning';
        warn.textContent = "Doesn't match any required size";
        thumb.appendChild(warn);
      }
      pendingChecks--;
      if (pendingChecks === 0) {
        // Merge existing fulfilled with preview types
        const allTypes = new Set([...fulfilledTypes, ...previewTypes]);
        fulfilledTypes = allTypes;
        const isComplete = renderRequirements();
        updateUploadButtons(isComplete);
      }
    };

    thumb.appendChild(img);
    thumb.appendChild(name);
    thumb.appendChild(badge);
    grid.appendChild(thumb);
  });
}

function updateUploadButtons(isComplete) {
  if (selectedFiles.length === 0) {
    uploadBtn.classList.remove('visible');
    uploadAnywayBtn.classList.remove('visible');
    return;
  }
  if (isComplete) {
    uploadBtn.classList.add('visible');
    uploadBtn.disabled = false;
    uploadAnywayBtn.classList.remove('visible');
  } else {
    uploadBtn.classList.add('visible');
    uploadBtn.disabled = true;
    uploadBtn.title = 'Upload all required device types first';
    uploadAnywayBtn.classList.add('visible');
  }
}

async function doUpload() {
  if (!selectedFiles.length) return;
  uploadBtn.disabled = true;
  uploadBtn.textContent = 'Uploading...';
  uploadAnywayBtn.classList.remove('visible');
  status.textContent = '';
  const form = new FormData();
  selectedFiles.forEach(f => form.append('screenshots', f));
  try {
    const res = await fetch('/upload', { method: 'POST', body: form });
    const data = await res.json();
    if (data.ok) {
      // Update fulfillment from server response
      fulfilledTypes = new Set(data.fulfilled || []);
      renderRequirements();

      let msg = data.count + ' screenshot(s) uploaded';
      if (data.missing && data.missing.length > 0) {
        msg += ' (missing: ' + data.missing.map(m => deviceLabels[m] || m).join(', ') + ')';
      }
      setStatus(msg, false);
      uploadBtn.classList.remove('visible');
      uploadAnywayBtn.classList.remove('visible');
      postUpload.classList.remove('hidden');
      skipBtn.style.display = 'none';
      selectedFiles = [];
    } else {
      setStatus(data.error || 'Upload failed', true);
      uploadBtn.disabled = false; uploadBtn.textContent = 'Upload Screenshots';
    }
  } catch (e) {
    setStatus('Connection error', true);
    uploadBtn.disabled = false; uploadBtn.textContent = 'Upload Screenshots';
  }
}

uploadBtn.addEventListener('click', doUpload);
uploadAnywayBtn.addEventListener('click', doUpload);

addMoreBtn.addEventListener('click', () => {
  postUpload.classList.add('hidden');
  grid.innerHTML = '';
  uploadBtn.disabled = false;
  uploadBtn.textContent = 'Upload Screenshots';
  status.textContent = '';
  fileInput.click();
});

doneBtn.addEventListener('click', async () => {
  await fetch('/done', { method: 'POST' });
  document.querySelector('.container').innerHTML =
    '<h2 style="color:#30d158;margin-bottom:0.5rem">Screenshots uploaded</h2>' +
    '<p style="color:#888">You can close this tab and return to the terminal.</p>';
});

skipBtn.addEventListener('click', async () => {
  await fetch('/skip', { method: 'POST' });
  document.querySelector('.container').innerHTML =
    '<h2 style="color:#888;margin-bottom:0.5rem">Skipped</h2>' +
    '<p style="color:#888">You can close this tab.</p>';
});

function setStatus(msg, isError) {
  status.textContent = msg;
  status.className = 'status' + (isError ? ' error' : ' success');
}
</script>
</body>
</html>`
