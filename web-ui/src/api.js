// @ts-check
// src/api.js

/**
 * @typedef {Object} Note
 * @property {string} id - The note ID
 * @property {string} title - The note title
 * @property {string} body - The note body
 * @property {string} color - The note color (hex code)
 * @property {string} created_at - ISO date string when note was created
 * @property {string} updated_at - ISO date string when note was last updated
 */

import { setState, state } from './store.js';

// @ts-ignore - this is the way to get the api url from the env file or pass it as ENV variable
const VITE_API_URL = import.meta.env.VITE_API_URL;
// @ts-ignore - same as above
const VITE_API_SAME_ORIGIN = !!import.meta.env.VITE_API_SAME_ORIGIN;

/* ---------- Constants ---------- */
// deno-lint-ignore no-window
const BASE_HOST = VITE_API_SAME_ORIGIN ? (window.location.origin) : (VITE_API_URL || 'http://localhost:8080');
const API_BASE = BASE_HOST + '/api/v1';
const WS_URL = (BASE_HOST.startsWith('https') ? 'wss://' : 'ws://') +
	BASE_HOST.replace(/^https?:\/\//, '') +
	'/ws/notes/stream';

/* ---------- Helper utilities ---------- */

// URL-safe base64 decoder
/**
 * @param {string} str
 * @returns {string | null}
 */
const decodeBase64Url = (str) => {
	try {
		// Convert URL-safe chars before atob
		const base64 = str.replace(/-/g, '+').replace(/_/g, '/');
		// Add padding if needed
		const padded = base64.padEnd(
			base64.length + (4 - base64.length % 4) % 4,
			'=',
		);
		return atob(padded);
	} catch {
		console.warn('Token decoding failed - invalid token format');
		return null; // return null on error
	}
};

/**
 * @param {string} tok
 * @returns {number}
 */
const decodeTokenExp = (tok) => {
	try {
		const payload = decodeBase64Url(tok.split('.')[1]);
		if (!payload) {
			return 0;
		}
		return JSON.parse(payload).exp * 1000;
	} catch {
		console.warn('Token parsing failed');
		return 0;
	}
};

// Safe storage operations
/**
 * @param {Storage} storage
 * @param {string} key
 * @param {string} value
 */
const safeSetItem = (storage, key, value) => {
	try {
		storage.setItem(key, value);
	} catch (e) {
		console.warn(`Failed to set ${key} in storage:`, e);
	}
};

/**
 * @param {Storage} storage
 * @param {string} key
 */
const safeRemoveItem = (storage, key) => {
	try {
		storage.removeItem(key);
	} catch (e) {
		console.warn(`Failed to remove ${key} from storage:`, e);
	}
};

/* ---------- Token Management ---------- */

export const initializeTokens = () => {
	const accessToken = sessionStorage.getItem('notePulseAccess') || null;
	const refreshToken = localStorage.getItem('notePulseRefresh') || null;
	const accessExp = accessToken ? decodeTokenExp(accessToken) : 0;

	// Calculate dynamic refresh threshold on startup
	let ACCESS_THRESHOLD_SEC = 300; // fallback: 5 minutes
	if (accessToken && accessExp > 0) {
		const tokenTTLsec = Math.max(0, (accessExp - Date.now()) / 1000);
		const MIN_THRESHOLD = 30;
		ACCESS_THRESHOLD_SEC = Math.max(
			MIN_THRESHOLD,
			Math.floor(tokenTTLsec * 0.5),
		);
	}

	setState({
		accessToken,
		refreshToken,
		accessExp,
		ACCESS_THRESHOLD_SEC,
	});
};

/**
 * @param {string} acc
 * @param {string} ref
 */
export const setTokens = (acc, ref) => {
	const accessExp = decodeTokenExp(acc);
	safeSetItem(sessionStorage, 'notePulseAccess', acc);

	// Task 4: Clamp token refresh threshold
	const tokenTTLsec = Math.max(0, (accessExp - Date.now()) / 1000);
	const MIN_THRESHOLD = 30; // 30 s
	const ACCESS_THRESHOLD_SEC = Math.max(
		MIN_THRESHOLD,
		Math.floor(tokenTTLsec * 0.5),
	);

	const updates = {
		accessToken: acc,
		accessExp,
		ACCESS_THRESHOLD_SEC,
	};

	if (ref) {
		/** @type {any} */ (updates).refreshToken = ref;
		safeSetItem(localStorage, 'notePulseRefresh', ref);
	}

	setState(updates);
};

export const clearTokens = () => {
	safeRemoveItem(sessionStorage, 'notePulseAccess');
	safeRemoveItem(localStorage, 'notePulseRefresh');
	setState({
		accessToken: null,
		refreshToken: null,
		accessExp: 0,
	});
};

/* ---------- Token Refresh Logic ---------- */

export const ensureValidToken = async () => {
	if (!state.accessToken) return;
	if (Date.now() + state.ACCESS_THRESHOLD_SEC * 1000 < state.accessExp) return;
	await refreshTokenFlow();
};

const refreshTokenFlow = () => {
	if (Date.now() < state.refreshBlockedUntil) {
		throw new Error('refresh back-off');
	}
	if (state.refreshingPromise) return state.refreshingPromise;
	if (!state.refreshToken) throw new Error('no refresh token');

	const refreshingPromise = (async () => {
		const controller = new AbortController();
		const timeoutId = setTimeout(() => controller.abort(), 10000); // 10 second timeout

		try {
			const res = await fetch(API_BASE + '/auth/refresh', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ refresh_token: state.refreshToken }),
				signal: controller.signal,
			});

			if (res.status === 401) {
				// Refresh token is invalid/expired - logout user
				logoutLocal();
				location.hash = '#/sign-in';
				throw new Error('refresh token expired');
			}

			if (!res.ok) throw new Error('refresh failed');
			const d = await res.json();
			setTokens(d.token, d.refresh_token);
			setState({
				refreshFailures: 0,
				refreshBlockedUntil: 0,
				refreshingPromise: null,
			});
		} finally {
			clearTimeout(timeoutId);
			setState({ refreshingPromise: null });
		}
	})().catch((err) => {
		// If it's a 401 error, we already handled it above, just reset the promise
		if (err.message === 'refresh token expired') {
			throw err;
		}

		const now = Date.now();
		if (now - state.lastFailure > 300000) {
			setState({ refreshFailures: 0 }); // cool-down
		}
		const refreshFailures = state.refreshFailures + 1;
		const delay = Math.min(1000 * (2 ** (refreshFailures - 1)), 30000);
		setState({
			refreshFailures,
			lastFailure: now,
			refreshBlockedUntil: Date.now() + delay,
		});
		throw err;
	});

	setState({ refreshingPromise });
	return refreshingPromise;
};

