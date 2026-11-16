export const environment = {
  production: true,
  // Same origin: nginx in the frontend container proxies /api/* to the backend.
  apiUrl: '/api'
};
