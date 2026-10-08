#!/usr/bin/env node
'use strict';
const fs = require('node:fs');
const path = require('node:path');
const { codeManifest, verifySignedReceipt, verifiedBadgeSummary } = require('./proof.js');
try {
  const root = path.resolve(process.argv[2] || '.');
  const receipt = JSON.parse(fs.readFileSync(path.join(root, '.pohw', 'receipt.json'), 'utf8'));
  const publicKey = fs.readFileSync(path.join(root, '.pohw', 'public-key.pem'), 'utf8');
  verifySignedReceipt(receipt, publicKey, codeManifest(root));
  const sha = process.env.GITHUB_SHA || 'local-verification';
  const result = verifiedBadgeSummary(receipt, sha);
  const outputPath = process.argv[3];
  if (outputPath) fs.writeFileSync(outputPath, JSON.stringify(result, null, 2) + '\n');
  console.log(`PoHW receipt verified; source content matches; assurance=self-attested; commit=${sha}`);
} catch (error) {
  console.error(`PoHW verification FAILED: ${error.message}`);
  process.exitCode = 1;
}
