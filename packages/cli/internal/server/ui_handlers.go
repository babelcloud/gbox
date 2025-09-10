package server

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// handleLiveViewHTML serves the live-view.html file
func (s *GBoxServer) handleLiveViewHTML(w http.ResponseWriter, r *http.Request) {
	// Try to find and serve the live-view static files
	liveViewPath := s.findLiveViewStaticPath()
	if liveViewPath != "" {
		htmlFile := filepath.Join(liveViewPath, "index.html")
		if _, err := os.Stat(htmlFile); err == nil {
			// Read and serve the file with correct MIME type
			file, err := os.Open(htmlFile)
			if err == nil {
				defer file.Close()
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				io.Copy(w, file)
				return
			}
		}
	}
	
	// If live-view is not built, redirect to /live-view
	http.Redirect(w, r, "/live-view", http.StatusTemporaryRedirect)
}

// handleLiveView serves the live-view application
func (s *GBoxServer) handleLiveView(w http.ResponseWriter, r *http.Request) {
	// Try to find and serve the live-view static files
	liveViewPath := s.findLiveViewStaticPath()
	if liveViewPath != "" {
		htmlFile := filepath.Join(liveViewPath, "index.html")
		if _, err := os.Stat(htmlFile); err == nil {
			// Read and serve the file with correct MIME type
			file, err := os.Open(htmlFile)
			if err == nil {
				defer file.Close()
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				io.Copy(w, file)
				return
			}
		}
	}
	
	// If live-view is not built, show a placeholder
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Live View - GBox</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #0a0a0a;
            color: #fff;
            height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            padding: 2rem;
        }
        .container {
            max-width: 600px;
            text-align: center;
        }
        h1 {
            font-size: 2.5rem;
            margin-bottom: 1rem;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .message {
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 12px;
            padding: 2rem;
            margin: 2rem 0;
        }
        code {
            display: block;
            margin: 1rem 0;
            padding: 1rem;
            background: #1a1a1a;
            border-radius: 8px;
            font-family: monospace;
            color: #4ade80;
        }
        .back-link {
            color: #667eea;
            text-decoration: none;
            margin-top: 2rem;
            display: inline-block;
        }
        .back-link:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>📱 Live View</h1>
        <div class="message">
            <p>The Live View interface is not yet built.</p>
            <p style="margin-top: 1rem;">To enable the full WebRTC streaming interface:</p>
            <code>cd packages/live-view && pnpm install && pnpm build:static</code>
            <p style="margin-top: 1rem; font-size: 0.9rem; color: #888;">
                After building, restart the server to load the interface.
            </p>
        </div>
        <a href="/" class="back-link">← Back to Home</a>
    </div>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}

