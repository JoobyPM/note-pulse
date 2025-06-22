import { noteFormTemplate } from './note.form.js';
import { mainTemplate } from './main.js';
import { editorPlaceholderTemplate, emptyStateTemplate, noteCardTemplate, skeletonCardTemplate } from './note.js';
import { authTemplate } from './auth.js';

export {
	authTemplate,
	editorPlaceholderTemplate,
	emptyStateTemplate,
	mainTemplate,
	noteCardTemplate,
	noteFormTemplate,
	skeletonCardTemplate,
};

export { sortOrderIcon } from './icons.js';

export const $tpl = (/** @type {string} */ tplStr) => {
	const wrapper = document.createElement('div');
	wrapper.innerHTML = tplStr;
	return wrapper;
};
