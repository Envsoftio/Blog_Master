#!/usr/bin/env node
import { createHash } from 'node:crypto'
import { readdirSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import { join, relative, sep } from 'node:path'
import process from 'node:process'

const schemaVersion = 'seoblog-release-v1'
const manifestName = 'release.json'

const requiredArtifacts = [
  'REVISION',
  'admin/.output/server/index.mjs',
  'backend/admincli',
  'backend/api',
  'backend/worker',
  'contracts/openapi/openapi.yaml',
  'ecosystem.config.cjs',
  'packages/content-client/dist/index.d.ts',
  'packages/content-client/dist/index.js',
  'packages/content-client/package.json',
  'scripts/release-manifest.mjs'
]

function usage() {
  console.error('usage: node scripts/release-manifest.mjs generate|verify|field <release-root> [expected-release-id|field-name]')
  process.exit(2)
}

function fail(message) {
  console.error(`[release-manifest] ERROR: ${message}`)
  process.exit(1)
}

function normalizePath(path) {
  return path.split(sep).join('/')
}

function walk(root, dir = root) {
  const entries = readdirSync(dir, { withFileTypes: true })
  const files = []
  for (const entry of entries) {
    const fullPath = join(dir, entry.name)
    const artifactPath = normalizePath(relative(root, fullPath))
    if (artifactPath === manifestName) {
      continue
    }
    if (entry.isDirectory()) {
      files.push(...walk(root, fullPath))
    } else if (entry.isFile()) {
      files.push(artifactPath)
    }
  }
  return files.sort((a, b) => a.localeCompare(b))
}

function readManifest(root) {
  try {
    return JSON.parse(readFileSync(join(root, manifestName), 'utf8'))
  } catch (error) {
    fail(`could not read ${manifestName}: ${error.message}`)
  }
}

function sha256File(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex')
}

function artifactRecord(root, artifactPath) {
  const fullPath = join(root, artifactPath)
  const stats = statSync(fullPath)
  return {
    path: artifactPath,
    sha256: sha256File(fullPath),
    bytes: stats.size
  }
}

function validateReleaseId(releaseId) {
  if (typeof releaseId !== 'string' || !/^[A-Za-z0-9._-]{1,128}$/.test(releaseId)) {
    fail('releaseId must be 1-128 characters of A-Z, a-z, 0-9, dot, underscore or dash')
  }
}

function validateArtifactPath(artifactPath) {
  if (
    typeof artifactPath !== 'string' ||
    artifactPath === '' ||
    artifactPath.startsWith('/') ||
    artifactPath.split('/').includes('..') ||
    artifactPath === manifestName
  ) {
    fail(`invalid artifact path in manifest: ${artifactPath}`)
  }
}

function ensureRequired(paths) {
  const available = new Set(paths)
  for (const required of requiredArtifacts) {
    if (!available.has(required)) {
      fail(`release archive is missing required artifact: ${required}`)
    }
  }
}

function generate(root) {
  const releaseId = process.env.RELEASE_ID || ''
  validateReleaseId(releaseId)
  const commitSha = process.env.GITHUB_SHA || process.env.COMMIT_SHA || ''
  if (commitSha === '') {
    fail('GITHUB_SHA or COMMIT_SHA is required when generating a release manifest')
  }

  const files = walk(root)
  ensureRequired(files)
  const manifest = {
    schemaVersion,
    releaseId,
    commitSha,
    builtAt: new Date().toISOString(),
    target: {
      goos: process.env.TARGET_GOOS || 'linux',
      goarch: process.env.TARGET_GOARCH || ''
    },
    artifacts: files.map((path) => artifactRecord(root, path))
  }

  writeFileSync(join(root, manifestName), `${JSON.stringify(manifest, null, 2)}\n`)
  console.log(`[release-manifest] wrote ${join(root, manifestName)} with ${manifest.artifacts.length} artifacts`)
}

function verify(root, expectedReleaseId = '') {
  const manifest = readManifest(root)
  if (manifest.schemaVersion !== schemaVersion) {
    fail(`unsupported manifest schema: ${manifest.schemaVersion}`)
  }
  validateReleaseId(manifest.releaseId)
  if (expectedReleaseId !== '' && manifest.releaseId !== expectedReleaseId) {
    fail(`releaseId ${manifest.releaseId} does not match expected ${expectedReleaseId}`)
  }
  if (!Array.isArray(manifest.artifacts) || manifest.artifacts.length === 0) {
    fail('manifest artifacts must be a non-empty array')
  }

  const actualPaths = walk(root)
  ensureRequired(actualPaths)

  const manifestPaths = new Set()
  for (const artifact of manifest.artifacts) {
    validateArtifactPath(artifact.path)
    if (manifestPaths.has(artifact.path)) {
      fail(`duplicate artifact path in manifest: ${artifact.path}`)
    }
    manifestPaths.add(artifact.path)
    if (!/^[0-9a-f]{64}$/.test(artifact.sha256)) {
      fail(`invalid sha256 for ${artifact.path}`)
    }
    const fullPath = join(root, artifact.path)
    const stats = statSync(fullPath)
    if (stats.size !== artifact.bytes) {
      fail(`size mismatch for ${artifact.path}`)
    }
    const actualHash = sha256File(fullPath)
    if (actualHash !== artifact.sha256) {
      fail(`checksum mismatch for ${artifact.path}`)
    }
  }

  const extras = actualPaths.filter((path) => !manifestPaths.has(path))
  const missing = [...manifestPaths].filter((path) => !actualPaths.includes(path))
  if (extras.length > 0) {
    fail(`release contains files not listed in manifest: ${extras.join(', ')}`)
  }
  if (missing.length > 0) {
    fail(`manifest lists files missing from release: ${missing.join(', ')}`)
  }

  console.log(`[release-manifest] verified ${manifest.releaseId} (${manifest.artifacts.length} artifacts)`)
}

function field(root, name) {
  const manifest = readManifest(root)
  if (name === 'releaseId') {
    validateReleaseId(manifest.releaseId)
    console.log(manifest.releaseId)
    return
  }
  if (name === 'commitSha') {
    console.log(manifest.commitSha || '')
    return
  }
  fail(`unsupported manifest field: ${name}`)
}

const [command, root, third] = process.argv.slice(2)
if (!command || !root) {
  usage()
}

switch (command) {
  case 'generate':
    generate(root)
    break
  case 'verify':
    verify(root, third || '')
    break
  case 'field':
    if (!third) {
      usage()
    }
    field(root, third)
    break
  default:
    usage()
}
