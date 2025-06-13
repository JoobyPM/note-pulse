import { reInitialise, renderNoteList, toast, useEvent } from '../ui.js';
import { $ } from '../utils/dom.js';
import { debounce } from '../utils/timing.js';
import { resetPaging, setState, state } from '../store.js';
import { sortOrderIcon } from '../templates/index.js';

// Helper function to refresh notes with current filters
const refreshNotes = () => {
	setState({ isLoading: true });
	renderNoteList();

	try {
		reInitialise();
	} catch (error) {
		// Handle API errors based on status
		if (error.message.includes('400')) {
			toast('Invalid filter settings', 'error');
		} else if (error.message.includes('401') || error.message.includes('403')) {
			// Auth error - handled by API layer
		} else {
			toast('Error loading notes', 'error');
		}
	}
};

// Search and sort functionality
export const setupSearchAndSort = () => {
	const searchInput = $('#search-input');
	const sortButton = $('#sort-button');
	const sortDropdown = $('#sort-dropdown');
	const sortLabel = $('#sort-label');
	const sortOrder = $('#sort-order');

	if (!searchInput || !sortButton || !sortDropdown) return;

	const debouncedSearch = debounce((e) => {
		const newQuery = e.target.value.trim();
		if (newQuery !== state.q) {
			setState({ q: newQuery });
			resetPaging();
			refreshNotes();
		}
	}, 300);

	// Search input with debouncing
	useEvent(searchInput, 'input', debouncedSearch);

	// Clear search on Esc
	useEvent(searchInput, 'keydown', (e) => {
		if (e.key === 'Escape') {
			e.target.value = '';
			e.target.focus();
			if (state.q !== '') {
				setState({ q: '' });
				resetPaging();
				refreshNotes();
			}
		}
	});

	// Sort button dropdown toggle
	useEvent(sortButton, 'click', (e) => {
		e.stopPropagation();
		const isHidden = sortDropdown.classList.contains('hidden');
		sortDropdown.classList.toggle('hidden');
		sortButton.setAttribute('aria-expanded', isHidden ? 'true' : 'false');
	});

	// Close dropdown when clicking outside
	useEvent(document, 'click', () => {
		if (!sortDropdown.classList.contains('hidden')) {
			sortDropdown.classList.add('hidden');
			sortButton.setAttribute('aria-expanded', 'false');
		}
	});

	// Sort dropdown options
	const sortOptions = sortDropdown.querySelectorAll('[data-sort]');
	sortOptions.forEach((option) => {
		useEvent(option, 'click', (e) => {
			const newSort = e.target.dataset.sort;
			const currentSort = state.sort;
			const currentOrder = state.order;

			let newOrder = 'desc';

			if (newSort === currentSort) {
				// Toggle order if same sort field
				newOrder = currentOrder === 'desc' ? 'asc' : 'desc';
			}

			setState({ sort: newSort, order: newOrder });
			updateSortUI();
			resetPaging();
			refreshNotes();

			// Scroll to top and close dropdown
			const list = $('#note-list');
			if (list) list.scrollTop = 0;
			sortDropdown.classList.add('hidden');
		});
	});

	// Update sort UI
	const updateSortUI = () => {
		const sortLabels = {
			'created_at': 'Created',
			'updated_at': 'Updated',
			'title': 'Title',
		};

		if (sortLabel) {
			sortLabel.textContent = sortLabels[state.sort] || 'Created';
		}

		// Update aria-label for sort button
		if (sortButton) {
			const sortName = sortLabels[state.sort] || 'Created';
			const direction = state.order === 'asc' ? 'ascending' : 'descending';
			sortButton.setAttribute(
				'aria-label',
				`Sort by ${sortName} (${direction})`,
			);
		}

		// Update aria-checked for dropdown options
		const sortOptions = sortDropdown?.querySelectorAll('[data-sort]');
		if (sortOptions) {
			sortOptions.forEach((option) => {
				const isChecked = option.dataset.sort === state.sort;
				option.setAttribute('aria-checked', isChecked.toString());
			});
		}

		if (sortOrder) {
			// Update SVG chevron direction based on sort order
			if (state.order === 'asc') {
				// Chevron up for ascending - canonical Tailwind/hero-icons path
				sortOrder.innerHTML = sortOrderIcon(true);
			} else {
				// Chevron down for descending
				sortOrder.innerHTML = sortOrderIcon(false);
			}
		}
	};
	// Initialize UI
	updateSortUI();
};
