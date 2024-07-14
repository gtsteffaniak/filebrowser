const CompressionPlugin = require("compression-webpack-plugin");
const path = require('path');

module.exports = {
  runtimeCompiler: true,
  publicPath: "[{[ .StaticURL ]}]",
  parallel: true,
  configureWebpack: {
    resolve: {
      alias: {
        // Add Ace Editor alias for importing it in your Vue components
        ace: "ace-builds/src-min-noconflict",
      },
      extensions: ["*", ".js", ".vue", ".json"],
    },
    plugins: [
      new CompressionPlugin({
        include: /\.js$/,
        deleteOriginalAssets: true,
        minRatio: 0.8,
      }),
    ],
  },
};
