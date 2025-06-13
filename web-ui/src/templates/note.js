export const noteCardTemplate = () => {
	return `<div class="flex cursor-pointer bg-white dark:bg-gray-800 rounded-lg shadow hover:bg-gray-50 dark:hover:bg-gray-700 mb-2 h-[120px]">
            <div class="w-2 rounded-l-lg note-color-bar rounded-lg"></div>
            <div class="flex-1 p-3 min-w-0">
              <h3 class="font-medium truncate note-title dark:text-gray-100"></h3>
              <p class="text-sm text-gray-500 dark:text-gray-400 line-clamp-2 note-body"></p>
              <span class="text-xs text-gray-400 dark:text-gray-500 note-updated"></span>
            </div>
          </div>`;
};

export const editorPlaceholderTemplate = () => {
	return `<div class="text-center text-gray-400 dark:text-gray-500 italic m-auto">
            <h2 class="text-xl mb-4">Pick a note from the list</h2>
            <p>Nothing to edit. Select a note or create one.</p>
          </div>`;
};

export const skeletonCardTemplate = (/** @type {number} */ rowHeight) => {
	return `<div style="height:${
		rowHeight - 8
	}px" class="animate-pulse bg-gray-200 dark:bg-gray-700 rounded mb-2"></div>`;
};

export const emptyStateTemplate = () => {
	return `<p id="no-notes-empty-state" class="text-gray-400 dark:text-gray-500 italic text-center p-4">No notes yet. Hit + to create one.</p>`;
};
