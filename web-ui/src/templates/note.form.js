export const noteFormTemplate = () => {
	return `
    <form class="note-form flex flex-col h-full">
      <input id="nf-title" placeholder="Note title" class="mb-3 text-2xl font-semibold outline-none w-full bg-transparent text-gray-900 dark:text-gray-100" >
      <textarea id="nf-body" rows="15" placeholder="Note content"
        class="flex-1 resize-none w-full outline-none border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 p-3 rounded mb-4">
      </textarea>
      <div class="flex items-center mb-4 space-x-3">
        <label class="text-sm">Color:</label>
        <input id="nf-color" type="color" value="#3b82f6" class="h-8 w-8 border-0" aria-label="Note color picker">
      </div>
      <div class="button-container">
        <button id="nf-submit" type="submit" class="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700">Save</button>
      </div>
    </form>`;
};
