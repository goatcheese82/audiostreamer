<script>
    const API_URL = import.meta.env.PUBLIC_API_URL || 'http://10.0.2.166:8080';
</script>

<h1>Settings</h1>

<div class="card" style="margin-bottom: 16px;">
    <h2>Server Info</h2>
    <div class="info-row">
        <span class="info-label">API URL</span>
        <code>{API_URL}</code>
    </div>
    <div class="info-row">
        <span class="info-label">Health Check</span>
        <a href="{API_URL}/health" target="_blank">{API_URL}/health</a>
    </div>
</div>

<div class="card" style="margin-bottom: 16px;">
    <h2>ESP32 Configuration</h2>
    <p style="color: var(--text-muted); font-size: 14px; margin-bottom: 12px;">
        Set these values in your ESP32 firmware's <code>config.h</code>:
    </p>
    <pre><code>#define SERVER_HOST  "10.0.2.166"
#define SERVER_PORT  8080
#define WIFI_SSID    "your-wifi-ssid"
#define WIFI_PASS    "your-wifi-password"</code></pre>
</div>

<div class="card">
    <h2>API Quick Test</h2>
    <p style="color: var(--text-muted); font-size: 14px; margin-bottom: 12px;">
        Test streaming with mpv or curl:
    </p>
    <pre><code># Stream a book by NFC tag UID
mpv http://10.0.2.166:8080/api/play/YOUR_TAG_UID

# Check server health
curl http://10.0.2.166:8080/health</code></pre>
</div>

<style>
    h1 { font-size: 24px; margin-bottom: 24px; }
    h2 { font-size: 16px; margin-bottom: 12px; color: var(--text-muted); }

    .info-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 8px 0;
        border-bottom: 1px solid var(--border);
        font-size: 14px;
    }
    .info-row:last-child { border-bottom: none; }
    .info-label { color: var(--text-muted); }

    pre {
        background: var(--bg);
        border: 1px solid var(--border);
        border-radius: var(--radius);
        padding: 12px 16px;
        overflow-x: auto;
        font-size: 13px;
    }
    code { font-size: 13px; }
</style>
