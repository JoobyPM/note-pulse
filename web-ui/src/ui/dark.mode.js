import { toast } from '../ui.js';
import { $ } from '../utils/dom.js';
// Dark mode functions
export const toggleDarkMode = () => {
	// Get current state from localStorage since it's managed globally
	let isDarkMode = false;
	try {
		isDarkMode = localStorage.getItem('notePulseDarkMode') === 'true';
	} catch (e) {
		console.warn('Failed to read dark mode preference:', e);
	}

	// Toggle the state
	isDarkMode = !isDarkMode;

	try {
		localStorage.setItem('notePulseDarkMode', isDarkMode.toString());
	} catch (e) {
		console.warn('Failed to save dark mode preference:', e);
	}

	// Apply the new state
	if (isDarkMode) {
		document.documentElement.classList.add('dark');
	} else {
		document.documentElement.classList.remove('dark');
	}

	// Update aria-pressed attribute
	const darkToggle = $('#dark-toggle');
	if (darkToggle) {
		darkToggle.setAttribute('aria-pressed', isDarkMode.toString());
	}

	// Show toast for dark mode toggle
	if (isDarkMode) {
		toast('🌙 Dark mode on', 'success');
	} else {
		toast('☀️ Light mode on', 'success');
	}
};

export const updateDarkMode = () => {
	// Read current state from localStorage since it's managed globally
	let isDarkMode = false;
	try {
		isDarkMode = localStorage.getItem('notePulseDarkMode') === 'true';
	} catch (e) {
		console.warn('Failed to read dark mode preference:', e);
	}

	// Update aria-pressed attribute if button exists
	const darkToggle = $('#dark-toggle');
	if (darkToggle) {
		darkToggle.setAttribute('aria-pressed', isDarkMode.toString());
	}
};
