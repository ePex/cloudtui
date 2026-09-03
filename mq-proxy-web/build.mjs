// Bundles src/main.js and inlines it into index.html's script tag,
// producing dist/index.html: a single, self-contained file that works
// over file:// (unlike the dev template's <script type="module">, which
// browsers block from loading over file://). See README.md and
// spec/21-amq-web-console for why this exists.
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import * as esbuild from 'esbuild';

const SCRIPT_TAG = '<script type="module" src="./src/main.js"></script>';

const result = await esbuild.build({
  entryPoints: ['src/main.js'],
  bundle: true,
  write: false,
  format: 'iife',
});
const bundledCode = result.outputFiles[0].text;

const template = readFileSync('index.html', 'utf-8');
if (!template.includes(SCRIPT_TAG)) {
  throw new Error(`index.html does not contain the expected script tag: ${SCRIPT_TAG}`);
}
const output = template.replace(SCRIPT_TAG, `<script>\n${bundledCode}</script>`);

mkdirSync('dist', { recursive: true });
writeFileSync('dist/index.html', output);
console.log('Built dist/index.html');
