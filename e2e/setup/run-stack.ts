import { startStack, stopStack } from './setup-stack';

async function main() {
  try {
    await startStack();
    console.log('Stack is running. Press Ctrl+C to stop.');
    
    // Keep the process running
    process.stdin.resume();
    
    // Handle Ctrl+C
    process.on('SIGINT', async () => {
      console.log('Stopping stack...');
      await stopStack();
      process.exit(0);
    });
  } catch (error) {
    console.error('Failed to start stack:', error);
    await stopStack();
    process.exit(1);
  }
}

main();
