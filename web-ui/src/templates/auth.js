export const authTemplate = (isSignIn, formId) => {
	const title = isSignIn ? 'Sign in' : 'Sign up';
	const button = isSignIn ? 'Sign in' : 'Create account';
	const btnClass = isSignIn ? 'bg-blue-600 hover:bg-blue-700' : 'bg-green-600 hover:bg-green-700';
	const footText = isSignIn ? 'No account?' : 'Have an account?';
	const footHref = isSignIn ? '#/sign-up' : '#/sign-in';
	const footLink = isSignIn ? 'Sign up' : 'Sign in';
	return `
      <div class="flex items-center justify-center h-full">
        <form id="${formId}" class="max-w-sm w-full bg-white dark:bg-gray-800 p-6 rounded-xl shadow">
          <h2 class="text-2xl font-semibold mb-4 text-center text-gray-900 dark:text-gray-100">${title}</h2>
          <input type="email"    id="email"    placeholder="Email"
                 class="w-full mb-3 px-3 py-2 border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 rounded"
                 autocomplete="email" required>
          <input type="password" id="password" placeholder="Password"
                 class="w-full mb-4 px-3 py-2 border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 rounded"
                 autocomplete="${isSignIn ? 'current-password' : 'new-password'}"
                 required minlength="6">
          <button class="w-full py-2 text-white rounded ${btnClass}">${button}</button>
          <p class="text-sm text-center mt-4 text-gray-600 dark:text-gray-400">${footText}
            <a href="${footHref}" class="text-blue-600 underline">${footLink}</a>
          </p>
        </form>
      </div>`;
};
