import { protocol } from 'electron'
import { createReadStream } from 'node:fs'
import { extname, join } from 'node:path'
const schemaConfig = {
  standard: true, supportFetchAPI: true, bypassCSP: true, corsEnabled: true, stream: true
}

protocol.registerSchemesAsPrivileged([{ scheme: "app", privileges: schemaConfig }]);

function getMimeType(extension: string) {
  const mimeTypes = {
    '.js': 'text/javascript',
    '.html': 'text/html',
    '.css': 'text/css',
    '.png': 'image/png',
    '.jpg': 'image/jpg',
    '.gif': 'image/gif',
    '.svg': 'image/svg+xml',
    '.json': 'application/json',
    '.woff': 'application/font-woff',
    '.woff2': 'application/font-woff2',
    '.ttf': 'application/font-ttf',
    '.eot': 'application/vnd.ms-fontobject',
    '.otf': 'application/font-otf',
    '.xml': 'application/xml',
    '.pdf': 'application/pdf',
    '.zip': 'application/zip',
    '.gz': 'application/gzip',
    '.appcache': 'text/cache-manifest',
    '.mp4': 'video/mp4',
    '.webm': 'video/webm',
    '.ogg': 'video/ogg',
    '.mp3': 'audio/mpeg',
    '.wav': 'audio/wav',
    '.m4a': 'audio/x-m4a',
    '.swf': 'application/x-shockwave-flash',
    '.flv': 'video/x-flv',
    '.exe': 'application/x-msdownload',
    '.psd': 'application/octet-stream',
    '.ai': 'application/postscript',
    '.eps': 'application/postscript',
    '.ps': 'application/postscript',
    '.doc': 'application/msword',
    '.rtf': 'application/rtf',
    '.xls': 'application/vnd.ms-excel',
    '.ppt': 'application/vnd.ms-powerpoint',
    '.docx': 'application/msword',
    '.xlsx': 'application/vnd.ms-excel',
    '.pptx': 'application/vnd.ms-powerpoint',
    '.wasm': 'application/wasm',
    '.m3u8': 'application/x-mpegURL',
    '.ts': 'video/MP2T',
    '.mjs': 'application/javascript',
    '.webmanifest': 'application/manifest+json',
    '.ico': 'image/x-icon',
    '.webp': 'image/webp',
    '.map': 'application/json',
    '.txt': 'text/plain',
    '.md': 'text/markdown',
    '.csv': 'text/csv',
    '.vtt': 'text/vtt',
    '.srt': 'text/srt',
    '.3gp': 'video/3gpp',
    '.3g2': 'video/3gpp2',
  }
  return Reflect.get(mimeTypes, extension) || ""
}
export function registerScheme() {
  protocol.registerStreamProtocol('app', (request, callback) => {
    console.log('protocol request: ', request);
    let pathname = new URL(request.url).pathname;
    console.log(pathname);
    let extension = extname(pathname).toLowerCase();
    console.log(extension);
    if (extension === '') {
      pathname = 'index.html'
      extension = '.html'
    }
    const tarFilePath = join(__dirname, pathname);
    console.log(tarFilePath);
    callback({
      statusCode: 200,
      headers: {
        'content-type': getMimeType(extension)
      },
      data: createReadStream(tarFilePath)
    })
  })
}