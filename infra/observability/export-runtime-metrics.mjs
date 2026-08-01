#!/usr/bin/env node

import { execFileSync } from 'node:child_process'
import { mkdirSync, readFileSync, renameSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'

const deployPath = resolve(process.env.SEOBLOG_DEPLOY_PATH || '/srv/seoblog')
const outputPath = resolve(process.env.SEOBLOG_METRICS_TEXTFILE || join(deployPath, 'shared/metrics/seoblog.prom'))
const pm2Home = resolve(process.env.SEOBLOG_PM2_HOME || join(process.env.USERPROFILE || process.env.HOME || '/nonexistent', '.pm2'))
const pm2Binary = process.env.SEOBLOG_PM2_BIN || 'pm2'
const expectedNames = ['seoblog-admin', 'seoblog-api', 'seoblog-worker']
const lines = [
  '# HELP seoblog_observability_export_success Whether the host runtime observation completed.',
  '# TYPE seoblog_observability_export_success gauge',
]

function labels(values) {
  return `{${Object.entries(values).sort(([a], [b]) => a.localeCompare(b)).map(([key, value]) => `${key}="${String(value).replaceAll('\\', '\\\\').replaceAll('"', '\\"')}"`).join(',')}}`
}

function lastJSONRecord(path) {
  try {
    const records = readFileSync(path, 'utf8').trim().split('\n').filter(Boolean)
    return records.length ? JSON.parse(records.at(-1)) : null
  } catch {
    return null
  }
}

let exportSuccess = 1
let processes = []
try {
  processes = JSON.parse(execFileSync(pm2Binary, ['jlist'], { encoding: 'utf8', timeout: 10_000 }))
} catch {
  exportSuccess = 0
}

let savedNames = new Set()
try {
  const saved = JSON.parse(readFileSync(join(pm2Home, 'dump.pm2'), 'utf8'))
  savedNames = new Set(saved.map((process) => process.name))
} catch {
  exportSuccess = 0
}

lines.push('# HELP seoblog_pm2_process_up Whether each required PM2 process is online.')
lines.push('# TYPE seoblog_pm2_process_up gauge')
lines.push('# HELP seoblog_pm2_process_restarts_total PM2 restart counter for each required process.')
lines.push('# TYPE seoblog_pm2_process_restarts_total gauge')
lines.push('# HELP seoblog_pm2_process_memory_bytes Resident memory reported by PM2.')
lines.push('# TYPE seoblog_pm2_process_memory_bytes gauge')
lines.push('# HELP seoblog_pm2_saved_process_present Whether the required process is in PM2 dump.pm2.')
lines.push('# TYPE seoblog_pm2_saved_process_present gauge')
for (const name of expectedNames) {
  const process = processes.find((candidate) => candidate.name === name)
  const processLabels = labels({ name })
  lines.push(`seoblog_pm2_process_up${processLabels} ${process?.pm2_env?.status === 'online' ? 1 : 0}`)
  lines.push(`seoblog_pm2_process_restarts_total${processLabels} ${Number(process?.pm2_env?.restart_time || 0)}`)
  lines.push(`seoblog_pm2_process_memory_bytes${processLabels} ${Number(process?.monit?.memory || 0)}`)
  lines.push(`seoblog_pm2_saved_process_present${processLabels} ${savedNames.has(name) ? 1 : 0}`)
}

const recovery = lastJSONRecord(join(deployPath, 'shared/backup-evidence/recovery-points.jsonl'))
const verifiedAt = Date.parse(recovery?.verifiedAt || '')
lines.push('# HELP seoblog_backup_last_verified_timestamp_seconds Unix timestamp of the last independently verified recovery point.')
lines.push('# TYPE seoblog_backup_last_verified_timestamp_seconds gauge')
lines.push(`seoblog_backup_last_verified_timestamp_seconds ${Number.isFinite(verifiedAt) ? verifiedAt / 1000 : 0}`)
lines.push('# HELP seoblog_backup_last_verification_success Whether the latest recovery-point evidence passed all checks.')
lines.push('# TYPE seoblog_backup_last_verification_success gauge')
const backupHealthy = recovery?.checks?.downloadedChecksum === true && recovery?.checks?.quickCheck === 'ok' && recovery?.checks?.foreignKeyCheck === 'ok'
lines.push(`seoblog_backup_last_verification_success ${backupHealthy ? 1 : 0}`)

const deployment = lastJSONRecord(join(deployPath, 'shared/deployments.jsonl'))
const deploymentAt = Date.parse(deployment?.recordedAt || '')
lines.push('# HELP seoblog_deployment_last_status Last recorded deployment status; one series is emitted for the current status.')
lines.push('# TYPE seoblog_deployment_last_status gauge')
lines.push(`seoblog_deployment_last_status${labels({ status: deployment?.status || 'unknown' })} 1`)
lines.push('# HELP seoblog_deployment_last_timestamp_seconds Unix timestamp of the last recorded deployment result.')
lines.push('# TYPE seoblog_deployment_last_timestamp_seconds gauge')
lines.push(`seoblog_deployment_last_timestamp_seconds ${Number.isFinite(deploymentAt) ? deploymentAt / 1000 : 0}`)
lines.push('# HELP seoblog_observability_export_timestamp_seconds Unix timestamp of this runtime evidence export.')
lines.push('# TYPE seoblog_observability_export_timestamp_seconds gauge')
lines.push(`seoblog_observability_export_timestamp_seconds ${Date.now() / 1000}`)
lines.push(`seoblog_observability_export_success ${exportSuccess}`)

mkdirSync(dirname(outputPath), { recursive: true, mode: 0o750 })
const temporaryPath = `${outputPath}.${process.pid}.tmp`
writeFileSync(temporaryPath, `${lines.join('\n')}\n`, { mode: 0o640 })
renameSync(temporaryPath, outputPath)
