// @ts-check
// src/main.js
import './generated/main.css';
import { api, ensureValidToken, initializeTokens, logoutLocal } from './api.js';
import { initializeToast } from './ui.js';
import { bootstrapRouter } from './router.js';

// Global dark mode initialization - runs before any view renders
const initializeDarkMode = () => {
	try {
		const saved = localStorage.getItem('notePulseDarkMode');
		const isDarkMode = saved === 'true';
		if (isDarkMode) {
			document.documentElement.classList.add('dark');
		} else {
			document.documentElement.classList.remove('dark');
		}
	} catch (e) {
		console.warn('Failed to read dark mode preference:', e);
	}
};

(async () => {
	// Initialize dark mode globally before any UI renders
	initializeDarkMode();

	// Initialize toast system when Notyf is available
	initializeToast();

	// Initialize tokens from storage
	initializeTokens();

	// Ensure token is valid if present, but don't block startup
	await ensureValidToken().catch(() => {
		// Token validation failed, but we'll let the API call handle logout if needed
	});

	// Validate token with server if we have one
	if (sessionStorage.getItem('notePulseAccess')) {
		api.me().catch(() => logoutLocal());
	}

	// Start the router
	bootstrapRouter();
})();
