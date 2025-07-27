import { startStack } from './setup-stack.js';
import { FullConfig } from '@playwright/test';
import waitOn from 'wait-on';

async function globalSetup(config: FullConfig) {
  console.log('Starting e2e test environment with PostgreSQL...');
  console.log('Environment variables:');
  console.log('- CI:', process.env.CI);
  console.log('- GITHUB_ACTIONS:', process.env.GITHUB_ACTIONS);
  console.log('- E2E_POSTGRES_PORT:', process.env.E2E_POSTGRES_PORT);

  // Always start the stack for e2e tests to ensure PostgreSQL is running
  // This ensures we have a consistent postgres-centric environment
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

  console.log('E2E test environment ready with PostgreSQL backend');
}

export default globalSetup;
