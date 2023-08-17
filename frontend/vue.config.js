const CompressionPlugin = require("compression-webpack-plugin");

module.exports = {
  runtimeCompiler: true,
  publicPath: "[{[ .StaticURL ]}]",
  parallel: true,
  configureWebpack: {
    plugins: [
      new CompressionPlugin({
        include: /\.js$/,
        deleteOriginalAssets: true,
        threshold: 10240, // Only compress files larger than 10KB
        minRatio: 0.8,
      }),
    ],
  },
};
