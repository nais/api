import pluginJs from "@eslint/js";
import * as graphql from "@graphql-eslint/eslint-plugin";

export default [
	{
		files: ["**/*.js"],
		rules: pluginJs.configs.recommended.rules,
	},
	{
		files: ["internal/v1/**/*.graphqls"],
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
		rules: {
			...graphql.configs["flat/schema-recommended"],
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
