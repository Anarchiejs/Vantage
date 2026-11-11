// Prepare and start Observatoire for the end-to-end tests.
// Can be launched from any directory: the repository root is derived from this file.
import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { mkdtemp, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')

function npmCommand() {
  return process.platform === 'win32' ? 'npm.cmd' : 'npm'
}

function run(command, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { cwd: root, stdio: 'inherit' })
    child.on('error', reject)
    child.on('exit', (code, signal) => {
      if (code === 0) resolve()
      else reject(new Error(`${command} ${args.join(' ')} exited (${code ?? signal})`))
    })
  })
}

const alpha = path.join(root, 'testdata', 'repos', 'alpha')
const beta = path.join(root, 'testdata', 'repos', 'beta')
if (!existsSync(path.join(alpha, '.git')) || !existsSync(path.join(beta, '.git'))) {
  console.error('The test repositories are missing (testdata/repos/alpha and testdata/repos/beta).')
  console.error('Generate them from the repository root, then run the tests again:')
  console.error('  go run ./scripts/fixtures/make-repos.go')
  process.exit(1)
}

if (!existsSync(path.join(root, 'web', 'dist', 'index.html'))) {
  await run(npmCommand(), ['--prefix', 'web', 'run', 'build'])
}

const tmp = await mkdtemp(path.join(os.tmpdir(), 'observatoire-e2e-'))
const binary = path.join(tmp, process.platform === 'win32' ? 'observatoire.exe' : 'observatoire')
const base = path.join(tmp, 'e2e.db')
const vault = path.join(root, 'testdata', 'vault')

async function removeTmp() {
  await rm(tmp, { recursive: true, force: true }).catch(() => {})
}

try {
  await run('go', ['build', '-tags', 'embed', '-o', binary, './cmd/observatoire'])
  await run(binary, [
    'import',
    '-db',
    base,
    '-vault',
    vault,
    '-repos',
    `Atlas=${alpha},Compass=${beta}`,
  ])
} catch (err) {
  console.error(err instanceof Error ? err.message : err)
  await removeTmp()
  process.exit(1)
}

const server = spawn(binary, ['serve', '-db', base, '-port', '7879'], { cwd: root, stdio: 'inherit' })

function stop() {
  if (server.exitCode === null && !server.killed) server.kill()
}

server.on('error', async (err) => {
  console.error(err)
  stop()
  await removeTmp()
  process.exit(1)
})
server.on('exit', async (code) => {
  await removeTmp()
  process.exit(code ?? 0)
})
process.on('SIGINT', stop)
process.on('SIGTERM', stop)
