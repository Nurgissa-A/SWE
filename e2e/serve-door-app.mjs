// Serves mobile/dist (the check-in app's web build) and forwards /api to the BiletFlow web app,
// the way the app is deployed next to the website.
import { createReadStream, existsSync, statSync } from 'node:fs';
import http from 'node:http';
import { extname, join, resolve } from 'node:path';

const DIST = resolve(process.env.DOOR_APP_DIST ?? '../mobile/dist');
const upstream = new URL(process.env.WEB_URL ?? 'http://localhost:3000');
const port = Number(process.env.PORT ?? 3200);
const types = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.png': 'image/png', '.ico': 'image/x-icon', '.json': 'application/json', '.ttf': 'font/ttf', '.wasm': 'application/wasm' };

if (!existsSync(join(DIST, 'index.html'))) {
  console.error(`No web build at ${DIST}. Run: cd mobile && npm run export:web`);
  process.exit(1);
}

http.createServer((req, res) => {
  if (req.url.startsWith('/api/')) {
    const up = http.request({ host: upstream.hostname, port: upstream.port, path: req.url, method: req.method, headers: { ...req.headers, host: upstream.host } }, (r) => {
      res.writeHead(r.statusCode, r.headers);
      r.pipe(res);
    });
    up.on('error', () => { res.writeHead(502); res.end(); });
    req.pipe(up);
    return;
  }
  let file = join(DIST, decodeURIComponent(req.url.split('?')[0]));
  if (!file.startsWith(DIST) || !existsSync(file) || statSync(file).isDirectory()) file = join(DIST, 'index.html');
  res.writeHead(200, { 'Content-Type': types[extname(file)] ?? 'application/octet-stream' });
  createReadStream(file).pipe(res);
}).listen(port, () => console.log(`check-in app on http://localhost:${port}`));
