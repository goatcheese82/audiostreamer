<script>
    import '../app.css';

    let { children } = $props();

    const nav = [
        { href: '/', label: 'Dashboard', icon: '⌂' },
        { href: '/library', label: 'Library', icon: '📚' },
        { href: '/tags', label: 'Tags', icon: '🏷' },
        { href: '/devices', label: 'Devices', icon: '📡' },
        { href: '/settings', label: 'Settings', icon: '⚙' },
    ];
</script>

<div class="layout">
    <aside class="sidebar">
        <div class="logo">
            <span class="logo-icon">🎧</span>
            <span class="logo-text">Audiostreamer</span>
        </div>
        <nav>
            {#each nav as item}
                <a href={item.href} class="nav-item">
                    <span class="nav-icon">{item.icon}</span>
                    <span>{item.label}</span>
                </a>
            {/each}
        </nav>
    </aside>
    <main class="content">
        {@render children()}
    </main>
</div>

<style>
    .layout {
        display: flex;
        min-height: 100vh;
    }

    .sidebar {
        width: 220px;
        background: var(--bg-card);
        border-right: 1px solid var(--border);
        padding: 20px 0;
        flex-shrink: 0;
    }

    .logo {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 0 20px 24px;
        border-bottom: 1px solid var(--border);
        margin-bottom: 16px;
    }

    .logo-icon { font-size: 24px; }
    .logo-text { font-size: 16px; font-weight: 600; }

    nav { display: flex; flex-direction: column; gap: 2px; padding: 0 8px; }

    .nav-item {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 10px 12px;
        border-radius: var(--radius);
        color: var(--text-muted);
        font-size: 14px;
        transition: all 0.15s;
    }
    .nav-item:hover {
        background: var(--bg-hover);
        color: var(--text);
    }

    .nav-icon { font-size: 16px; width: 20px; text-align: center; }

    .content {
        flex: 1;
        padding: 32px;
        max-width: 1200px;
    }
</style>
