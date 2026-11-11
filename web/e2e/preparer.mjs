// Prepare and start Observatoire for the end-to-end tests.
// Can be launched from any directory: the repository root is derived from this file.
import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { mkdtemp, rm } from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const racine = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')

function commandeNpm() {
  return process.platform === 'win32' ? 'npm.cmd' : 'npm'
}

function lancer(commande, args) {
  return new Promise((resolve, reject) => {
    const enfant = spawn(commande, args, { cwd: racine, stdio: 'inherit' })
    enfant.on('error', reject)
    enfant.on('exit', (code, signal) => {
      if (code === 0) resolve()
      else reject(new Error(`${commande} ${args.join(' ')} exited (${code ?? signal})`))
    })
  })
}

const alpha = path.join(racine, 'testdata', 'depots', 'alpha')
const beta = path.join(racine, 'testdata', 'depots', 'beta')
if (!existsSync(path.join(alpha, '.git')) || !existsSync(path.join(beta, '.git'))) {
  console.error('The test repositories are missing (testdata/depots/alpha and testdata/depots/beta).')
  console.error('Generate them from the repository root, then run the tests again:')
  console.error('  go run ./scripts/fixtures/make-repos.go')
  process.exit(1)
}

if (!existsSync(path.join(racine, 'web', 'dist', 'index.html'))) {
  await lancer(commandeNpm(), ['--prefix', 'web', 'run', 'build'])
}

const tmp = await mkdtemp(path.join(os.tmpdir(), 'observatoire-e2e-'))
const binaire = path.join(tmp, process.platform === 'win32' ? 'observatoire.exe' : 'observatoire')
const base = path.join(tmp, 'e2e.db')
const vault = path.join(racine, 'testdata', 'vault')

async function effacerTmp() {
  await rm(tmp, { recursive: true, force: true }).catch(() => {})
}

try {
  await lancer('go', ['build', '-tags', 'embed', '-o', binaire, './cmd/observatoire'])
  await lancer(binaire, [
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
  await effacerTmp()
  process.exit(1)
}

const serveur = spawn(binaire, ['serve', '-db', base, '-port', '7879'], { cwd: racine, stdio: 'inherit' })

function arreter() {
  if (serveur.exitCode === null && !serveur.killed) serveur.kill()
}

serveur.on('error', async (err) => {
  console.error(err)
  arreter()
  await effacerTmp()
  process.exit(1)
})
serveur.on('exit', async (code) => {
  await effacerTmp()
  process.exit(code ?? 0)
})
process.on('SIGINT', arreter)
process.on('SIGTERM', arreter)
