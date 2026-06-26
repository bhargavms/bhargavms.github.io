# Semantic Release

This repo uses [semantic-release](https://semantic-release.gitbook.io/) to bump
`mogra-site` and `mogra-proxy` independently from Conventional Commits on `main`.

## Commit messages

Use [Conventional Commits](https://www.conventionalcommits.org/):

| Type | Release bump |
|------|----------------|
| `feat:` | minor |
| `fix:`, `perf:` | patch |
| `feat!:` or footer `BREAKING CHANGE:` | major |
| `docs:`, `chore:`, `style:`, `refactor:`, `test:`, `ci:` | no release |

Commitlint runs locally on `commit-msg` (via pre-commit) and on pull requests in CI.

## Release flow

1. Push to `main` triggers the site and/or proxy publish workflow (path filters).
2. semantic-release creates a component tag (`mogra-site-vX.Y.Z` or `mogra-proxy-vX.Y.Z`), a GitHub Release, and release notes.
3. On a new version, CI builds the Docker image, pushes to GHCR, and commits the new tag to `bhargavms/infra`.
4. Argo CD syncs the cluster from the infra repo.

## Required secret: `INFRA_DEPLOY_TOKEN`

Cross-repo infra updates need a token with write access to `bhargavms/infra`.

1. Create a fine-grained PAT (or classic PAT) for a bot/user account with **Contents: Read and write** on `bhargavms/infra` only.
2. In this repo: **Settings → Secrets and variables → Actions → New repository secret**
3. Name: `INFRA_DEPLOY_TOKEN`
4. Value: the PAT

Without this secret, releases still create git tags, GitHub Releases, and GHCR images, but the cluster manifest in infra will not update automatically.

## Baseline tags

Release history starts from:

- `mogra-site-v3.2.0`
- `mogra-proxy-v1.1.0`

## Local tooling

```bash
npm ci
npx commitlint --edit   # validate a commit message file
npm run release:site    # dry-run locally only if GITHUB_TOKEN is set
```
