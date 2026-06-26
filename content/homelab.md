---
title: Homelab
terminal: true
---

<div class="terminal-prompt">cat homelab/README.md</div>

# mogra.dev — self-hosted on Kubernetes

This site runs on my private Kubernetes cluster, not GitHub Pages.

<div class="ascii-divider">────────────────────────────────────────────────────────</div>

## Architecture

<div class="terminal-block flow-diagram">
<pre>Internet
   │
   ▼
Cloudflare (DNS + SSL)
   │
   ▼
cloudflared (Zero Trust Tunnel)
   │
   ├── /api/*  ──►  mogra-proxy (Go caching proxy)
   │                    │
   │                    ├── GitHub API
   │                    └── StackExchange API
   │
   └── /*      ──►  mogra (nginx + Hugo static site)</pre>
</div>

<div class="terminal-prompt">kubectl get deployments -n mogra</div>

<div class="terminal-block">
<span class="output">NAME          READY   UP-TO-DATE   AVAILABLE</span>
<span class="output">mogra         1/1     1            1</span>
<span class="output">mogra-proxy   1/1     1            1</span>
</div>

## Components

| Component | Role |
|-----------|------|
| **Hugo** | Static site generator — blog, resume, portfolio |
| **nginx** | Serves static files from container |
| **Go proxy** | Caches GitHub + StackOverflow API responses |
| **Argo CD** | GitOps deployment from `infra` repo |
| **Cloudflare Tunnel** | Exposes cluster services without public ingress |
| **Terraform Cloud** | Manages DNS, tunnel config, and cluster secrets |

<div class="ascii-divider">────────────────────────────────────────────────────────</div>

## Why self-host?

- Full control over deployment pipeline (GHCR → ArgoCD → cluster)
- Practice what I preach on infrastructure
- Run a lightweight API proxy without third-party serverless limits
- The site itself is a portfolio piece

<div class="terminal-prompt">echo "Built with Go, Hugo, and too much YAML"</div>

<div class="terminal-block">
<span class="comment"># Built with Go, Hugo, and too much YAML</span>
</div>

<p><a class="btn" href="/projects/">See projects</a></p>
