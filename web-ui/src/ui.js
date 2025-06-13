// @ts-nocheck - no typechecking for now
'use strict';

/* ----------------- third-party ----------------- */
import { Notyf } from 'notyf';
import 'notyf/notyf.min.css';

/* ----------------- local modules ---------------- */
import { api, connectSocket, disconnectSocket, logoutLocal } from './api.js';
import { $, el } from './utils/dom.js';
import { ClusterizeNotes } from './ui/clusterize-notes.js';
import { buildNoteForm } from './ui/note-form.js';
import { $tpl, editorPlaceholderTemplate, mainTemplate, noteCardTemplate } from './templates/index.js';
import { toggleDarkMode, updateDarkMode } from './ui/dark.mode.js';
import { setupSearchAndSort } from './ui/search.js';
import { timeAgo } from './utils/time.js';
import { addNote, clearNotes, removeNote, setState, state, subscribe, updateNote } from './store.js';
import { isValidHex, normalizeHex } from './utils/color.js';
import { router } from './router.js';

/* ---------- toast ---------- */
let notyf;
export const initializeToast = () => {
	notyf = new Notyf({ duration: 3000, position: { x: 'center', y: 'top' } });
};
export const toast = (m, t = 'error') => t === 'error' ? notyf.error(m) : notyf.success(m);

/* ---------- globals & helpers ---------- */
let cluster = null; // active ClusterizeNotes instance
let globalEventCleanups = []; // holds unsubscribe / removeEvent fn refs

const $counter = () => $('#notes-count');
/** Increment totalCount state and refresh counter. */

/** Re‑compute "N of A" string.  @param {number|null} firstIdx */
function updateNotesCounter(pct) {
	const offset = typeof pct === 'number' ? pct : state.offset;

	if (updateNotesCounter?.lastOffset === offset) return;

	updateNotesCounter.lastOffset = offset;
	const counterEl = $counter();
	if (!counterEl) return;
	const total = state.totalCount ?? 0;
	if (!total) {
		counterEl.textContent = '';
		return;
	}

	const current = Math.max(1, offset);

	counterEl.textContent = `${current} of ${total}`;
}

subscribe(updateNotesCounter);

const addNoteEl = (note, index = 0) => {
	addNote(note, index);
	cluster?.addNote(note, index);
};

const deleteNote = (id) => {
	removeNote(id);
	cluster?.deleteNote(id);
};

export const addGlobalEventCleanup = (fn) => {
	globalEventCleanups.push(fn);
};

export const useEvent = (el, type, fn) => {
	el.addEventListener(type, fn);
	const off = () => el.removeEventListener(type, fn);
	addGlobalEventCleanup(off);
	return off;
};

/* ---------- template helpers ---------- */
const noteCardTpl = $tpl(noteCardTemplate());
const editorPlaceholderTpl = $tpl(editorPlaceholderTemplate());

const createNoteCard = (note, selected = false) => {
	const node = noteCardTpl.cloneNode(true).firstElementChild;
	if (selected) node.classList.add('selected');

	const safeColor = note.color && isValidHex(note.color) ? normalizeHex(note.color) : '#3b82f6';
	node.querySelector('.note-color-bar').style.background = safeColor;

	node.querySelector('.note-title').textContent = note.title;
	node.querySelector('.note-body').textContent = note.body;
	node.querySelector('.note-updated').textContent = timeAgo(note.updated_at);

	node.dataset.noteId = note.id;
	node.setAttribute('role', 'option');
	node.setAttribute('aria-label', note.title);
	node.setAttribute('aria-selected', selected.toString());
	node.setAttribute('tabindex', '0');
	node.querySelector('.note-title').id = `nt-${note.id}`;
	return node;
};

const createEditorPlaceholder = () => editorPlaceholderTpl.cloneNode(true).firstElementChild;

/* ---------- offset-based fetch helper ---------- */
const fetchSlice = async (offset) => {
	setState({ offset });
	const resp = await api.listNotes({
		offset,
		limit: state.limit,
		sort: state.sort,
		order: state.order,
		q: state.q,
	});

	resp.notes.forEach((n) => state.notesById.set(n.id, n));

	// keep order array dense, undefined holes allowed
	if (state.noteOrder.length < resp.total_count) {
		state.noteOrder.length = resp.total_count;
	}
	resp.notes.forEach((n, i) => (state.noteOrder[offset + i] = n.id));

	setState({
		notesById: state.notesById,
		noteOrder: state.noteOrder,
		totalCount: resp.total_count,
	});

	if (offset === 0) updateNotesCounter(offset + 1);
	return resp;
};

