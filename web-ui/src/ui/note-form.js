// @ts-nocheck - we using templates in src/templates/note.form.js, and we don't need to check the types
// src/note-form.js
import { isValidHex, normalizeHex } from '../utils/color.js';
import { toast, useEvent } from '../ui.js';
import { noteFormTemplate } from '../templates/index.js';

/* ---------- Note Form Builder ---------- */
export const buildNoteForm = ({
	mode = 'edit', // 'edit' | 'create'
	note = { title: '', body: '', color: '#3b82f6' },
	onSubmit,
	onDelete,
	onCancel,
}) => {
	const tpl = document.createElement('div');
	tpl.innerHTML = noteFormTemplate();
	const root = tpl.querySelector('.note-form');

	// Set data attribute for mode-specific styling
	root.setAttribute('data-mode', mode);

	// Wire inputs
	const title = root.querySelector('#nf-title');
	const body = root.querySelector('#nf-body');
	const color = root.querySelector('#nf-color');

	title.value = note.title || '';
	body.value = note.body || '';
	color.value = note.color && isValidHex(note.color) ? normalizeHex(note.color) : '#3b82f6';

	// Adjust buttons for mode
	const submitBtn = root.querySelector('#nf-submit');
	const buttonContainer = root.querySelector('.button-container');

	submitBtn.textContent = mode === 'create' ? 'Create' : 'Save';

	// Create delete button only for edit mode
	let deleteBtn = null;
	if (mode === 'edit') {
		deleteBtn = document.createElement('button');
		deleteBtn.type = 'button';
		deleteBtn.className = 'px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700';
		deleteBtn.setAttribute('aria-label', 'Delete note');
		deleteBtn.textContent = 'Delete';
		buttonContainer.appendChild(deleteBtn);
	}

	// Create cancel button only when needed
	let cancelBtn = null;
	if (mode === 'create' && onCancel) {
		cancelBtn = document.createElement('button');
		cancelBtn.type = 'button';
		cancelBtn.className =
			'px-4 py-2 bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 rounded hover:bg-gray-300 dark:hover:bg-gray-500';
		cancelBtn.textContent = 'Cancel';
		buttonContainer.appendChild(cancelBtn);
	}

	// Handlers (all callers share)
	useEvent(root, 'submit', (e) => {
		e.preventDefault();

		const colorValue = color.value;
		if (!color.checkValidity() || (colorValue && !isValidHex(colorValue))) {
			toast('Invalid color');
			return;
		}

		onSubmit?.({
			...note,
			title: title.value,
			body: body.value,
			color: colorValue ? normalizeHex(colorValue) : colorValue,
		});
	});

	if (deleteBtn) {
		useEvent(deleteBtn, 'click', () => onDelete?.(note));
	}
	if (cancelBtn) {
		useEvent(cancelBtn, 'click', () => onCancel?.());
	}

	return root;
};