/* ---------- API Wrapper ---------- */

/**
 * @param {string} path
 * @param {RequestInit} opts
 * @param {boolean} retried
 * @returns {Promise<any>}
 */
export const apiFetch = async (path, opts = {}, retried = false) => {
	await ensureValidToken().catch(() => {
		// Token validation failed, continuing with current token
	});

	// Create fresh AbortController per call
	const ctrl = new AbortController();

	const cfg = {
		headers: {
			'Content-Type': 'application/json',
			...(/** @type {any} */ (opts).headers || {}),
		},
		...opts,
	};
	if (state.accessToken) {
		cfg.headers.Authorization = 'Bearer ' + state.accessToken;
	}

	// Set fresh signal
	/** @type {any} */ (cfg).signal = ctrl.signal;

	const res = await fetch(API_BASE + path, cfg);

	if (res.status === 401 && !retried) {
		try {
			await refreshTokenFlow();
			cfg.headers.Authorization = 'Bearer ' + state.accessToken;
			return apiFetch(path, cfg, true);
		} catch {
			// Refresh token failed, force logout
			logoutLocal();
			location.hash = '#/sign-in';
			throw new Error('Session expired');
		}
	}

	if (!res.ok) {
		let err = 'Error ' + res.status;
		try {
			const d = await res.json();
			if (d.error) err = d.error;
		} catch {
			console.error('Error parsing JSON response:', res);
		}
		throw new Error(err);
	}
	return res.status === 204 ? null : res.json();
};

/* ---------- API Endpoints ---------- */

/* ---------- Constants ---------- */

