const HEX_FULL = /^#[0-9a-f]{6}$/i;
const HEX_SHORT = /^#[0-9a-f]{3}$/i;

export const isValidHex = (c) => HEX_FULL.test(c) || HEX_SHORT.test(c);

export const normalizeHex = (c) => {
	if (!c) return c;
	if (HEX_FULL.test(c)) return c.slice(0, 7);
	if (HEX_SHORT.test(c)) {
		const [, r, g, b] = c.match(HEX_SHORT);
		return `#${r}${r}${g}${g}${b}${b}`;
	}
	return c;
};
