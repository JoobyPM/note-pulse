/**
 * Returns a debounced version of `fn`.
 * The call is delayed until `ms` ms have elapsed since the last invocation.
 *
 * @template {(...args: any[]) => any} F
 * @param {F} fn         - Function to debounce
 * @param {number} [ms=200] - Delay in milliseconds
 * @returns {(...args: Parameters<F>) => void}  Debounced function
 */
export function debounce(fn, ms = 200) {
	/** @type {ReturnType<typeof setTimeout> | undefined} */
	let timer;

	const debounced = function (/** @type {any[]} */ ...args) {
		if (timer !== undefined) clearTimeout(timer);
		// preserve caller's `this`
		timer = setTimeout(() => fn.apply(this, args), ms);
	};

	/** Cancel any pending invocation. */
	debounced.cancel = () => {
		if (timer !== undefined) clearTimeout(timer);
		timer = undefined;
	};

	return debounced;
}
