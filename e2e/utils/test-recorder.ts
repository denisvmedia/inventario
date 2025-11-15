import { Page } from '@playwright/test';
import path from 'path';
import fs from 'fs';

/**
 * Helper class for taking screenshots and managing test artifacts
 */
export class TestRecorder {
  private page: Page;
  private testName: string;
  private screenshotCounter: number = 0;
  private screenshotsDir: string;
  private videosDir: string;

  /**
   * Create a new TestRecorder
   *
   * @param page Playwright page object
   * @param testName Name of the test (used for file naming)
   */
  constructor(page: Page, testName: string) {
    this.page = page;
    this.testName = testName.replace(/\s+/g, '-').toLowerCase();
    this.screenshotsDir = path.join('test-results', 'screenshots');
    this.videosDir = path.join('test-results', 'videos');

    // Ensure directories exist
    this.ensureDirectoryExists(this.screenshotsDir);
    this.ensureDirectoryExists(this.videosDir);
  }

  /**
   * Take a screenshot with auto-incrementing counter
   *
   * @param name Optional name to include in the filename
   * @param fullPage Whether to take a full page screenshot (default: true)
   * @returns Path to the saved screenshot
   */
  async takeScreenshot(name?: string, fullPage: boolean = true): Promise<string> {
    const counter = String(++this.screenshotCounter).padStart(2, '0');
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
    const filename = name
      ? `${this.testName}-${name}-${counter}-${timestamp}.png`
      : `${this.testName}-${counter}-${timestamp}.png`;

    const filePath = path.join(this.screenshotsDir, filename);

    await this.page.screenshot({
      path: filePath,
      fullPage
    });

    return filePath;
  }

  /**
   * Take a screenshot of a specific element
   *
   * @param selector CSS selector for the element
   * @param name Optional name to include in the filename
   * @param index Optional index to specify which element to capture if multiple match (default: 0)
   * @returns Path to the saved screenshot
   */
  async takeElementScreenshot(selector: string, name?: string, index: number = 0): Promise<string> {
    const counter = String(++this.screenshotCounter).padStart(2, '0');
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
    const elementName = name || selector.replace(/[^a-zA-Z0-9]/g, '-');
    const filename = `${this.testName}-element-${elementName}-${counter}-${timestamp}.png`;
    const filePath = path.join(this.screenshotsDir, filename);

    const locator = this.page.locator(selector);
    const count = await locator.count();

    if (count === 0) {
      throw new Error(`Selector "${selector}" matched 0 elements, expected at least one.`);
    }

    if (index >= count) {
      throw new Error(`Selector "${selector}" matched ${count} elements, but index ${index} is out of bounds.`);
    }

    await locator.nth(index).screenshot({ path: filePath });

    return filePath;
  }

  /**
   * Ensure a directory exists
   *
   * @param dir Directory path
   */
  private ensureDirectoryExists(dir: string): void {
    if (!fs.existsSync(dir)) {
      fs.mkdirSync(dir, { recursive: true });
    }
  }

  /**
   * Log a message with the test name prefix
   * This helps identify which test is logging when tests run in parallel
   *
   * @param message The message to log
   * @param args Additional arguments to log
   */
  log(message: string, ...args: any[]): void {
    console.log(`[${this.testName}] ${message}`, ...args);
  }

  /**
   * Log a warning message with the test name prefix
   *
   * @param message The message to log
   * @param args Additional arguments to log
   */
  warn(message: string, ...args: any[]): void {
    console.warn(`[${this.testName}] ${message}`, ...args);
  }

  /**
   * Log an error message with the test name prefix
   *
   * @param message The message to log
   * @param args Additional arguments to log
   */
  error(message: string, ...args: any[]): void {
    console.error(`[${this.testName}] ${message}`, ...args);
  }
}

/**
 * Helper function to log with optional recorder
 * If recorder is provided, uses it; otherwise falls back to console.log
 */
export function log(recorder: TestRecorder | undefined, message: string, ...args: any[]): void {
  if (recorder) {
    recorder.log(message, ...args);
  } else {
    console.log(message, ...args);
  }
}

/**
 * Helper function to log warnings with optional recorder
 * If recorder is provided, uses it; otherwise falls back to console.warn
 */
export function warn(recorder: TestRecorder | undefined, message: string, ...args: any[]): void {
  if (recorder) {
    recorder.warn(message, ...args);
  } else {
    console.warn(message, ...args);
  }
}

/**
 * Helper function to log errors with optional recorder
 * If recorder is provided, uses it; otherwise falls back to console.error
 */
export function error(recorder: TestRecorder | undefined, message: string, ...args: any[]): void {
  if (recorder) {
    recorder.error(message, ...args);
  } else {
    console.error(message, ...args);
  }
}
