'use strict';

let InstallerServer = require('./serve-installer.js');
let Builds = require('./builds.js');

// Real User-Agent strings sent by webi bootstrap scripts.
//
// Libc taxonomy:
//   none = static build, no runtime libc dep (often built with musl, but self-contained)
//   musl = requires musl C/C++ runtime at runtime (e.g. node-musl)
//   gnu  = requires glibc at runtime (crashes on musl-only/Alpine)
//   libc = host UA value meaning "I have glibc" (not used in release metadata)
//
// Known issues:
//
// 1. WATERFALL libc vs gnu: The WATERFALL maps `libc` => ['none', 'libc']
//    but never tries 'gnu'. Packages with glibc-linked builds (libc='gnu' in
//    Go cache) won't match for hosts reporting 'libc'. Fix: update WATERFALL
//    to `libc: ['none', 'gnu', 'libc']` in build-classifier submodule.
//
// 2. Go cache .git regression: The Go cache includes .git source repo URLs
//    as releases, creating ANYOS/ANYARCH triplets. These match before
//    platform-specific binaries. Fix: exclude .git from Go cache output.

let UA_CASES = [
  // === macOS (no libc issue — darwin uses libc='none') ===
  {
    label: 'bat macOS arm64',
    pkg: 'bat',
    ua: 'aarch64/unknown Darwin/24.2.0 libc',
    expectOs: 'darwin',
    expectArch: 'aarch64',
    expectExt: 'tar.gz',
  },
  {
    label: 'bat macOS amd64',
    pkg: 'bat',
    ua: 'x86_64/unknown Darwin/23.0.0 libc',
    expectOs: 'darwin',
    expectArch: 'x86_64',
    expectExt: 'tar.gz',
  },
  {
    label: 'go macOS arm64',
    pkg: 'go',
    ua: 'aarch64/unknown Darwin/24.2.0 libc',
    expectOs: 'darwin',
    expectArch: 'aarch64',
    expectExt: 'tar.gz',
  },
  {
    label: 'node macOS arm64',
    pkg: 'node',
    ua: 'aarch64/unknown Darwin/24.2.0 libc',
    expectOs: 'darwin',
    expectArch: 'aarch64',
    expectExt: 'tar.xz',
  },
  {
    label: 'rg macOS arm64',
    pkg: 'rg',
    ua: 'aarch64/unknown Darwin/24.2.0 libc',
    expectOs: 'darwin',
    expectArch: 'aarch64',
    expectExt: 'tar.gz',
  },

  // === Windows ===
  {
    label: 'bat Windows amd64',
    pkg: 'bat',
    ua: 'x86_64/unknown Windows/10.0.19041 msvc',
    expectOs: 'windows',
    expectArch: 'x86_64',
    expectExt: 'zip',
  },
  {
    label: 'go Windows amd64',
    pkg: 'go',
    ua: 'x86_64/unknown Windows/10.0.19041 msvc',
    expectOs: 'windows',
    expectArch: 'x86_64',
    expectExt: 'zip',
  },

  // === Linux musl (Alpine/Docker) ===
  {
    label: 'bat Linux musl',
    pkg: 'bat',
    ua: 'x86_64/unknown Linux/5.15.0 musl',
    expectOs: 'linux',
    expectArch: 'x86_64',
    expectExt: 'tar.gz',
  },

  // === Linux glibc — packages with libc='none' in cache ===
  {
    label: 'go Linux amd64',
    pkg: 'go',
    ua: 'x86_64/unknown Linux/5.15.0 libc',
    expectOs: 'linux',
    expectArch: 'x86_64',
    expectExt: 'tar.gz',
  },
  // === Known: WATERFALL libc vs gnu ===
  // Packages whose Go cache has libc='gnu' for Linux glibc builds.
  // The WATERFALL maps `libc` => ['none', 'libc'] but never tries 'gnu'.
  {
    label: 'bat Linux amd64 (known: WATERFALL libc vs gnu)',
    pkg: 'bat',
    ua: 'x86_64/unknown Linux/5.15.0 libc',
    known: 'WATERFALL libc->gnu gap',
  },
  {
    label: 'rg Linux amd64 (known: WATERFALL libc vs gnu)',
    pkg: 'rg',
    ua: 'x86_64/unknown Linux/5.15.0 libc',
    known: 'WATERFALL libc->gnu gap',
  },
  {
    label: 'node Linux amd64 (known: WATERFALL libc vs gnu)',
    pkg: 'node',
    ua: 'x86_64/unknown Linux/5.15.0 libc',
    known: 'WATERFALL libc->gnu gap',
  },

  // === Known: ANYOS/ANYARCH .git priority ===
  {
    label: 'jq macOS arm64 (known: ANYOS .git priority)',
    pkg: 'jq',
    ua: 'aarch64/unknown Darwin/24.2.0 libc',
    known: 'ANYOS .git matched before platform binary',
  },
  {
    label: 'caddy macOS arm64 (known: ANYOS .git priority)',
    pkg: 'caddy',
    ua: 'aarch64/unknown Darwin/24.2.0 libc',
    known: 'ANYOS .git matched before platform binary',
  },
  {
    label: 'caddy Linux amd64 (known: ANYOS .git priority)',
    pkg: 'caddy',
    ua: 'x86_64/unknown Linux/5.15.0 libc',
    known: 'ANYOS .git matched before platform binary',
  },
];