/* ---------- render list with Clusterize-Lazy ---------- */
export const renderNoteList = () => {
	const list = $('#note-list');
	if (cluster) return;

	cluster = new ClusterizeNotes({
		container: list,
		getCard: (id) => {
			const note = state.notesById.get(id);
			return note ? createNoteCard(note, state.currentNoteId === id) : document.createElement('div');
		},
		fetchSlice,
		rowHeight: 128, // 120px + 8px margin
		// onFirstVisible: updateNotesCounter,
		scrollingProgress: (pct) => {
			updateNotesCounter(pct);
		},
	});

	/* card click / key activation */
	useEvent(list, 'click', (e) => {
		const card = e.target.closest('[data-note-id]');
		if (card) {
			const note = state.notesById.get(card.dataset.noteId);
			if (note) loadEditor(note);
		}
	});

	useEvent(list, 'keydown', (e) => {
		if (e.key === 'Enter' || e.key === ' ') {
			const card = e.target.closest('[data-note-id]');
			if (card) {
				e.preventDefault();
				const note = state.notesById.get(card.dataset.noteId);
				if (note) loadEditor(note);
			}
		}
	});
};

/* ---------- main app ---------- */
export const renderApp = () => {
	cleanupGlobalEvents();
	disconnectSocket();

	$('#root').innerHTML = mainTemplate();

	/* top bar & toggles */
	useEvent($('#new-note'), 'click', openNoteModal);
	useEvent($('#dark-toggle'), 'click', toggleDarkMode);
	updateDarkMode();
	setupSearchAndSort();

	/* sign-out */
	useEvent($('#sign-out'), 'click', async () => {
		try {
			await api.signOut();
		} catch (e) {
			console.error(e);
		}
		logoutLocal();
		location.hash = '#/sign-in';
		router();
		toast('Signed out', 'success');
	});

	/* enable/disable new-note button when WS connects */
	const unsubscribe = subscribe((st) => {
		const btn = $('#new-note');
		if (btn) {
			if (st.ws && st.ws.readyState === WebSocket.OPEN) {
				btn.removeAttribute('disabled');
			} else btn.setAttribute('disabled', '');
		}
	});
	addGlobalEventCleanup(unsubscribe);

	/* initial UI */
	clearNotes();
	clearEditor();
	renderNoteList();

	connectSocket(handleWsMessage).catch((e) => toast(e.message));
};

/* ---------- editor helpers ---------- */
const clearEditor = () => {
	const editor = $('#editor');
	editor.textContent = '';
	editor.appendChild(createEditorPlaceholder());

	try {
		sessionStorage.removeItem('notePulseLastNote');
	} catch (e) {
		console.error(e);
	}
	document.title = 'NotePulse';
	setState({ currentNoteId: null });
};

