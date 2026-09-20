import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'

// boards serve --dev sets TYPOLOGY_VIEWER_PUBLIC to the XDG (or --viewer) boards dir.
const publicDir = process.env.TYPOLOGY_VIEWER_PUBLIC
  ? path.resolve(process.env.TYPOLOGY_VIEWER_PUBLIC)
  : 'public'

export default defineConfig({
  plugins: [react()],
  publicDir,
  server: {
    port: 5173,
    open: false,
  },
})
