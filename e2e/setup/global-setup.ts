import { startStack } from './setup-stack.js';
import { FullConfig } from '@playwright/test';
import waitOn from 'wait-on';

async function globalSetup(config: FullConfig) {
  console.log('Starting e2e test environment...');
  console.log('Environment variables:');
  console.log('- CI:', process.env.CI);
  console.log('- GITHUB_ACTIONS:', process.env.GITHUB_ACTIONS);
  console.log('- INVENTARIO_DB_DSN:', process.env.INVENTARIO_DB_DSN ? 'set' : 'not set');

  // Check if we're using external database (CI) or need to start our own
  if (process.env.INVENTARIO_DB_DSN) {
    console.log('Using external database, skipping PostgreSQL container startup');
  } else {
    console.log('No external database configured, will start PostgreSQL container');
  }

  // Always start the stack - it will handle PostgreSQL appropriately
  await startStack();

  // Wait for both frontend and backend to be ready
  await waitOn({
    resources: [
      'http://localhost:5173', // Frontend
      'http://localhost:3333'  // Backend
    ],
    delay: 100, // minimum delay before starting (ms)
    interval: 250, // interval between attempts
    timeout: 60000, // maximum wait time (ms) - increased for container startup
    tcpTimeout: 1000, // timeout for a single TCP connection
    window: 1000 // how many successful checks in a row are required
  });

  console.log('E2E test environment ready');
}

export default globalSetup;