const loadEditor = (note) => {
	setState({ currentNoteId: note.id });

	try {
		sessionStorage.setItem('notePulseLastNote', note.id);
	} catch (e) {
		console.error(e);
	}

	if (!location.hash.startsWith('#/app')) {
		location.hash = '#/app';
	}

	const safeTitle = note.title ? note.title.replace(/[<>&"']/g, '') : 'Untitled';
	document.title = `${safeTitle} - NotePulse`;

	/* update selected card without full list rerender */
	const cards = $('#note-list').querySelectorAll('[data-note-id]');
	for (const card of cards) {
		if (card.dataset.noteId === note.id) {
			card.classList.add('selected');
			card.setAttribute('aria-selected', 'true');
		} else {
			card.classList.remove('selected');
			card.setAttribute('aria-selected', 'false');
		}
	}

	const noteList = $('#note-list');
	if (noteList) noteList.setAttribute('aria-activedescendant', `nt-${note.id}`);

	/* build editor */
	const editor = $('#editor');
	editor.textContent = '';

	const renderEditorContent = () => {
		editor.textContent = '';

		const form = buildNoteForm({
			mode: 'edit',
			note,
			onSubmit: async (formData) => {
				const submitBtn = form.querySelector('#nf-submit');
				const prev = { ...note };
				submitBtn.disabled = true;

				Object.assign(note, formData, { updated_at: new Date().toISOString() });
				updateNote(note);
				cluster?.updateNote(note);

				try {
					const r = await api.updateNote(note.id, formData);
					updateNote(r.note);
					cluster?.updateNote(r.note);

					const titleInput = form.querySelector('#nf-title');
					const bodyInput = form.querySelector('#nf-body');
					const colorInput = form.querySelector('#nf-color');

					if (
						r.note.title !== titleInput.value ||
						r.note.body !== bodyInput.value ||
						r.note.color !== colorInput.value
					) {
						loadEditor(r.note);
					}

					toast('Saved', 'success');
				} catch {
					Object.assign(note, prev);
					updateNote(note);
					cluster?.updateNote(note);
					loadEditor(note);
					toast('Error saving note');
				} finally {
					submitBtn.disabled = false;
				}
			},
			onDelete: async () => {
				deleteNote(note.id);
				clearEditor();
				try {
					await api.deleteNote(note.id);
				} catch {
					toast('Error deleting note');
				}
			},
		});

		if (form) {
			editor.appendChild(form);
			const titleInput = form.querySelector('#nf-title');
			if (titleInput) requestAnimationFrame(() => titleInput.focus());
		}
	};

	renderEditorContent();
};

/* ---------- WebSocket handler ---------- */
const handleWsMessage = (ev) => {
	const { type, note } = JSON.parse(ev.data);

	switch (type) {
		case 'created': {
			const isNew = !state.notesById.has(note.id);
			updateNote(note);

			if (isNew) {
				addNoteEl(note, 0);
				toast('Note created', 'success');
				loadEditor(note);
			} else {
				cluster?.updateNote(note);
			}
			break;
		}
		case 'updated': {
			if (state.notesById.has(note.id)) {
				updateNote(note);
				cluster?.updateNote(note);
				if (state.currentNoteId === note.id) loadEditor(note);
			}
			break;
		}
		case 'deleted': {
			deleteNote(note.id);
			if (state.currentNoteId === note.id) clearEditor();
			toast('Note deleted', 'error');
			break;
		}
	}
};

/* ---------- new-note modal ---------- */
const openNoteModal = () => {
	const prevFocus = document.activeElement;

	const overlay = el('div', 'modal-overlay');
	overlay.setAttribute('role', 'dialog');
	overlay.setAttribute('aria-modal', 'true');
	overlay.setAttribute('aria-labelledby', 'modal-title');

	const modalContainer = document.createElement('div');
	modalContainer.className = 'bg-white dark:bg-gray-800 rounded-xl shadow-lg p-6 w-96';

	const title = document.createElement('h3');
	title.id = 'modal-title';
	title.className = 'text-lg font-semibold mb-4 text-gray-900 dark:text-gray-100';
	title.textContent = 'New note';
	modalContainer.appendChild(title);

	const closeModal = () => {
		overlay.remove();
		document.removeEventListener('keydown', handleKeydown);
		prevFocus?.focus();
	};

	const form = buildNoteForm({
		mode: 'create',
		note: { title: '', body: '', color: '#3b82f6' },
		onSubmit: async (formData) => {
			closeModal();

			try {
				await api.createNote(formData);
				// upon success, the note will be added to the list by the WebSocket handler
			} catch (e) {
				console.error(e);
				toast('Failed to create note');
			}
		},
		onCancel: closeModal,
	});

	if (form) modalContainer.appendChild(form);

	overlay.appendChild(modalContainer);
	document.body.appendChild(overlay);

	const focusable = overlay.querySelectorAll('input, textarea, button');
	const firstEl = focusable[0];
	const lastEl = focusable[focusable.length - 1];

	const handleKeydown = (e) => {
		if (e.key === 'Escape') {
			closeModal();
		} else if (e.key === 'Tab') {
			if (e.shiftKey) {
				if (document.activeElement === firstEl) {
					e.preventDefault();
					lastEl.focus();
				}
			} else if (document.activeElement === lastEl) {
				e.preventDefault();
				firstEl.focus();
			}
		}
	};

	document.addEventListener('keydown', handleKeydown);
	firstEl?.focus();
};

/**
 * Gracefully dispose all global listeners and the active Clusterize instance.
 * (Auth UI calls this before it swaps screens.)
 */
export function cleanupGlobalEvents() {
	cluster?.destroy();
	cluster = null;
	globalEventCleanups.forEach((fn) => fn());
	globalEventCleanups = [];
}

/**
 * Re‑initialise the note list after search / sort changes or when an anchor
 * id is supplied.
 *
 * @param {string|null} anchorId
 */
export function reInitialise() {
	// 1) reset reactive store
	clearNotes();

	// 2) rebuild Clusterize instance
	if (cluster) {
		cluster.destroy();
		cluster = null;
	}
	renderNoteList();
}
