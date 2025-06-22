import { state } from './store.js';
import { renderSignIn, renderSignUp } from './ui/auth.js';
import { renderApp } from './ui.js';

export const router = () => {
	const route = location.hash || (state.accessToken ? '#/app' : '#/sign-in');

	if (!state.accessToken && route.startsWith('#/app')) {
		location.hash = '#/sign-in';
		return;
	}
	if (state.accessToken && route.startsWith('#/sign-')) {
		location.hash = '#/app';
		return;
	}

	if (route === '#/sign-in') return renderSignIn();
	if (route === '#/sign-up') return renderSignUp();
	if (route.startsWith('#/app')) return renderApp();

	location.hash = state.accessToken ? '#/app' : '#/sign-in';
};

export const bootstrapRouter = () => {
	addEventListener('hashchange', router);
	router();
};
