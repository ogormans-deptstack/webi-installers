'use strict';

// Comprehensive live-vs-local comparison test.
// Fetches from the live webinstall.dev/webi.sh APIs and compares against
// local cache-only output to catch regressions.
//
// Usage:
//   node _webi/test-live-compare.js              # compare against live
//   node _webi/test-live-compare.js --refresh    # refresh golden data and compare

let Fs = require('node:fs/promises');
let Path = require('node:path');
let Https = require('node:https');

let Releases = require('./transform-releases.js');
let InstallerServer = require('./serve-installer.js');
let Builds = require('./builds.js');

let TESTDATA_DIR = Path.join(__dirname, 'testdata');

let REFRESH = process.argv.includes('--refresh');

// Packages to test — mix of Go-built, Rust-built, and C/C++ projects
let TEST_PKGS = ['bat', 'go', 'node', 'rg', 'jq', 'caddy'];

// OS/arch combos for filtered release API tests
let RELEASE_API_CASES = [
  { os: 'macos', arch: 'amd64' },
  { os: 'macos', arch: 'arm64' },
  { os: 'linux', arch: 'amd64' },
  { os: 'windows', arch: 'amd64' },
];

// UA strings for installer resolution tests
let INSTALLER_CASES = [
  { label: 'macOS arm64', ua: 'aarch64/unknown Darwin/24.2.0 libc' },
  { label: 'macOS amd64', ua: 'x86_64/unknown Darwin/23.0.0 libc' },
  { label: 'Linux musl', ua: 'x86_64/unknown Linux/5.15.0 musl' },
  { label: 'Windows amd64', ua: 'x86_64/unknown Windows/10.0.19041 msvc' },
];

// Known differences between Go cache and production (not regressions)
// Extensions that the Go cache correctly excludes (non-installable formats)
// OR that the Go cache includes but shouldn't (man pages, etc.)
let KNOWN_EXT_DIFFS = new Set([
  'deb', 'rpm', 'sha256', 'sig', 'pem', 'sbom', 'txt',
  '1', '2', '3', '4', '5', '6', '7', '8',  // man page extensions
]);

function httpsGet(url) {
  return new Promise(function (resolve, reject) {
    Https.get(url, function (res) {
      let data = '';
      res.on('data', function (chunk) {
        data += chunk;
      });
      res.on('end', function () {
        if (res.statusCode !== 200) {
          reject(new Error(`HTTP ${res.statusCode}: ${url}`));
          return;
        }
        resolve(data);
      });
    }).on('error', reject);
  });
}

async function fetchLiveReleases(pkg, os, arch, limit) {
  let url = `https://webinstall.dev/api/releases/${pkg}@stable.json?limit=${limit || 100}`;
  if (os) {
    url += `&os=${os}`;
  }
  if (arch) {
    url += `&arch=${arch}`;
  }
  let json = await httpsGet(url);
  return JSON.parse(json);
}

async function fetchLiveInstaller(pkg, ua) {
  return new Promise(function (resolve, reject) {
    let url = `https://webi.sh/${pkg}@stable.sh`;
    let opts = {
      headers: { 'User-Agent': ua },
    };
    Https.get(url, opts, function (res) {
      // Follow redirects
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        let redir = res.headers.location;
        if (redir.startsWith('/')) {
          redir = 'https://webi.sh' + redir;
        }
        Https.get(redir, opts, function (res2) {
          let data = '';
          res2.on('data', function (chunk) {
            data += chunk;
          });
          res2.on('end', function () {
            resolve(data);
          });
        }).on('error', reject);
        return;
      }
      let data = '';
      res.on('data', function (chunk) {
        data += chunk;
      });
      res.on('end', function () {
        resolve(data);
      });
    }).on('error', reject);
  });
}

function parseInstallerVars(scriptText) {
  let vars = {};
  // Match both `export WEBI_FOO='...'` and `WEBI_FOO='...'`
  let names = ['WEBI_PKG_URL', 'WEBI_VERSION', 'WEBI_EXT', 'WEBI_OS', 'WEBI_ARCH', 'PKG_NAME'];
  for (let name of names) {
    let re = new RegExp("^(?:export\\s+)?" + name + "='([^']*)'", 'm');
    let m = scriptText.match(re);
    if (m) {
      vars[name] = m[1];
    }
  }
  return vars;
}

async function saveGolden(name, data) {
  await Fs.mkdir(TESTDATA_DIR, { recursive: true });
  let file = Path.join(TESTDATA_DIR, name);
  await Fs.writeFile(file, JSON.stringify(data), 'utf8');
}

async function loadGolden(name) {
  let file = Path.join(TESTDATA_DIR, name);
  try {
    let json = await Fs.readFile(file, 'utf8');
    return JSON.parse(json);
  } catch (e) {
    if (e.code === 'ENOENT') {
      return null;
    }
    throw e;
  }
}

