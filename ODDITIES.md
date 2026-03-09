# Version & Release Oddities

Known non-standard version formats and release quirks across Webi packages.
This matters for version parsing, sorting, and classification.

## 4-Part Versions

| Package | Example | Notes |
|---------|---------|-------|
| chromedriver | `121.0.6120.0` | From Google Chrome's versioning. No transform applied. |
| gpg | `2.2.19.0` | 4th segment treated as build metadata. `releases.js` converts to `2.2.19+0`. |

## Date-Based Versions

| Package | Notes |
|---------|-------|
| atomicparsley | Uses date-based version strings. |

## Non-Numeric Tag Prefixes

| Package | Raw Tag | Cleaned | Transform |
|---------|---------|---------|-----------|
| lf | `r21` | `0.21.0` | `r` prefix → prepend `0.` |
| bun | `bun-v1.0.0` | `1.0.0` | Strip `bun-` prefix |
| jq | `jq-1.7` | `1.7` | Strip `jq-` prefix |
| watchexec | `cli-v1.2.3` | `1.2.3` | Strip `cli-` prefix |
| ffmpeg | `b6.0` | `6.0` | Strip `b` prefix |

## Underscore-Delimited Tags

| Package | Raw Tag | Cleaned | Transform |
|---------|---------|---------|-----------|
| postgres | `REL_17_0` | `17.0` | Strip `REL_`, replace `_` with `.` |
| psql | `REL_17_0` | `17.0` | Same as postgres |
| pg | `REL_17_0` | `17.0` | Same as postgres |

## Platform Suffix in Version

| Package | Raw Tag | Cleaned | Transform |
|---------|---------|---------|-----------|
| git (Windows) | `2.41.0.windows.1` | `2.41.0` | Strip `.windows.N` suffix |

## Complex Pre-Release Formats

| Package | Example | Notes |
|---------|---------|-------|
| flutter | `2.3.0-16.0.pre` | Pre-release with extra dots and numeric segments |
| iterm2 | `iTerm2_3_5_0beta17` | Underscores as separators, beta attached without dash → `3.5.0-beta17` |

## Bundled Dependency Versions (Not Package Versions)

| Package | Context | Example |
|---------|---------|---------|
| node | v8 engine version in metadata | `11.3.244.8` (4-part) |
| node | zlib version in metadata | `1.2.13.1-motley` (4-part + suffix) |
