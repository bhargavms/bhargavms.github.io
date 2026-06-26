function apiBase() {
  const { hostname, port } = window.location;
  if ((hostname === 'localhost' || hostname === '127.0.0.1') && port === '1313') {
    return `http://${hostname}:8080/api`;
  }
  return '/api';
}

async function fetchJSON(path) {
  const res = await fetch(`${apiBase()}${path}`);
  if (!res.ok) {
    throw new Error(`HTTP ${res.status}`);
  }
  return res.json();
}

function formatNumber(value) {
  return new Intl.NumberFormat('en-US').format(value ?? 0);
}

function signedDelta(value) {
  if (!value) return '';
  return value > 0 ? `+${formatNumber(value)}` : formatNumber(value);
}

function renderProjectCard(project, stats, index = 0) {
  const li = document.createElement('li');
  li.className = 'card card-reveal';
  li.style.animationDelay = `${index * 50}ms`;
  const stars = stats.stargazers_count ?? 0;
  const lang = stats.language ?? '—';
  const desc = stats.description || project.description;
  li.innerHTML = `
    <h3><a href="https://github.com/${project.repo}" target="_blank" rel="noopener">${project.name}</a></h3>
    <div class="card-meta"><span class="tabular-nums">★ ${stars}</span> &middot; ${lang}</div>
    <p>${desc}</p>
    ${project.tags ? `<ul class="tag-list">${project.tags.map(t => `<li>${t}</li>`).join('')}</ul>` : ''}
  `;
  return li;
}

function renderSOSummary(summary) {
  const repChange = summary.reputation_change_week;
  const repChangeHTML = repChange
    ? `<span class="so-delta so-delta--positive">${signedDelta(repChange)}</span>`
    : '';
  const topTag = summary.top_tag;
  const topTagHTML = topTag
    ? `<div class="so-summary-meta">
        <span class="so-summary-meta__label">Top tag</span>
        <span class="so-tag">${topTag.name}</span>
        <span class="so-summary-meta__value tabular-nums">${formatNumber(topTag.count)} answers</span>
      </div>`
    : '';
  const privilege = summary.next_privilege;
  const privilegePct = Math.round((privilege?.progress ?? 0) * 100);
  const privilegeHTML = privilege
    ? `<div class="so-progress">
        <div class="so-progress__header">
          <span class="so-progress__label">Next privilege</span>
          <span class="so-progress__target tabular-nums">${formatNumber(privilege.rep)} rep</span>
        </div>
        <div class="so-progress__track" role="progressbar" aria-valuenow="${privilegePct}" aria-valuemin="0" aria-valuemax="100">
          <div class="so-progress__fill" style="width: ${privilegePct}%"></div>
        </div>
        <p class="so-progress__detail">${privilege.name}</p>
      </div>`
    : '';

  const peopleReached = summary.people_reached || '—';

  return `
    <div class="so-summary">
      <article class="so-summary-card">
        <p class="so-summary-card__label">Reputation</p>
        <div class="so-summary-card__hero">
          <p class="so-stat tabular-nums">${formatNumber(summary.reputation)}</p>
          ${repChangeHTML}
        </div>
        ${topTagHTML}
        ${privilegeHTML}
      </article>

      <article class="so-summary-card">
        <p class="so-summary-card__label">Impact</p>
        <div class="so-summary-card__hero">
          <p class="so-stat tabular-nums">${peopleReached}</p>
        </div>
        <p class="so-impact-label">people reached</p>
      </article>
    </div>
  `;
}

async function loadProjects() {
  const container = document.querySelector('[data-widget="projects"]');
  if (!container) return;

  try {
    const configRes = await fetch('/data/projects.json');
    const config = await configRes.json();
    const projects = config.featured || [];

    const results = await Promise.all(
      projects.map(async (p, index) => {
        try {
          const [owner, name] = p.repo.split('/');
          const stats = await fetchJSON(`/github/repo/${owner}/${name}`);
          return renderProjectCard(p, stats, index);
        } catch {
          const li = document.createElement('li');
          li.className = 'card card-reveal';
          li.style.animationDelay = `${index * 50}ms`;
          li.innerHTML = `
            <h3><a href="https://github.com/${p.repo}" target="_blank" rel="noopener">${p.name}</a></h3>
            <p>${p.description}</p>
          `;
          return li;
        }
      })
    );

    container.replaceChildren(...results);
  } catch (err) {
    container.innerHTML = `<p class="widget-error">Could not load projects: ${err.message}</p>`;
  }
}

async function loadStackOverflow() {
  const container = document.querySelector('[data-widget="stackoverflow"]');
  if (!container) return;

  try {
    const summary = await fetchJSON('/stackoverflow/summary');

    const wrapper = document.createElement('div');
    wrapper.className = 'so-section';

    const summaryEl = document.createElement('div');
    summaryEl.innerHTML = renderSOSummary(summary);
    wrapper.appendChild(summaryEl.firstElementChild);

    if (summary.profile_link) {
      const profileLink = document.createElement('a');
      profileLink.className = 'so-profile-link';
      profileLink.href = summary.profile_link;
      profileLink.target = '_blank';
      profileLink.rel = 'noopener';
      profileLink.textContent = 'View full profile →';
      wrapper.appendChild(profileLink);
    }

    container.replaceChildren(wrapper);
  } catch (err) {
    container.innerHTML = `<p class="widget-error">Could not load Stack Overflow: ${err.message}</p>`;
  }
}

document.addEventListener('DOMContentLoaded', () => {
  loadProjects();
  loadStackOverflow();
});
