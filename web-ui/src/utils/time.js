// Static units array for timeAgo
/** @type {[number, string][]} */
const timeUnits = [[60, 's'], [60, 'm'], [24, 'h'], [30, 'd'], [12, 'mo']];

/**
 * @param {string | number | Date} ts
 * @returns {string}
 */
export const timeAgo = (ts) => {
	const now = Date.now(); // Memoised once per call
	const diff = Math.floor(
		(now - /** @type {number} */ (new Date(ts).getTime())) / 1000,
	);
	if (diff < 5) return 'just now';
	let v = diff, i = 0;
	while (i < timeUnits.length && v >= /** @type {number} */ (timeUnits[i][0])) {
		v = Math.floor(v / /** @type {number} */ (timeUnits[i][0]));
		i++;
	}
	return v + /** @type {string} */
		(timeUnits[Math.min(i, timeUnits.length - 1)][1]) + ' ago';
};
