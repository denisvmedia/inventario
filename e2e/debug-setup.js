#!/usr/bin/env node

// Debug script to test PostgreSQL setup
import { startPostgres, stopPostgres, resetDatabase } from './setup/setup-stack.js';

async function debugSetup() {
  console.log('=== PostgreSQL Setup Debug ===');
  
  try {
    console.log('1. Starting PostgreSQL...');
    await startPostgres();
    console.log('✅ PostgreSQL started successfully');
    
    console.log('2. Testing database reset...');
    await resetDatabase();
    console.log('✅ Database reset successful');
    
    console.log('3. Stopping PostgreSQL...');
    await stopPostgres();
    console.log('✅ PostgreSQL stopped successfully');
    
    console.log('🎉 All tests passed!');
  } catch (error) {
    console.error('❌ Error:', error.message);
    console.error('Stack:', error.stack);
    
    // Cleanup on error
    try {
      await stopPostgres();
    } catch (cleanupError) {
      console.error('Cleanup error:', cleanupError.message);
    }
    
    process.exit(1);
  }
}

debugSetup();
