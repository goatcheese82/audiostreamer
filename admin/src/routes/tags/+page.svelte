<script>
    import { api } from '$lib/api.js';
    import { onMount } from 'svelte';

    let tags = $state([]);
    let books = $state([]);
    let loading = $state(true);
    let message = $state('');

    // New tag form
    let newTagUID = $state('');
    let newBookID = $state('');
    let newLabel = $state('');

    onMount(async () => {
        try {
            [tags, books] = await Promise.all([
                api.listTags(),
                api.listBooks(),
            ]);
        } catch (e) {
            message = `Error: ${e.message}`;
        } finally {
            loading = false;
        }
    });

    async function createTag() {
        if (!newTagUID || !newBookID) {
            message = 'Tag UID and book are required';
            return;
        }

        try {
            await api.createTag({
                tag_uid: newTagUID.replace(/[:\s-]/g, '').toUpperCase(),
                book_id: newBookID,
                label: newLabel,
            });
            message = 'Tag assigned successfully';
            newTagUID = '';
            newBookID = '';
            newLabel = '';
            tags = await api.listTags();
        } catch (e) {
            message = `Error: ${e.message}`;
        }
    }

    async function deleteTag(uid) {
        if (!confirm(`Remove tag ${uid}?`)) return;
        try {
            await api.deleteTag(uid);
            tags = tags.filter(t => t.tag_uid !== uid);
            message = 'Tag removed';
        } catch (e) {
            message = `Error: ${e.message}`;
        }
    }
</script>

<h1>NFC Tags</h1>

{#if message}
    <div class="card" style="margin: 16px 0; padding: 12px 16px; font-size: 14px;">
        {message}
    </div>
{/if}

<div class="card" style="margin-bottom: 24px;">
    <h2>Assign Tag</h2>
    <div class="form">
        <div class="field">
            <label for="tag-uid">Tag UID</label>
            <input id="tag-uid" type="text" placeholder="e.g. 04A32B1C5E8000" bind:value={newTagUID} />
        </div>
        <div class="field">
            <label for="book-select">Book</label>
            <select id="book-select" bind:value={newBookID}>
                <option value="">Select a book...</option>
                {#each books as book}
                    <option value={book.id}>{book.title}{book.author ? ` — ${book.author}` : ''}</option>
                {/each}
            </select>
        </div>
        <div class="field">
            <label for="label">Label (optional)</label>
            <input id="label" type="text" placeholder="e.g. Blue tag - kitchen" bind:value={newLabel} />
        </div>
        <button class="btn-primary" onclick={createTag}>Assign Tag</button>
    </div>
</div>

{#if loading}
    <p style="color: var(--text-muted)">Loading tags...</p>
{:else if tags.length === 0}
    <div class="card" style="text-align: center; padding: 40px;">
        <p style="color: var(--text-muted);">No tags assigned yet. Use the form above or scan a tag with your ESP32.</p>
    </div>
{:else}
    <div class="tag-list">
        {#each tags as tag}
            <div class="card tag-card">
                <div class="tag-info">
                    <div class="tag-uid"><code>{tag.tag_uid}</code></div>
                    {#if tag.label}<div class="tag-label">{tag.label}</div>{/if}
                    <div class="tag-book">
                        📚 {tag.book_title || 'Unknown book'}
                        {#if tag.book_author}<span style="color: var(--text-muted)"> — {tag.book_author}</span>{/if}
                    </div>
                </div>
                <button class="btn-danger" onclick={() => deleteTag(tag.tag_uid)}>Remove</button>
            </div>
        {/each}
    </div>
{/if}

<style>
    h1 { font-size: 24px; margin-bottom: 24px; }
    h2 { font-size: 16px; margin-bottom: 16px; color: var(--text-muted); }

    .form { display: flex; flex-direction: column; gap: 12px; }
    .field { display: flex; flex-direction: column; gap: 4px; }
    .field label { font-size: 12px; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; }

    .tag-list { display: flex; flex-direction: column; gap: 8px; }
    .tag-card { display: flex; justify-content: space-between; align-items: center; padding: 16px 20px; }
    .tag-uid { font-size: 14px; }
    .tag-label { font-size: 13px; color: var(--text-muted); }
    .tag-book { font-size: 14px; margin-top: 4px; }
    code { font-size: 13px; background: var(--bg); padding: 2px 8px; border-radius: 4px; }
</style>
