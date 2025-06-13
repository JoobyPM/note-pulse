// @ts-nocheck - Clusterize is not typed
import Clusterize from 'clusterize-lazy';
import { debounce } from '../utils/timing.js';
import { emptyStateTemplate, skeletonCardTemplate } from '../templates/index.js';

/**
 * Lightweight façade around Clusterize‑Lazy.
 * The parent code supplies:
 *   • container      — scrollable element (the <div id="note-list"> wrapper)
 *   • getCard(id)    — fn that returns a ready‑made DOM card for a note id
 *   • fetchSlice(off) — fn(offset) -> Promise<{ notes: Note[], total_count: number }>
 */
export class ClusterizeNotes {
	/**
	 * @param {{
	 *   container: HTMLElement
	 *   getCard: (id: string) => HTMLElement
	 *   fetchSlice: (offset: number) => Promise<{ notes: any[], total_count: number }>
	 *   callbacks?: { scrollingProgress?: (pct: number) => void }
	 *   rowHeight?: number
	 * }} cfg
	 */
	constructor(cfg) {
		this.cfg = cfg;
		this.rowHeight = cfg.rowHeight || 80;
		this.container = cfg.container;

		// internal hidden content node for Clusterize to paint into
		this.contentElem = document.createElement('div');
		this.contentElem.id = 'clusterize-content';
		this.container.appendChild(this.contentElem);

		// skeleton row (simple pulse bar)
		const skeleton = skeletonCardTemplate(this.rowHeight);

		const onStop = cfg.callbacks?.scrollingProgress ? debounce(cfg.callbacks.scrollingProgress, 120) : undefined;

		// Wire up Clusterize‑Lazy
		this.cluster = new Clusterize({
			scrollElem: this.container,
			contentElem: this.contentElem,
			rowHeight: this.rowHeight,
			buildIndex: true,
			primaryKey: 'id',
			renderSkeletonRow: () => skeleton,

			renderRaw: (_, note) => cfg.getCard(note.id).outerHTML,

			// first page (offset 0)
			fetchOnInit: async () => {
				const r = await cfg.fetchSlice(0);
				return { totalRows: r.total_count, rows: r.notes };
			},

			// subsequent pages
			fetchOnScroll: async (offset) => {
				const r = await cfg.fetchSlice(offset);
				return r.notes;
			},

			onScrollFinish: (idx) => {
				cfg.onFirstVisible?.(idx);
				onStop?.(idx);
			},
			debounceMs: 20,
			renderEmptyState: () => {
				return emptyStateTemplate();
			},
			scrollingProgress: cfg.scrollingProgress,
		});
	}

	/** programmatic scroll helper (0‑based row index) */
	scrollTo(index) {
		this.cluster.scrollToRow(index);
	}

	/** batch update - arr of { id, data } */
	update(patches) {
		this.cluster.update(patches);
	}

	/** insert one note at index (defaults to top) */
	addNote(note, index = 0) {
		const wasAbove = index < this.getFirstVisible();
		this.cluster.insert([note], index);
		if (wasAbove) this.container.scrollTop += this.rowHeight; // keep anchor
	}

	/** replace an existing note by id */
	updateNote(note) {
		this.cluster.update([{ id: note.id, data: note }]);
	}

	/** remove by id */
	deleteNote(id) {
		const idx = this.cluster._dump().index?.get(id) ?? -1;
		const wasAbove = idx > -1 && idx < this.getFirstVisible();
		this.cluster.delete([id]);
		if (wasAbove) {
			this.container.scrollTop = Math.max(
				0,
				this.container.scrollTop - this.rowHeight,
			);
		}
	}

	invalidate() {
		this.cluster.invalidateCache();
	}

	destroy() {
		this.cluster.destroy();
	}

	/** Utility: current first visible index */
	getFirstVisible() {
		return Math.floor(this.container.scrollTop / this.rowHeight);
	}
}