async function main() {
  let failures = 0;
  let passes = 0;
  let knowns = 0;
  let errors = 0;

  console.log('Initializing build cache...');
  await Builds.init();
  console.log('');

  console.log('=== Installer Resolution Tests ===');
  console.log('');

  for (let tc of UA_CASES) {
    try {
      let [pkg, params] = await InstallerServer.helper({
        unameAgent: tc.ua,
        projectName: tc.pkg,
        tag: 'stable',
        formats: ['tar', 'exe', 'zip', 'xz', 'dmg'],
        libc: '',
      });

      // Known issue — just verify it fails as expected
      if (tc.known) {
        if (pkg.channel === 'error' || !pkg.download || pkg.download.includes('doesntexist') || pkg.ext === 'git') {
          console.log(`  KNOWN ${tc.label}`);
          knowns++;
        } else {
          console.log(`  PASS ${tc.label} (known issue resolved!): v${pkg.version} .${pkg.ext}`);
          passes++;
        }
        continue;
      }

      if (pkg.channel === 'error') {
        console.log(`  FAIL ${tc.label}: resolved to error package`);
        failures++;
        continue;
      }

      let diffs = [];

      if (tc.expectOs && pkg.os !== tc.expectOs) {
        diffs.push(`os: got=${pkg.os} want=${tc.expectOs}`);
      }
      if (tc.expectArch && pkg.arch !== tc.expectArch) {
        diffs.push(`arch: got=${pkg.arch} want=${tc.expectArch}`);
      }
      if (tc.expectExt && pkg.ext !== tc.expectExt) {
        diffs.push(`ext: got=${pkg.ext} want=${tc.expectExt}`);
      }

      if (!pkg.version || pkg.version === '0.0.0') {
        diffs.push('version: missing or zero');
      }

      if (!pkg.download || pkg.download.includes('doesntexist')) {
        diffs.push('download: missing or error');
      }

      if (diffs.length > 0) {
        console.log(`  FAIL ${tc.label}: ${diffs.join(', ')}`);
        failures++;
      } else {
        console.log(`  PASS ${tc.label}: v${pkg.version} .${pkg.ext} ${pkg.download.split('/').pop()}`);
        passes++;
      }
    } catch (err) {
      if (tc.known) {
        console.log(`  KNOWN ${tc.label} (error: ${err.message})`);
        knowns++;
        continue;
      }
      console.log(`  ERROR ${tc.label}: ${err.message}`);
      errors++;
    }
  }

  console.log('');
  console.log(`=== Results: ${passes} passed, ${failures} failed, ${knowns} known, ${errors} errors ===`);
  if (failures > 0 || errors > 0) {
    process.exit(1);
  }
}

main().catch(function (err) {
  console.error(err.stack);
  process.exit(1);
});
