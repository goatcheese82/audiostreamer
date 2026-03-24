<script>
    import { api } from '$lib/api.js';
    import { onMount } from 'svelte';

    let devices = $state([]);
    let loading = $state(true);

    onMount(async () => {
        try {
            devices = await api.listDevices();
        } catch (e) {
            console.error('Failed to load devices:', e);
        } finally {
            loading = false;
        }
    });

    function timeAgo(dateStr) {
        if (!dateStr) return 'Never';
        const diff = Date.now() - new Date(dateStr).getTime();
        const mins = Math.floor(diff / 60000);
        if (mins < 1) return 'Just now';
        if (mins < 60) return `${mins}m ago`;
        const hours = Math.floor(mins / 60);
        if (hours < 24) return `${hours}h ago`;
        const days = Math.floor(hours / 24);
        return `${days}d ago`;
    }
</script>

<h1>Devices</h1>

{#if loading}
    <p style="color: var(--text-muted)">Loading devices...</p>
{:else if devices.length === 0}
    <div class="card" style="text-align: center; padding: 40px;">
        <p style="color: var(--text-muted);">No devices registered yet. Devices appear here when an ESP32 makes its first API call.</p>
    </div>
{:else}
    <div class="device-list">
        {#each devices as device}
            <div class="card device-card">
                <div class="device-info">
                    <div class="device-id"><code>{device.device_id}</code></div>
                    <div class="device-name">{device.name || 'Unnamed'}</div>
                </div>
                <div class="device-meta">
                    {#if device.firmware_ver}
                        <span class="badge badge-accent">v{device.firmware_ver}</span>
                    {/if}
                    <span class="last-seen">{timeAgo(device.last_seen)}</span>
                </div>
            </div>
        {/each}
    </div>
{/if}

<style>
    h1 { font-size: 24px; margin-bottom: 24px; }

    .device-list { display: flex; flex-direction: column; gap: 8px; }
    .device-card { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; }
    .device-id { font-size: 14px; }
    .device-name { color: var(--text-muted); font-size: 13px; }
    .device-meta { display: flex; align-items: center; gap: 12px; }
    .last-seen { font-size: 13px; color: var(--text-muted); }
    code { font-size: 13px; background: var(--bg); padding: 2px 8px; border-radius: 4px; }
</style>
