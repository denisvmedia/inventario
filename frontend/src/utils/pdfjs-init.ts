import * as pdfjsLib from 'pdfjs-dist'

// Set the worker source to use the CDN version for better reliability
// This avoids issues with local worker file serving in development
pdfjsLib.GlobalWorkerOptions.workerSrc = `//cdnjs.cloudflare.com/ajax/libs/pdf.js/${pdfjsLib.version}/pdf.worker.min.mjs`

export { pdfjsLib }
