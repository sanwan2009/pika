import globals from 'globals';
import tseslint from 'typescript-eslint';

export default [
    {ignores: ['dist', 'node_modules']},
    {
        files: ['src/**/*.{ts,tsx}'],
        languageOptions: {
            parser: tseslint.parser,
            parserOptions: {ecmaVersion: 'latest', sourceType: 'module', ecmaFeatures: {jsx: true}},
            globals: {...globals.browser, ...globals.es2022},
        },
    },
];
