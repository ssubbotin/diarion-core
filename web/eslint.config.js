import js from '@eslint/js';
import ts from 'typescript-eslint';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';

export default ts.config(
	js.configs.recommended,
	...ts.configs.recommended,
	...svelte.configs['flat/recommended'],
	{
		languageOptions: {
			globals: { ...globals.browser, ...globals.node }
		}
	},
	{
		files: ['**/*.svelte'],
		languageOptions: { parserOptions: { parser: ts.parser } }
	},
	{
		// Plain in-app links (`<a href="/about">`) are used throughout Diarion's UI;
		// SvelteKit's `resolve()` is intended for base-path setups we don't use in Phase 1.
		// Disable the rule for vendored shadcn primitives and our own UI components.
		files: ['src/lib/components/ui/**', 'src/lib/ui/**'],
		rules: {
			'svelte/no-navigation-without-resolve': 'off'
		}
	},
	{ ignores: ['build/', '.svelte-kit/', 'dist/', 'playwright-report/'] }
);
