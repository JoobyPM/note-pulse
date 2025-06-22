// eslint.config.js
import js from '@eslint/js';

export default [
	js.configs.recommended,
	{
		files: ['src/**/*.js'],
		languageOptions: {
			ecmaVersion: 2022,
			sourceType: 'module',
			globals: {
				console: 'readonly',
				window: 'readonly',
				document: 'readonly',
				sessionStorage: 'readonly',
				localStorage: 'readonly',
				location: 'readonly',
				setTimeout: 'readonly',
				clearTimeout: 'readonly',
				addEventListener: 'readonly',
				fetch: 'readonly',
				WebSocket: 'readonly',
				atob: 'readonly',
				Notyf: 'readonly',
				AbortController: 'readonly',
				IntersectionObserver: 'readonly',
				DOMParser: 'readonly',
				NodeFilter: 'readonly',
				URL: 'readonly',
				requestAnimationFrame: 'readonly',
			},
		},
		rules: {
			'no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
			'no-console': ['warn', { allow: ['warn', 'error'] }],
			'prefer-const': 'error',
			'no-var': 'error',
		},
	},
];
