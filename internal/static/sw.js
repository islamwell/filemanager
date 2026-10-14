// NRQ Progressive Web App (PWA) Service Worker
const CACHE_NAME = 'nrq-cache-v1.0.17';

const STATIC_ASSETS = [
  '/',
  '/offline',
  '/static/css/style.css',
  '/static/js/app.js',
  '/static/img/nq-logo.png',
  '/static/img/nq-favicon.png',
  '/static/img/icon.svg',
  '/manifest.json'
];

// Install: Cache critical app shell and offline fallback page
self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(CACHE_NAME).then((cache) => {
      return cache.addAll(STATIC_ASSETS);
    }).then(() => {
      return self.skipWaiting();
    })
  );
});

// Activate: Prune outdated caches and take control immediately
self.addEventListener('activate', (event) => {
  event.waitUntil(
    caches.keys().then((keys) => {
      return Promise.all(
        keys.map((key) => {
          if (key !== CACHE_NAME) {
            return caches.delete(key);
          }
        })
      );
    }).then(() => {
      return self.clients.claim();
    })
  );
});

// Fetch: Strategy based on request type
self.addEventListener('fetch', (event) => {
  const request = event.request;

  // Only handle GET requests
  if (request.method !== 'GET') {
    return;
  }

  const url = new URL(request.url);

  // Bypass audio streaming, large downloads, or range requests to prevent buffering bugs
  if (url.pathname.includes('/audio') ||
      url.pathname.includes('/download') ||
      url.pathname.includes('/view') ||
      request.headers.get('range')) {
    return;
  }

  // HTML Page Navigation: Network-First with Offline Fallback
  if (request.mode === 'navigate') {
    event.respondWith(
      fetch(request)
        .then((response) => {
          // If valid server response, cache a clone for offline navigation
          if (response && response.status === 200) {
            const copy = response.clone();
            caches.open(CACHE_NAME).then((cache) => {
              cache.put(request, copy);
            });
          }
          return response;
        })
        .catch(() => {
          // Offline fallback: try exact cached page, or render dedicated offline page
          return caches.match(request).then((cached) => {
            if (cached) {
              return cached;
            }
            return caches.match('/offline');
          });
        })
    );
    return;
  }

  // Static Assets (CSS, JS, SVG, Manifest): Stale-While-Revalidate
  if (url.pathname.startsWith('/static/') || url.pathname === '/manifest.json') {
    event.respondWith(
      caches.match(request).then((cached) => {
        const fetchPromise = fetch(request).then((networkResponse) => {
          if (networkResponse && networkResponse.status === 200) {
            const copy = networkResponse.clone();
            caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
          }
          return networkResponse;
        }).catch(() => {
          // Network failed, nothing to do if already cached
        });

        return cached || fetchPromise;
      })
    );
    return;
  }

  // Default GET requests: Network first with Cache fallback
  event.respondWith(
    fetch(request)
      .then((networkResponse) => {
        if (networkResponse && networkResponse.status === 200) {
          const copy = networkResponse.clone();
          caches.open(CACHE_NAME).then((cache) => cache.put(request, copy));
        }
        return networkResponse;
      })
      .catch(() => {
        return caches.match(request);
      })
  );
});
