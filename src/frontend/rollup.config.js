import vue from 'rollup-plugin-vue'
import { nodeResolve } from '@rollup/plugin-node-resolve'
import commonjs from '@rollup/plugin-commonjs'
import { terser } from "rollup-plugin-terser"
import postcss from 'rollup-plugin-postcss'
import babel from '@rollup/plugin-babel'
import replace from '@rollup/plugin-replace'
import livereload from 'rollup-plugin-livereload'
import css from 'rollup-plugin-css-only'
import autoprefixer from 'autoprefixer'

export default {
  input: 'src/main.js', // Entry file
  output: {
    file: 'dist/build.js', // Output file
    format: 'iife', // Immediately Invoked Function Expression format suitable for <script> tag
  },
  plugins: [
    replace({
      'process.env.NODE_ENV': JSON.stringify('production'),
      'process.env.VUE_ENV': '"client"'
    }),
    nodeResolve({ browser: true, jsnext: true }), // Resolve modules from node_modules
    commonjs(), // Convert CommonJS modules to ES6
    vue({ css: false }), // Handle .vue files
    css({ output: 'bundle.css' }), // css to separate file
    postcss({ plugins: [autoprefixer()]}),
    babel({ babelHelpers: 'bundled' }), // Transpile to ES5
    terser(), // Minify the build
    livereload('dist') // Live reload for development
  ],
}
