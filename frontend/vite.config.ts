import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    // ponytail: middleware returns 404 for /cache/ paths during dev — unnecessary, dev server won't serve those.
    {
      name: 'wails-cache-bypass',
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          if (req.url && req.url.startsWith('/cache/')) {
            res.statusCode = 404;
            res.end('Not Found');
            return;
          }
          next();
        });
      }
    }
  ]
})
