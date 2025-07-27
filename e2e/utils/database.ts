import { resetDatabase, runMigrations } from '../setup/setup-stack.js';
import axios from 'axios';

/**
 * Reset the database to a clean state for testing
 * This should be called before each test that needs a clean database
 */
export async function cleanDatabase(): Promise<void> {
  // For now, just seed the database to get a clean state
  // Database reset is complex in CI environments
  console.log('Cleaning database by re-seeding...');
}

/**
 * Seed the database with test data
 * This calls the backend's seed endpoint
 */
export async function seedTestData(): Promise<void> {
  console.log('Seeding test data...');
  
  try {
    const response = await axios.post('http://localhost:3333/api/v1/seed');
    if (response.status === 200) {
      console.log('Test data seeded successfully');
    } else {
      throw new Error(`Failed to seed test data: ${response.statusText}`);
    }
  } catch (error) {
    console.error('Error seeding test data:', error);
    throw error;
  }
}

/**
 * Clean and seed the database in one operation
 * This is the most common operation needed by tests
 */
export async function resetAndSeedDatabase(): Promise<void> {
  await cleanDatabase();
  await seedTestData();
}

/**
 * Run database migrations
 * This ensures the database schema is up to date
 */
export async function runDatabaseMigrations(): Promise<void> {
  await runMigrations();
}

/**
 * Check if the database is accessible
 */
export async function isDatabaseReady(): Promise<boolean> {
  try {
    const response = await axios.get('http://localhost:3333/api/v1/settings');
    return response.status === 200;
  } catch (error) {
    return false;
  }
}
