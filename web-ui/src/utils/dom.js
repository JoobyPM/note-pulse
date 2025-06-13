export const $ = (sel, ctx = document) => ctx.querySelector(sel);
export const el = (tag, cls = '') => {
	const e = document.createElement(tag);
	if (cls) e.className = cls;
	return e;
};
