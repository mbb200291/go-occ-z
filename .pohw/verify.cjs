#!/usr/bin/env node
'use strict';
const fs = require('node:fs');
const path = require('node:path');
const { codeManifestForCommit, verifySignedReceipt, verifiedBadgeSummary } = require('./proof.js');
try {
  const root = path.resolve(process.argv[2] || '.');
  const receiptPath = process.argv[3];
  if (!receiptPath) throw new Error('Usage: node .pohw/verify.cjs <repository> <receipt.json> [summary.json]');
  const receipt = JSON.parse(fs.readFileSync(receiptPath, 'utf8'));
  const publicKey = fs.readFileSync(path.join(root, '.pohw', 'public-key.pem'), 'utf8');
  const current = require('node:child_process').execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim();
  if (receipt.body.targetCommit !== current) throw new Error('Receipt target commit does not match checked-out Git HEAD');
  verifySignedReceipt(receipt, publicKey, codeManifestForCommit(root, current));
  const summary = verifiedBadgeSummary(receipt, current);
  if (process.argv[4]) fs.writeFileSync(process.argv[4], JSON.stringify(summary, null, 2) + '\n');
  console.log(`PoHW receipt verified; source commit=${current}; assurance=self-attested`);
} catch (error) { console.error(`PoHW verification FAILED: ${error.message}`); process.exitCode = 1; }
