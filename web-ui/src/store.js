// @ts-check
const listeners = new Set();

export const state = {
	// Auth
	accessToken: null,
	refreshToken: null,

	// Notes
	notesById: new Map(),
	noteOrder: [], // dense array (undefined allowed) of ids
	currentNoteId: null,

	// Search & sort
	offset: 0,
	limit: 30,
	q: '',
	sort: 'created_at',
	order: 'desc',
	totalCount: null, // supplied by first fetch

	// WebSocket state
	ws: null,
	reconnectTimerId: null,
	backoffDelay: 0,

	// Token refresh state
	refreshingPromise: null,
	refreshFailures: 0,
	refreshBlockedUntil: 0,
	lastFailure: 0,
	accessExp: 0,
	ACCESS_THRESHOLD_SEC: 300,
};

// State helpers
export const setState = (patch) => {
	Object.assign(state, patch);
	listeners.forEach((fn) => fn(state));
};

export const subscribe = (fn) => {
	listeners.add(fn);
	return () => listeners.delete(fn);
};

// Helper to get the current note
export const getCurrentNote = () => state.currentNoteId ? state.notesById.get(state.currentNoteId) : null;

// Add or update a note
export const updateNote = (note) => {
	state.notesById.set(note.id, note);
	if (!state.noteOrder.includes(note.id)) state.noteOrder.unshift(note.id);
	setState({ notesById: state.notesById, noteOrder: state.noteOrder });
};

export const addNote = (note) => {
	state.noteOrder.unshift(note.id);
	state.notesById.set(note.id, note);
	setState({ notesById: state.notesById, noteOrder: state.noteOrder, totalCount: (state.totalCount ?? 0) + 1 });
};

// Remove a note
export const removeNote = (id) => {
	const idx = state.noteOrder.indexOf(id);
	if (idx !== -1) state.noteOrder.splice(idx, 1);
	state.notesById.delete(id);
	setState({
		totalCount: state.totalCount - 1,
		notesById: state.notesById,
		noteOrder: state.noteOrder,
		currentNoteId: state.currentNoteId === id ? null : state.currentNoteId,
	});
};

// Clear all notes
export const clearNotes = () => {
	state.noteOrder.length = 0;
	state.notesById.clear();
	setState({
		notesById: state.notesById,
		noteOrder: state.noteOrder,
		currentNoteId: null,
		totalCount: null,
	});
};

/**
 * Legacy helper used by search / sort widgets.
 * It wipes local paging info without touching current filters.
 */
export const resetPaging = () => {
	state.noteOrder.length = 0;
	state.notesById.clear();
	setState({
		notesById: state.notesById,
		noteOrder: state.noteOrder,
		totalCount: null,
	});
};
