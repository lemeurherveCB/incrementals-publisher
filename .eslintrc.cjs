module.exports = {
  "env": {
    "es2022": true,
    "node": true
  },
  "extends": [
    "eslint:recommended",
    "plugin:import/recommended",
  ],
  "parserOptions": {
    "ecmaVersion": 2022,
    "sourceType": "module",
  },
  "overrides": [
    {
      "files": ["test/**/*.js"],
      "env": {"mocha": true},
      "plugins": ["mocha"],
      "extends": [
        "eslint:recommended",
        "plugin:import/recommended",
        "plugin:mocha/recommended"
      ],
    },
  ],
  "rules": {
    "indent": ["error", 2],
    "quotes": ["error", "double"],
    "key-spacing": ["error", {"mode": "strict"}],
    "import/extensions": ["error", "ignorePackages"],
  },
};
