const API_URL = import.meta.env.PUBLIC_API_URL || 'http://10.0.2.166:8080';
const ADMIN_TOKEN = import.meta.env.PUBLIC_ADMIN_TOKEN || '';

async function request(method, path, body = null) {
    const headers = { 'Content-Type': 'application/json' };
    if (ADMIN_TOKEN) {
        headers['Authorization'] = `Bearer ${ADMIN_TOKEN}`;
    }

    const opts = { method, headers };
    if (body) opts.body = JSON.stringify(body);

    const res = await fetch(`${API_URL}${path}`, opts);
    if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || res.statusText);
    }
    return res.json();
}

export const api = {
    // Books
    listBooks: () => request('GET', '/api/books'),
    getBook: (id) => request('GET', `/api/books/${id}`),
    createBook: (book) => request('POST', '/api/books', book),
    updateBook: (id, book) => request('PUT', `/api/books/${id}`, book),
    deleteBook: (id) => request('DELETE', `/api/books/${id}`),
    scanBooks: () => request('POST', '/api/books/scan'),
    importFromABS: () => request('POST', '/api/books/import'),

    // Tags
    listTags: () => request('GET', '/api/tags'),
    createTag: (tag) => request('POST', '/api/tags', tag),
    deleteTag: (uid) => request('DELETE', `/api/tags/${uid}`),

    // Devices
    listDevices: () => request('GET', '/api/devices'),

    // Accounts
    listAccounts: () => request('GET', '/api/accounts'),
    createAccount: (account) => request('POST', '/api/accounts', account),
    deleteAccount: (id) => request('DELETE', `/api/accounts/${id}`),

    // Book access
    grantAccess: (accountId, bookId) => request('POST', '/api/access', { account_id: accountId, book_id: bookId }),
    revokeAccess: (accountId, bookId) => request('DELETE', '/api/access', { account_id: accountId, book_id: bookId }),
    grantAllAccess: (accountId) => request('POST', '/api/access/all', { account_id: accountId }),
    listBookAccess: (bookId) => request('GET', `/api/access/book/${bookId}`),
    listAccountBooks: (accountId) => request('GET', `/api/access/account/${accountId}`),

    // Playback info
    getBookInfo: (nfcId) => request('GET', `/api/book/${nfcId}`),
};
