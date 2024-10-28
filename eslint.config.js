import pluginJs from "@eslint/js";
import * as graphql from "@graphql-eslint/eslint-plugin";

export default [
	{
		files: ["**/*.js"],
		rules: pluginJs.configs.recommended.rules,
	},
	{
		files: ["**/*.graphqls"],
		languageOptions: {
			parser: graphql.parser,
			parserOptions: {
				graphQLConfig: {
					schema: "./internal/v1/graphv1/schema/*.graphqls",
				},
			},
		},
		plugins: {
			"@graphql-eslint": { rules: graphql.rules },
		},
		rules: graphql.configs["flat/schema-all"],
	},
];
