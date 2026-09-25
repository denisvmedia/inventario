/**
 * Fail when a Playwright run skipped tests, or ran fewer than it should.
 *
 * A skipped test reports green, so a spec that quietly rejoins the skipped
 * set looks exactly like a passing one. The OAuth lane exists because that
 * had already happened (#2634); this is what keeps it from happening again.
 *
 *   node scripts/assert-report.mjs <report.json> <minimum-expected>
 */
import { readFileSync } from "node:fs";

const [reportPath, minimumRaw] = process.argv.slice(2);
if (!reportPath || !minimumRaw) {
  console.error("usage: assert-report.mjs <report.json> <minimum-expected>");
  process.exit(2);
}

const minimum = Number(minimumRaw);
if (!Number.isInteger(minimum) || minimum < 1) {
  console.error(`::error::Not a usable minimum: ${minimumRaw}`);
  process.exit(2);
}

let stats;
try {
  ({ stats } = JSON.parse(readFileSync(reportPath, "utf8")));
} catch (error) {
  console.error(`::error::Could not read ${reportPath}: ${error.message}`);
  console.error("The suite did not run, so there is nothing to report on.");
  process.exit(1);
}

const expected = stats?.expected ?? 0;
const skipped = stats?.skipped ?? 0;
console.log(`expected=${expected} skipped=${skipped}`);

if (skipped > 0) {
  console.error(
    `::error::${skipped} test(s) self-skipped. This lane exists so they cannot.`,
  );
  process.exit(1);
}

if (expected < minimum) {
  console.error(
    `::error::Only ${expected} test(s) ran; at least ${minimum} were expected.`,
  );
  process.exit(1);
}