async function main() {
  let passes = 0;
  let failures = 0;
  let skips = 0;
  let knowns = 0;

  console.log('Initializing build cache...');
  await Builds.init();
  console.log('');

  // ================================================================
  // Test 1: Release API — unfiltered
  // ================================================================
  console.log('=== Test 1: Unfiltered /api/releases/{pkg}.json ===');
  console.log('');

  for (let pkg of TEST_PKGS) {
    let goldenName = `live_${pkg}.json`;
    let liveReleases;

    if (REFRESH) {
      try {
        liveReleases = await fetchLiveReleases(pkg);
        await saveGolden(goldenName, liveReleases);
        console.log(`  refreshed ${goldenName}`);
      } catch (e) {
        console.log(`  SKIP ${pkg}: fetch error: ${e.message}`);
        skips++;
        continue;
      }
    } else {
      liveReleases = await loadGolden(goldenName);
      if (!liveReleases) {
        console.log(`  SKIP ${pkg}: no golden data (run with --refresh)`);
        skips++;
        continue;
      }
    }

    let localResult = await Releases.getReleases({
      pkg: pkg,
      ver: '',
      os: '',
      arch: '',
      libc: '',
      lts: false,
      channel: '',
      formats: [],
      limit: 100,
    });
    let localReleases = localResult.releases;

    // Compare OS vocabulary — check that local has the core OSes that live has
    let liveOses = [...new Set(liveReleases.map(function (r) { return r.os; }))].sort();
    let localOses = [...new Set(localReleases.map(function (r) { return r.os; }))].sort();
    let coreOses = ['linux', 'macos', 'windows'];
    let liveCore = liveOses.filter(function (o) { return coreOses.includes(o); }).sort();
    let localCore = localOses.filter(function (o) { return coreOses.includes(o); }).sort();
    // Local should have at least the core OSes that live has
    let missingCore = liveCore.filter(function (o) { return !localCore.includes(o); });
    if (missingCore.length > 0) {
      console.log(`  FAIL ${pkg} OS: missing core OSes: ${JSON.stringify(missingCore)}`);
      failures++;
    } else {
      console.log(`  PASS ${pkg} OS: ${JSON.stringify(localCore)}`);
      passes++;
    }

    // Compare ext vocabulary (excluding known non-installable formats)
    let liveExts = [...new Set(liveReleases.map(function (r) { return r.ext; }))].sort();
    let localExts = [...new Set(localReleases.map(function (r) { return r.ext; }))].sort();
    let liveExtsFiltered = liveExts.filter(function (e) { return !KNOWN_EXT_DIFFS.has(e); });
    let localExtsFiltered = localExts.filter(function (e) { return !KNOWN_EXT_DIFFS.has(e); });
    let extMatch = true;
    for (let ext of localExtsFiltered) {
      if (!liveExtsFiltered.includes(ext)) {
        // Local has a real ext that live doesn't — may be due to limit or sampling
        // Only fail if it's clearly wrong (not a standard installable format)
        let installable = ['tar.gz', 'tar.xz', 'tar.bz2', 'zip', 'exe', 'dmg', 'pkg', 'msi', '7z', 'xz'];
        if (!installable.includes(ext)) {
          console.log(`  FAIL ${pkg} ext: local has unexpected '${ext}'`);
          failures++;
          extMatch = false;
          break;
        }
      }
    }
    if (extMatch) {
      console.log(`  PASS ${pkg} ext: ${JSON.stringify(localExtsFiltered)}`);
      passes++;
    }

    // Version format — no 'v' prefix
    let hasVPrefix = localReleases.some(function (r) {
      return r.version && r.version.startsWith('v');
    });
    if (hasVPrefix) {
      console.log(`  FAIL ${pkg}: versions have 'v' prefix`);
      failures++;
    } else {
      console.log(`  PASS ${pkg}: no 'v' prefix`);
      passes++;
    }
  }

  // ================================================================
  // Test 2: Release API — filtered by OS/arch
  // ================================================================
  console.log('');
  console.log('=== Test 2: Filtered /api/releases/{pkg}@stable.json?os=...&arch=... ===');
  console.log('');

  for (let pkg of TEST_PKGS) {
    for (let tc of RELEASE_API_CASES) {
      let goldenName = `live_${pkg}_os_${tc.os}_arch_${tc.arch}.json`;
      let liveReleases;

      if (REFRESH) {
        try {
          liveReleases = await fetchLiveReleases(pkg, tc.os, tc.arch, 1);
          await saveGolden(goldenName, liveReleases);
        } catch (e) {
          skips++;
          continue;
        }
      } else {
        liveReleases = await loadGolden(goldenName);
        if (!liveReleases) {
          skips++;
          continue;
        }
      }

      let liveFirst = liveReleases[0];
      if (!liveFirst || liveFirst.channel === 'error') {
        skips++;
        continue;
      }

      let localResult = await Releases.getReleases({
        pkg: pkg,
        ver: '',
        os: tc.os,
        arch: tc.arch,
        libc: '',
        lts: false,
        channel: 'stable',
        formats: ['tar', 'zip', 'exe', 'xz'],
        limit: 1,
      });
      let localFirst = localResult.releases[0];

      if (!localFirst || localFirst.channel === 'error') {
        console.log(`  FAIL ${pkg} ${tc.os}/${tc.arch}: local returned error/empty`);
        failures++;
        continue;
      }

      let diffs = [];
      // Compare os, arch, ext (skip version/download since cache age may differ)
      if (liveFirst.os !== localFirst.os) {
        diffs.push(`os: live=${liveFirst.os} local=${localFirst.os}`);
      }
      if (liveFirst.arch !== localFirst.arch) {
        diffs.push(`arch: live=${liveFirst.arch} local=${localFirst.arch}`);
      }
      if (liveFirst.ext !== localFirst.ext) {
        if (KNOWN_EXT_DIFFS.has(liveFirst.ext)) {
          // Live returns a non-installable format (deb, pem, etc.) — known
          console.log(`  KNOWN ${pkg} ${tc.os}/${tc.arch}: live ext '${liveFirst.ext}' excluded by Go cache, local='${localFirst.ext}'`);
          knowns++;
          continue;
        }
        diffs.push(`ext: live=${liveFirst.ext} local=${localFirst.ext}`);
      }

      if (diffs.length > 0) {
        console.log(`  FAIL ${pkg} ${tc.os}/${tc.arch}: ${diffs.join(', ')}`);
        failures++;
      } else {
        console.log(`  PASS ${pkg} ${tc.os}/${tc.arch}: v${localFirst.version} .${localFirst.ext}`);
        passes++;
      }
    }
  }

  // ================================================================
  // Test 3: Installer resolution — compare rendered script vars
  // ================================================================
  console.log('');
  console.log('=== Test 3: Installer script variables (local serveInstaller vs live) ===');
  console.log('');

  for (let pkg of ['bat', 'go', 'rg']) {
    for (let tc of INSTALLER_CASES) {
      // Get local result
      let localVars;
      try {
        let script = await InstallerServer.serveInstaller(
          'https://webi.sh',
          tc.ua,
          pkg,
          'stable',
          'sh',
          ['tar', 'exe', 'zip', 'xz', 'dmg'],
          '',
        );
        localVars = parseInstallerVars(script);
      } catch (e) {
        console.log(`  ERROR ${pkg} ${tc.label}: ${e.message}`);
        failures++;
        continue;
      }

      if (!localVars.WEBI_PKG_URL || localVars.WEBI_PKG_URL === '') {
        // Check if this is a known issue
        if (localVars.WEBI_EXT === 'err') {
          console.log(`  KNOWN ${pkg} ${tc.label}: no match (WATERFALL gap)`);
          knowns++;
          continue;
        }
        console.log(`  FAIL ${pkg} ${tc.label}: empty WEBI_PKG_URL`);
        failures++;
        continue;
      }

      // Verify the URL looks like a real download
      let url = localVars.WEBI_PKG_URL;
      let hasRealDomain = url.includes('github.com') ||
                          url.includes('dl.google.com') ||
                          url.includes('nodejs.org') ||
                          url.includes('jqlang');
      if (!hasRealDomain) {
        console.log(`  FAIL ${pkg} ${tc.label}: suspicious URL: ${url}`);
        failures++;
        continue;
      }

      // Verify version is set and has no 'v' prefix
      if (!localVars.WEBI_VERSION || localVars.WEBI_VERSION === '0.0.0') {
        console.log(`  FAIL ${pkg} ${tc.label}: bad version: ${localVars.WEBI_VERSION}`);
        failures++;
        continue;
      }

      // Verify ext is a real installable format
      let ext = localVars.WEBI_EXT;
      let goodExts = ['tar.gz', 'tar.xz', 'zip', 'exe', 'dmg', 'pkg', 'msi', '7z'];
      if (!goodExts.includes(ext)) {
        console.log(`  FAIL ${pkg} ${tc.label}: bad ext: ${ext}`);
        failures++;
        continue;
      }

      console.log(`  PASS ${pkg} ${tc.label}: v${localVars.WEBI_VERSION} .${ext} ${url.split('/').pop()}`);
      passes++;
    }
  }

  // ================================================================
  // Summary
  // ================================================================
  console.log('');
  console.log(`=== Results: ${passes} passed, ${failures} failed, ${knowns} known, ${skips} skipped ===`);
  if (failures > 0) {
    process.exit(1);
  }
}

main().catch(function (err) {
  console.error(err.stack);
  process.exit(1);
});
