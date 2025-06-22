import { cleanupGlobalEvents, toast, useEvent } from '../ui.js';
import { $ } from '../utils/dom.js';
import { api } from '../api.js';
import { setState } from '../store.js';
import { authTemplate } from '../templates/index.js';
import { router } from '../router.js';

/* ---------- Auth view builder ---------- */
const renderAuth = (/** @type {string} */ mode /* 'in' | 'up' */) => {
	cleanupGlobalEvents();

	const isSignIn = mode === 'in';
	const formId = isSignIn ? 'sign-in-form' : 'sign-up-form';

	$('#root').innerHTML = authTemplate(isSignIn, formId);

	/* submit handler */
	useEvent($(`#${formId}`), 'submit', async (e) => {
		e.preventDefault();
		const email = e.target.email.value;
		const pass = e.target.password.value;

		try {
			const r = isSignIn ? await api.signIn(email, pass) : await api.signUp(email, pass);

			setState({ accessToken: r.token, refreshToken: r.refresh_token });
			location.hash = '#/app';
			router();
		} catch (e) {
			console.error(e);
			toast(
				isSignIn ? 'Invalid credentials or sign-in failed' : 'Error creating account:' + e,
			);
		}
	});
};

/* ---------- Public shortcuts ---------- */
const renderSignIn = () => renderAuth('in');
const renderSignUp = () => renderAuth('up');

export { renderSignIn, renderSignUp };
