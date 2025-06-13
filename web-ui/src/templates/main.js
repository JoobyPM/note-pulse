export const mainTemplate = () => {
	return `
    <div class="flex h-full">
      <aside class="w-80 border-r border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 overflow-hidden">
        <div class="flex items-center justify-between p-4">
          <div class="flex items-center space-x-2">
            <img id="notePulseLogo" src="logo.svg" class="size-12" alt="NotePulse">
            <h2 class="text-xl font-semibold text-gray-900 dark:text-gray-100">My Notes</h2>
          </div>
          <div class="flex items-center space-x-2">
            <button id="dark-toggle" class="p-1 rounded hover:bg-gray-100 dark:hover:bg-gray-700" aria-label="Toggle dark mode" aria-pressed="false">
              <svg class="w-4 h-4 text-gray-600 dark:text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                <path class="dark:hidden" d="M10 2L13.09 8.26L20 9L14 14.74L15.18 21.02L10 17.77L4.82 21.02L6 14.74L0 9L6.91 8.26L10 2Z"/>
                <path class="hidden dark:block" d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z"/>
              </svg>
            </button>
            <button id="sign-out" class="px-2 py-1 bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 rounded text-sm hover:bg-gray-300 dark:hover:bg-gray-500">Sign out</button>
            <button id="new-note" class="p-2 bg-blue-600 text-white rounded-full hover:bg-blue-700" aria-label="Create note" disabled>+</button>
          </div>
        </div>
        <div class="px-4 pb-3 border-b border-gray-200 dark:border-gray-700">
          <div class="flex items-center space-x-2 mb-2">
            <input 
              type="search" 
              id="search-input" 
              placeholder="Search…" 
              class="flex-1 px-3 py-2 text-sm border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 rounded"
              autocomplete="off"
            >
            <div class="relative">
              <button id="sort-button" class="px-3 py-2 text-sm bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded hover:bg-gray-200 dark:hover:bg-gray-600 flex items-center space-x-1" aria-haspopup="menu" aria-expanded="false" aria-label="Sort by Created (descending)">
                <span id="sort-label">Created</span>
                <svg id="sort-order" class="w-4 h-4 inline" aria-hidden="true" fill="currentColor" viewBox="0 0 20 20" style="pointer-events: none;">
                  <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"/>
                </svg>
              </button>
              <div id="sort-dropdown" class="absolute top-full right-0 mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-600 rounded shadow-lg z-10 hidden min-w-max" role="menu" aria-labelledby="sort-button">
                <button class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700" role="menuitemradio" aria-checked="true" data-sort="created_at">Created</button>
                <button class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700" role="menuitemradio" aria-checked="false" data-sort="updated_at">Updated</button>
                <button class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-gray-700" role="menuitemradio" aria-checked="false" data-sort="title">Title</button>
              </div>
            </div>
          </div>
          <div id="notes-count" class="text-sm text-gray-500 dark:text-gray-400" aria-live="polite" aria-atomic="true"></div>
        </div>
        <div id="note-list" class="note-list overflow-y-auto h-[calc(100%-120px)] p-2 space-y-2" role="listbox" aria-label="Notes" tabindex="0">
          <div id="contentArea" class="clusterize-content">
          </div>
        </div>
      </aside>
      <main class="flex-1 p-6 overflow-y-auto">
        <div id="editor" class="h-full flex flex-col"></div>
      </main>
    </div>`;
};
