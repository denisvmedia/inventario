import { startStack } from './setup-stack.js';
import { FullConfig } from '@playwright/test';
import waitOn from 'wait-on';

async function globalSetup(config: FullConfig) {
  // Start the stack first
  if (process.env.START_STACK !== 'false') {
    console.log('Starting the stack for E2E tests...');
    await startStack();
  }

  // Wait for the frontend to be available
  await waitOn({
    resources: ['http://localhost:5173'],
    delay: 100, // minimum delay before starting (ms)
    interval: 250, // interval between attempts
    timeout: 30000, // maximum wait time (ms)
    tcpTimeout: 1000, // timeout for a single TCP connection
    window: 1000 // how many successful checks in a row are required
  });
}

export default globalSetup;
