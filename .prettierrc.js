import SqlPlugin, * as pps from "prettier-plugin-sql"

const SQLCFormat = {
	...SqlPlugin,
	printers: {
		sql: {
			print(path, opts) {
				const source = SqlPlugin.printers.sql.print(path, opts);
				return source.replaceAll(/(sqlc\.\w+)\s\(/g, "$1\(");
			}
		}
	}
}

const config = {
	"useTabs": true,
	"singleQuote": false,
	"trailingComma": "all",
	"printWidth": 100,
	"plugins": [SQLCFormat],
	"overrides": [
		{
			"files": "*.sql",
			"options": {
				"language": "postgresql",
				"paramTypes": "{ named: ['@'], custom: [{ regex: '\\w+\\.\\w+' }] }",
				"functionCase": "upper",
				"keywordCase": "upper",
				"dataTypeCase": "upper",
				"newlineBeforeSemicolon": true
			}
		}
	]
}

export default config;
