<script>
    import { api } from '$lib/api.js';
    import { onMount } from 'svelte';

    let books = $state([]);
    let tags = $state([]);
    let devices = $state([]);
    let loading = $state(true);

    onMount(async () => {
        try {
            [books, tags, devices] = await Promise.all([
                api.listBooks(),
                api.listTags(),
                api.listDevices(),
            ]);
        } catch (e) {
            console.error('Failed to load dashboard:', e);
        } finally {
            loading = false;
        }
    });
</script>

<h1>Dashboard</h1>

{#if loading}
    <p style="color: var(--text-muted)">Loading...</p>
{:else}
    <div class="stats">
        <div class="card stat">
            <div class="stat-value">{books.length}</div>
            <div class="stat-label">Books</div>
        </div>
        <div class="card stat">
            <div class="stat-value">{tags.length}</div>
            <div class="stat-label">Tags Assigned</div>
        </div>
        <div class="card stat">
            <div class="stat-value">{devices.length}</div>
            <div class="stat-label">Devices</div>
        </div>
        <div class="card stat">
            <div class="stat-value">{tags.length > 0 ? Math.round(tags.length / books.length * 100) : 0}%</div>
            <div class="stat-label">Books Tagged</div>
        </div>
    </div>

    {#if devices.length > 0}
        <h2>Devices</h2>
        <div class="card" style="margin-top: 12px;">
            <table>
                <thead>
                    <tr>
                        <th>Device ID</th>
                        <th>Name</th>
                        <th>Last Seen</th>
                        <th>Firmware</th>
                    </tr>
                </thead>
                <tbody>
                    {#each devices as device}
                        <tr>
                            <td><code>{device.device_id}</code></td>
                            <td>{device.name || '—'}</td>
                            <td>{device.last_seen ? new Date(device.last_seen).toLocaleString() : 'Never'}</td>
                            <td>{device.firmware_ver || '—'}</td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        </div>
    {/if}

    {#if books.length === 0}
        <div class="card" style="margin-top: 24px; text-align: center; padding: 40px;">
            <p style="font-size: 18px; margin-bottom: 12px;">No books yet</p>
            <p style="color: var(--text-muted); margin-bottom: 20px;">Scan your audiobook directory or import from Audiobookshelf to get started.</p>
            <a href="/library" class="btn-primary" style="display: inline-block; padding: 10px 24px;">Go to Library</a>
        </div>
    {/if}
{/if}

<style>
    h1 { margin-bottom: 24px; font-size: 24px; }
    h2 { margin-top: 32px; font-size: 18px; }

    .stats {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
        gap: 16px;
    }

    .stat { text-align: center; padding: 24px; }
    .stat-value { font-size: 36px; font-weight: 700; color: var(--accent); }
    .stat-label { font-size: 14px; color: var(--text-muted); margin-top: 4px; }

    table { width: 100%; border-collapse: collapse; }
    th { text-align: left; color: var(--text-muted); font-size: 12px; text-transform: uppercase; letter-spacing: 0.05em; padding: 8px 12px; border-bottom: 1px solid var(--border); }
    td { padding: 10px 12px; border-bottom: 1px solid var(--border); font-size: 14px; }
    code { font-size: 12px; background: var(--bg); padding: 2px 6px; border-radius: 4px; }
</style>
