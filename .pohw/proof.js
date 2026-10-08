"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.RECEIPT_VERSION = void 0;
exports.canonicalJson = canonicalJson;
exports.codeManifest = codeManifest;
exports.keyFingerprint = keyFingerprint;
exports.createLocalKeys = createLocalKeys;
exports.issueReceipt = issueReceipt;
exports.verifySignedReceipt = verifySignedReceipt;
exports.verifiedBadgeSummary = verifiedBadgeSummary;
exports.badgeMarkdown = badgeMarkdown;
exports.updateBadgeReadme = updateBadgeReadme;
/** PoHW portable self-attested receipt v1. No external npm runtime dependencies. */
const node_crypto_1 = require("node:crypto");
const node_child_process_1 = require("node:child_process");
const fs = __importStar(require("node:fs"));
const path = __importStar(require("node:path"));
exports.RECEIPT_VERSION = 'pohw-editing-receipt/v1';
const EXCLUDED = new Set(['.git', '.pohw', 'node_modules', 'dist', 'out', 'build', 'coverage', '.venv', 'venv']);
const EXTENSIONS = new Set(['.go', '.ts', '.tsx', '.js', '.jsx', '.py', '.rs', '.java', '.c', '.h', '.cpp', '.hpp', '.cs', '.rb', '.php', '.swift', '.kt', '.kts', '.sh', '.sql', '.vue', '.svelte', '.html', '.css', '.json', '.yaml', '.yml', '.toml', '.md', '.xml', '.proto']);
const sha256 = (data) => (0, node_crypto_1.createHash)('sha256').update(data).digest('hex');
/** Recursive key ordering prevents signatures depending on JSON field insertion order. */
function canonicalJson(value) {
    if (value === null || typeof value !== 'object')
        return JSON.stringify(value);
    if (Array.isArray(value))
        return `[${value.map(canonicalJson).join(',')}]`;
    const obj = value;
    return `{${Object.keys(obj).sort().map(k => `${JSON.stringify(k)}:${canonicalJson(obj[k])}`).join(',')}}`;
}
function eligible(relative) {
    const normalized = relative.replace(/\\/g, '/');
    const chunks = normalized.split('/');
    if (chunks.some(x => EXCLUDED.has(x)) || normalized.startsWith('/') || chunks.includes('..'))
        return false;
    return EXTENSIONS.has(path.posix.extname(normalized).toLowerCase());
}
/** Include committed/indexed and not-yet-added files; require identical code contents at CI checkout. */
function codeManifest(root) {
    const output = (0, node_child_process_1.execFileSync)('git', ['ls-files', '-z', '--cached', '--others', '--exclude-standard'], {
        cwd: root, timeout: 15_000, maxBuffer: 12 * 1024 * 1024
    });
    const names = [...new Set(output.toString('utf8').split('\0').filter(Boolean))].filter(eligible).sort();
    if (names.length > 20000)
        throw new Error('PoHW file limit exceeded (20000)');
    const files = [];
    for (const name of names) {
        const absolute = path.join(root, name);
        const stat = fs.lstatSync(absolute);
        if (!stat.isFile())
            throw new Error(`PoHW excludes symlinks and non-regular source files: ${name}`);
        if (stat.size > 2 * 1024 * 1024)
            throw new Error(`PoHW source file too large: ${name}`);
        files.push({ path: name.replace(/\\/g, '/'), sha256: sha256(fs.readFileSync(absolute)) });
    }
    return { algorithm: 'pohw-code-files-sha256-v1', rootHash: sha256(canonicalJson(files)), files };
}
function keyFingerprint(publicKeyPem) {
    const der = (0, node_crypto_1.createPublicKey)(publicKeyPem).export({ type: 'spki', format: 'der' });
    return sha256(der);
}
function createLocalKeys() {
    const keys = (0, node_crypto_1.generateKeyPairSync)('ed25519');
    return {
        privateKeyPem: keys.privateKey.export({ type: 'pkcs8', format: 'pem' }).toString(),
        publicKeyPem: keys.publicKey.export({ type: 'spki', format: 'pem' }).toString()
    };
}
function issueReceipt(body, privateKeyPem) {
    const signature = (0, node_crypto_1.sign)(null, Buffer.from(canonicalJson(body)), (0, node_crypto_1.createPrivateKey)(privateKeyPem)).toString('base64');
    return { body, signature };
}
function validPercent(x) { return typeof x === 'number' && Number.isFinite(x) && x >= 0 && x <= 100; }
function verifySignedReceipt(receipt, publicKeyPem, manifest) {
    if (!receipt || !receipt.body || typeof receipt.signature !== 'string')
        throw new Error('Malformed PoHW receipt');
    const body = receipt.body;
    if (body.schema !== exports.RECEIPT_VERSION || body.claim !== 'self-attested-editor-behavior')
        throw new Error('Unsupported PoHW receipt claim/schema');
    if (body.keyId !== keyFingerprint(publicKeyPem))
        throw new Error('Receipt signer differs from registered public key');
    if (body.code.algorithm !== manifest.algorithm || body.code.rootHash !== manifest.rootHash || body.code.files !== manifest.files.length) {
        throw new Error('Code content does not match signed receipt (rerun PoHW Publish Signed Receipt)');
    }
    const m = body.metrics;
    if (!m || m.algorithm !== 'rule-based-v0.1' || !Number.isSafeInteger(m.units) || !Number.isSafeInteger(m.monitoredUnits) ||
        m.units < 0 || m.monitoredUnits < 0 || m.monitoredUnits > m.units ||
        !validPercent(m.coveragePercent) || (m.scorePercent !== null && !validPercent(m.scorePercent)) ||
        (m.monitoredUnits === 0 && m.scorePercent !== null) ||
        (m.monitoredUnits > 0 && m.scorePercent === null) ||
        Math.abs(m.coveragePercent - (m.units ? m.monitoredUnits / m.units * 100 : 0)) > 0.011) {
        throw new Error('Receipt metrics are inconsistent');
    }
    if (!/^[a-f0-9]{64}$/.test(body.session?.auditTip ?? '') || !Number.isFinite(Date.parse(body.issuedAt)))
        throw new Error('Invalid receipt metadata');
    let ok = false;
    try {
        ok = (0, node_crypto_1.verify)(null, Buffer.from(canonicalJson(body)), (0, node_crypto_1.createPublicKey)(publicKeyPem), Buffer.from(receipt.signature, 'base64'));
    }
    catch { /* reject */ }
    if (!ok)
        throw new Error('Receipt signature verification failed');
}
/** This is only ever published after the signed receipt was independently checked. */
function verifiedBadgeSummary(receipt, sourceCommit) {
    return {
        verification: 'signature-and-content-valid',
        assurance: 'self-attested',
        humanScore: receipt.body.metrics.scorePercent === null ? 'N/A' : receipt.body.metrics.scorePercent,
        monitoredCoverage: receipt.body.metrics.coveragePercent,
        sourceCommit,
        sourceCodeHash: receipt.body.code.rootHash,
        issuedAt: receipt.body.issuedAt
    };
}
/** Badge initialization may be run repeatedly without duplicating the README block. */
function badgeMarkdown(owner, repo, defaultBranch) {
    const base = `https://raw.githubusercontent.com/${owner}/${repo}/pohw-badges/summary.json`;
    const dynamic = (field, label, color) => `https://img.shields.io/badge/dynamic/json?url=${encodeURIComponent(base)}&query=${encodeURIComponent('$.' + field)}&suffix=%25&label=${encodeURIComponent(label)}&color=${color}`;
    return [
        '<!-- pohw-badges:start -->',
        `[![PoHW Editing Score](${dynamic('humanScore', 'PoHW Editing Score', 'blue')})](https://github.com/${owner}/${repo}/actions/workflows/pohw-verify.yml)`,
        `[![PoHW Monitored](${dynamic('monitoredCoverage', 'PoHW Monitored', 'informational')})](https://github.com/${owner}/${repo}/actions/workflows/pohw-verify.yml)`,
        `[![PoHW Receipt Verification](https://github.com/${owner}/${repo}/actions/workflows/pohw-verify.yml/badge.svg?branch=${encodeURIComponent(defaultBranch)})](https://github.com/${owner}/${repo}/actions/workflows/pohw-verify.yml)`,
        '<!-- pohw-badges:end -->'
    ].join('\n');
}
function updateBadgeReadme(readme, badges) {
    const start = '<!-- pohw-badges:start -->', end = '<!-- pohw-badges:end -->';
    const a = readme.indexOf(start), b = readme.indexOf(end);
    if ((a >= 0) !== (b >= 0) || (a >= 0 && b < a))
        throw new Error('Malformed PoHW README badge markers');
    if (a >= 0)
        return readme.slice(0, a) + badges + readme.slice(b + end.length);
    return `${badges}\n\n${readme}`;
}