export const api = {
	/**
	 * @param {string} e
	 * @param {string} p
	 */
	signUp: (e, p) =>
		apiFetch('/auth/sign-up', {
			method: 'POST',
			body: JSON.stringify({ email: e, password: p }),
		}),
	/**
	 * @param {string} e
	 * @param {string} p
	 */
	signIn: (e, p) =>
		apiFetch('/auth/sign-in', {
			method: 'POST',
			body: JSON.stringify({ email: e, password: p }),
		}),
	signOut: () =>
		apiFetch('/auth/sign-out', {
			method: 'POST',
			body: JSON.stringify({ refresh_token: state.refreshToken }),
		}),
	/**
	 * @param {object} params
	 */
	listNotes: (params = {}) => {
		/*eslint-disable*/
		const query = new URLSearchParams();
		if (params.offset !== undefined) query.set('offset', params.offset);
		if (params.limit !== undefined) query.set('limit', params.limit);
		if (params.q) query.set('q', params.q);
		if (params.sort) query.set('sort', params.sort);
		if (params.order) query.set('order', params.order);
		return apiFetch('/notes' + (query.toString() ? `?${query}` : ''));
	},

	createNote: (n) => apiFetch('/notes', { method: 'POST', body: JSON.stringify(n) }),

	updateNote: (i, f) => apiFetch(`/notes/${i}`, { method: 'PATCH', body: JSON.stringify(f) }),

	deleteNote: (i) => apiFetch(`/notes/${i}`, { method: 'DELETE' }),
	me: () => apiFetch('/me'),
};

/* ---------- WebSocket Management ---------- */

/**
 * @param {(this: WebSocket, ev: MessageEvent) => any} onMessage
 */
export const connectSocket = async (onMessage) => {
	if (
		state.ws &&
		(/** @type {WebSocket} */ (state.ws).readyState ===
				WebSocket.OPEN || /** @type {WebSocket} */
			(state.ws).readyState === WebSocket.CONNECTING)
	) return;

	if (Date.now() < state.refreshBlockedUntil) {
		setTimeout(
			() => connectSocket(onMessage),
			state.refreshBlockedUntil - Date.now() + 100,
		);
		return;
	}
	await ensureValidToken().catch(() => {
		// Token validation failed, continuing with WebSocket connection attempt
	});
	if (!state.accessToken) return;

	if (state.ws) {
		/** @type {WebSocket} */ (state.ws).onclose = null;
		/** @type {WebSocket} */ (state.ws).onmessage = null;
		/** @type {WebSocket} */ (state.ws).onerror = null;
		/** @type {WebSocket} */ (state.ws).close();
	}

	const ws = new WebSocket(`${WS_URL}?token=${state.accessToken}`);

	// Clear the pending reconnect timer once a socket opens
	if (state.reconnectTimerId) {
		clearTimeout(state.reconnectTimerId);
		setState({ reconnectTimerId: null, backoffDelay: 0 });
	}

	// Task 3: Enable "Create note" only after WebSocket is open
	ws.onopen = () => {
		setState({ ws });
	};

	ws.onmessage = onMessage;
	ws.onerror = () => backoffRetry(() => connectSocket(onMessage));
	ws.onclose = async (evt) => {
		setState({ ws: null });

		if ([1008, 1006, 4001].includes(evt.code)) {
			try {
				await refreshTokenFlow();
			} catch {
				// Refresh token failed, but we'll retry connection anyway
			}
			setTimeout(() => connectSocket(onMessage), 500);
		} else {
			backoffRetry(() => connectSocket(onMessage));
		}
	};

	setState({ ws });
};

/**
 * @param {function} fn
 */
const backoffRetry = (fn) => {
	const backoffDelay = state.backoffDelay ? Math.min(state.backoffDelay * 2, 30000) : 1000;
	// Add random jitter (±10%) to prevent thundering herd
	const jitter = backoffDelay * 0.1 * (2 * Math.random() - 1);
	const delay = Math.max(100, backoffDelay + jitter);
	const reconnectTimerId = setTimeout(fn, delay);
	setState({ backoffDelay, reconnectTimerId });
};

export const disconnectSocket = () => {
	if (state.reconnectTimerId) {
		clearTimeout(state.reconnectTimerId);
	}

	if (state.ws) {
		/** @type {WebSocket} */ (state.ws).onclose = null;
		/** @type {WebSocket} */ (state.ws).onmessage = null;
		/** @type {WebSocket} */ (state.ws).onerror = null;
		/** @type {WebSocket} */ (state.ws).close();
	}

	setState({
		ws: null,
		reconnectTimerId: null,
		backoffDelay: 0,
	});
};

export const logoutLocal = () => {
	clearTokens();
	disconnectSocket();
};
