module.exports = {
    "root": true,
    "parser": "@typescript-eslint/parser",
    "parserOptions": {
        "project": ["./tsconfig.json"],
        "tsconfigRootDir": __dirname,
        "ecmaFeatures": {
            "jsx": true
        }
    },
    "settings": {
        "react": {
            "version": "detect"
        }
    },
    "plugins": [
        "@typescript-eslint"
    ],
    "extends": [
        "eslint:recommended",
        "plugin:@typescript-eslint/eslint-recommended",
        "plugin:@typescript-eslint/recommended",
        "plugin:react/recommended"
    ],
    "rules": {
        "comma-dangle": ["error", "never"],
        "eqeqeq": "error",
        "brace-style": "off",
        "@typescript-eslint/brace-style": [
            "error",
            "1tbs",
            {
                "allowSingleLine": false
            }
        ],
        "@typescript-eslint/no-inferrable-types": 0,
        "@typescript-eslint/quotes": ["error", "double", { "avoidEscape": true }],
        "@typescript-eslint/space-before-function-paren": 0,
        "@typescript-eslint/semi": 1,
        "@typescript-eslint/no-magic-numbers": 0,
        "@typescript-eslint/strict-boolean-expressions": 0,
        "@typescript-eslint/explicit-function-return-type": [
            "error",
            {
                allowExpressions: true
            }
        ],
        "@typescript-eslint/typedef": [
            "error",
            {
                "arrayDestructuring": false,
                "arrowParameter": false,
                "memberVariableDeclaration": true,
                "parameter": true,
                "objectDestructuring": false,
                "propertyDeclaration": true,
                "variableDeclaration": false
            },
        ],
        "@typescript-eslint/prefer-readonly-parameter-types": 0,
        "@typescript-eslint/no-dynamic-delete": 0,
        "@typescript-eslint/no-type-alias": 0,
        "@typescript-eslint/explicit-member-accessibility": [
            "error",
            {
                "overrides": {
                    "constructors": "off"
                }
            }
        ],
        "@typescript-eslint/init-declarations": 0
    }
};