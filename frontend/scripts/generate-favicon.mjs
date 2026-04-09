import sharp from 'sharp'
import { readFileSync } from 'fs'
import { resolve, dirname } from 'path'
import { fileURLToPath } from 'url'

const __dirname = dirname(fileURLToPath(import.meta.url))
const svgPath = resolve(__dirname, '../public/favicon.svg')
const svg = readFileSync(svgPath)

// 32x32 for browser tab / search engines
await sharp(svg).resize(32, 32).png().toFile(resolve(__dirname, '../public/favicon-32.png'))

// 192x192 for Android / PWA
await sharp(svg).resize(192, 192).png().toFile(resolve(__dirname, '../public/favicon-192.png'))

// 180x180 for Apple touch icon
await sharp(svg).resize(180, 180).png().toFile(resolve(__dirname, '../public/apple-touch-icon.png'))

// 48x48 minimum required by Google Search
await sharp(svg).resize(48, 48).png().toFile(resolve(__dirname, '../public/favicon-48.png'))

console.log('Favicon PNG files generated successfully.')