// handleAdbExposeUI serves the ADB Expose management interface
func (s *GBoxServer) handleAdbExposeUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>ADB Expose - GBox</title>
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #0a0a0a;
            color: #fff;
            min-height: 100vh;
            padding: 2rem;
        }
        .header {
            max-width: 1200px;
            margin: 0 auto 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        h1 {
            font-size: 2rem;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        .add-forward {
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 2rem;
        }
        .form-row {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 1rem;
        }
        .form-group {
            display: flex;
            flex-direction: column;
        }
        label {
            color: #888;
            font-size: 0.875rem;
            margin-bottom: 0.5rem;
        }
        input, select {
            background: #1a1a1a;
            border: 1px solid #333;
            color: #fff;
            padding: 0.75rem;
            border-radius: 6px;
            font-size: 1rem;
        }
        button {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            border: none;
            padding: 0.75rem 2rem;
            border-radius: 6px;
            font-size: 1rem;
            cursor: pointer;
            transition: opacity 0.2s;
        }
        button:hover {
            opacity: 0.9;
        }
        .forwards-list {
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 12px;
            padding: 1.5rem;
        }
        .forward-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 1rem;
            background: #1a1a1a;
            border-radius: 8px;
            margin-bottom: 0.5rem;
        }
        .forward-info {
            display: flex;
            align-items: center;
            gap: 2rem;
        }
        .device-name {
            color: #667eea;
            font-weight: 500;
        }
        .port-mapping {
            color: #aaa;
            font-family: monospace;
        }
        .remove-btn {
            background: #ef4444;
            padding: 0.5rem 1rem;
            font-size: 0.875rem;
        }
        .empty-state {
            text-align: center;
            padding: 3rem;
            color: #666;
        }
        .back-link {
            color: #667eea;
            text-decoration: none;
        }
        .back-link:hover {
            text-decoration: underline;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>🔌 ADB Expose</h1>
        <a href="/" class="back-link">← Back to Home</a>
    </div>
    
    <div class="container">
        <div class="add-forward">
            <h2 style="margin-bottom: 1rem;">Add Port Forward</h2>
            <div class="form-row">
                <div class="form-group">
                    <label>Device</label>
                    <select id="deviceSelect">
                        <option value="">Default Device</option>
                    </select>
                </div>
                <div class="form-group">
                    <label>Local Port</label>
                    <input type="number" id="localPort" placeholder="8080" min="1" max="65535">
                </div>
                <div class="form-group">
                    <label>Remote Port</label>
                    <input type="number" id="remotePort" placeholder="8080" min="1" max="65535">
                </div>
                <div class="form-group">
                    <label>Protocol</label>
                    <select id="protocol">
                        <option value="tcp">TCP</option>
                        <option value="unix">Unix Socket</option>
                    </select>
                </div>
            </div>
            <button onclick="addForward()">Add Forward</button>
        </div>
        
        <div class="forwards-list">
            <h2 style="margin-bottom: 1rem;">Active Port Forwards</h2>
            <div id="forwardsList">
                <div class="empty-state">No active port forwards</div>
            </div>
        </div>
    </div>
    
    <script>
        // Load devices
        fetch('/api/devices')
            .then(res => res.json())
            .then(data => {
                const select = document.getElementById('deviceSelect');
                if (data.devices && data.devices.length > 0) {
                    data.devices.forEach(device => {
                        const option = document.createElement('option');
                        option.value = device.id;
                        option.textContent = device['ro.product.model'] || device.id;
                        select.appendChild(option);
                    });
                }
            });
        
        // Load forwards
        function loadForwards() {
            fetch('/api/adb-expose/list')
                .then(res => res.json())
                .then(data => {
                    const list = document.getElementById('forwardsList');
                    if (data.forwards && data.forwards.length > 0) {
                        list.innerHTML = data.forwards.map(forward => 
                            '<div class="forward-item">' +
                            '<div class="forward-info">' +
                            '<span class="device-name">' + forward.device_serial + '</span>' +
                            '<span class="port-mapping">' + forward.local + ' → ' + forward.remote + '</span>' +
                            '</div>' +
                            '<button class="remove-btn" onclick="removeForward(\'' + forward.device_serial + '\', ' + 
                            (forward.local_port || 0) + ', ' + (forward.remote_port || 0) + ')">' +
                            'Remove' +
                            '</button>' +
                            '</div>'
                        ).join('');
                    } else {
                        list.innerHTML = '<div class="empty-state">No active port forwards</div>';
                    }
                });
        }
        
        // Add forward
        function addForward() {
            const device = document.getElementById('deviceSelect').value;
            const localPort = document.getElementById('localPort').value;
            const remotePort = document.getElementById('remotePort').value;
            const protocol = document.getElementById('protocol').value;
            
            if (!localPort || !remotePort) {
                alert('Please enter both local and remote ports');
                return;
            }
            
            fetch('/api/adb-expose/start', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    device_serial: device,
                    local_port: parseInt(localPort),
                    remote_port: parseInt(remotePort),
                    protocol: protocol
                })
            })
            .then(res => res.json())
            .then(data => {
                if (data.success) {
                    loadForwards();
                    // Clear inputs
                    document.getElementById('localPort').value = '';
                    document.getElementById('remotePort').value = '';
                } else {
                    alert('Failed to add forward: ' + (data.error || 'Unknown error'));
                }
            });
        }
        
        // Remove forward
        function removeForward(device, localPort, remotePort) {
            fetch('/api/adb-expose/stop', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    device_serial: device,
                    local_port: localPort,
                    remote_port: remotePort
                })
            })
            .then(res => res.json())
            .then(data => {
                if (data.success) {
                    loadForwards();
                }
            });
        }
        
        // Initial load
        loadForwards();
        
        // Refresh every 5 seconds
        setInterval(loadForwards, 5000);
    </script>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, html)
}