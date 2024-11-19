import pluginJs from "@eslint/js";
import graphql from "@graphql-eslint/eslint-plugin";

export default [
	{
		files: ["**/*.js"],
		rules: pluginJs.configs.recommended.rules,
	},
	{
		files: ["internal/**/*.graphqls"],
		languageOptions: {
			parser: graphql.parser,
			parserOptions: {
				graphQLConfig: {
					schema: "./internal/graph/schema/*.graphqls",
				},
			},
		},
		plugins: {
			"@graphql-eslint": graphql,
		},
		rules: {
			...graphql.configs["flat/schema-recommended"].rules,
			"@graphql-eslint/description-style": ["off"],
			"@graphql-eslint/require-description": ["off"],
			"@graphql-eslint/input-name": [
				"error",
				{ checkInputType: true, caseSensitiveInputType: false },
			],
			"@graphql-eslint/strict-id-in-types": [
				"error",
				{
					acceptedIdNames: ["id"],
					acceptedIdTypes: ["ID"],
					exceptions: {
						types: [
							"TeamInventoryCountApplications",
							"ApplicationManifest",
							"ApplicationResources",
						],
						suffixes: ["Payload", "Connection", "Edge", "Status"],
					},
				},
			],
		},
	},
];
