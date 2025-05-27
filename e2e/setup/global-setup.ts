import { startStack } from './setup-stack.js';
import { FullConfig } from '@playwright/test';
import waitOn from 'wait-on';

async function globalSetup(config: FullConfig) {
  await waitOn({
    resources: ['http://localhost:5173'],
    delay: 100, // minimum delay before starting (ms)
    interval: 250, // interval between attempts
    timeout: 30000, // maximum wait time (ms)
    tcpTimeout: 1000, // timeout for a single TCP connection
    window: 1000 // how many successful checks in a row are required
  });

  // Only start the stack if it's not already running
  // This is useful for local development where you might want to start the stack manually
  if (process.env.START_STACK === 'true') {
    await startStack();
  }
}

export default globalSetup;
