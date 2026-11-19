const capability = process.argv[2] || 'unknown';
console.error('PENDING: ' + capability + ' belongs to a later Task and has no completed integration environment. This is NOT a passing test.');
process.exitCode = 2;
