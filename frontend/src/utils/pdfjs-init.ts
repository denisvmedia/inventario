import * as pdfjsLib from 'pdfjs-dist'

// Set the worker source to use the local worker file
// This is more reliable than using a CDN and works offline
pdfjsLib.GlobalWorkerOptions.workerSrc = '/pdf.worker.min.js'

export { pdfjsLib }
