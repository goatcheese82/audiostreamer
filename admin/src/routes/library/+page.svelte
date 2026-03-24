<script>
    import { api } from '$lib/api.js';
    import { onMount } from 'svelte';

    let books = $state([]);
    let loading = $state(true);
    let scanning = $state(false);
    let importing = $state(false);
    let message = $state('');
    let search = $state('');

    let filtered = $derived(
        search
            ? books.filter(b =>
                b.title.toLowerCase().includes(search.toLowerCase()) ||
                b.author.toLowerCase().includes(search.toLowerCase())
            )
            : books
    );

    onMount(loadBooks);

    async function loadBooks() {
        loading = true;
        try {
            books = await api.listBooks();
        } catch (e) {
            message = `Error: ${e.message}`;
        } finally {
            loading = false;
        }
    }

    async function scanBooks() {
        scanning = true;
        message = '';
        try {
            const result = await api.scanBooks();
            message = `Scan complete: ${result.found} found, ${result.created} new, ${result.skipped} skipped`;
            await loadBooks();
        } catch (e) {
            message = `Scan error: ${e.message}`;
        } finally {
            scanning = false;
        }
    }

    async function importFromABS() {
        importing = true;
        message = '';
        try {
            const result = await api.importFromABS();
            message = `Import complete: ${result.imported} imported, ${result.skipped} skipped`;
            await loadBooks();
        } catch (e) {
            message = `Import error: ${e.message}`;
        } finally {
            importing = false;
        }
    }

    async function deleteBook(id, title) {
        if (!confirm(`Delete "${title}"? This will also remove any tag assignments.`)) return;
        try {
            await api.deleteBook(id);
            books = books.filter(b => b.id !== id);
            message = `Deleted "${title}"`;
        } catch (e) {
            message = `Error: ${e.message}`;
        }
    }

    function formatDuration(sec) {
        if (!sec) return '—';
        const h = Math.floor(sec / 3600);
        const m = Math.floor((sec % 3600) / 60);
        if (h > 0) return `${h}h ${m}m`;
        return `${m}m`;
    }
</script>

<div class="header">
    <h1>Library</h1>
    <div class="actions">
        <button class="btn-primary" onclick={scanBooks} disabled={scanning}>
            {scanning ? 'Scanning...' : 'Scan Directory'}
        </button>
        <button class="btn-ghost" onclick={importFromABS} disabled={importing}>
            {importing ? 'Importing...' : 'Import from ABS'}
        </button>
    </div>
</div>

{#if message}
    <div class="card" style="margin-bottom: 16px; padding: 12px 16px; font-size: 14px;">
        {message}
    </div>
{/if}

<div style="margin-bottom: 16px;">
    <input type="text" placeholder="Search books..." bind:value={search} />
</div>

{#if loading}
    <p style="color: var(--text-muted)">Loading library...</p>
{:else if filtered.length === 0}
    <div class="card" style="text-align: center; padding: 40px;">
        <p style="color: var(--text-muted);">
            {search ? 'No books match your search.' : 'No books in library. Click "Scan Directory" to find audiobooks.'}
        </p>
    </div>
{:else}
    <div class="book-list">
        {#each filtered as book}
            <div class="card book-card">
                <div class="book-info">
                    <div class="book-title">{book.title}</div>
                    <div class="book-meta">
                        {#if book.author}<span>{book.author}</span>{/if}
                        {#if book.narrator}<span> · Read by {book.narrator}</span>{/if}
                    </div>
                    <div class="book-details">
                        <span class="badge badge-accent">{book.file_paths?.length || 0} files</span>
                        <span class="badge badge-success">{formatDuration(book.duration_sec)}</span>
                    </div>
                </div>
                <div class="book-actions">
                    <button class="btn-danger" onclick={() => deleteBook(book.id, book.title)}>Delete</button>
                </div>
            </div>
        {/each}
    </div>
{/if}

<style>
    .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
    h1 { font-size: 24px; }
    .actions { display: flex; gap: 8px; }

    .book-list { display: flex; flex-direction: column; gap: 8px; }
    .book-card { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; }
    .book-title { font-weight: 600; font-size: 15px; }
    .book-meta { color: var(--text-muted); font-size: 13px; margin-top: 2px; }
    .book-details { display: flex; gap: 8px; margin-top: 8px; }
    .book-actions { flex-shrink: 0; margin-left: 16px; }
</style>
