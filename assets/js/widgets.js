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

function renderProjectCard(project, stats) {
  const li = document.createElement('li');
  li.className = 'card';
  const stars = stats.stargazers_count ?? 0;
  const lang = stats.language ?? '—';
  const desc = stats.description || project.description;
  li.innerHTML = `
    <h3><a href="https://github.com/${project.repo}" target="_blank" rel="noopener">${project.name}</a></h3>
    <div class="card-meta">★ ${stars} &middot; ${lang}</div>
    <p>${desc}</p>
    ${project.tags ? `<ul class="tag-list">${project.tags.map(t => `<li>${t}</li>`).join('')}</ul>` : ''}
  `;
  return li;
}

function renderAnswerCard(answer) {
  const li = document.createElement('li');
  li.className = 'card';
  const votes = answer.score ?? 0;
  const title = answer.title || `Answer #${answer.answer_id || ''}`;
  const link = answer.link || (answer.answer_id ? `https://stackoverflow.com/a/${answer.answer_id}` : '#');
  const excerpt = (answer.excerpt || answer.body || '').replace(/<[^>]+>/g, '').trim();
  li.innerHTML = `
    <h3><a href="${link}" target="_blank" rel="noopener">${title}</a></h3>
    <div class="card-meta">▲ ${votes} votes</div>
    <p>${excerpt ? excerpt.slice(0, 160) + '…' : 'View on Stack Overflow'}</p>
  `;
  return li;
}

async function loadProjects() {
  const container = document.querySelector('[data-widget="projects"]');
  if (!container) return;

  try {
    const configRes = await fetch('/data/projects.json');
    const config = await configRes.json();
    const projects = config.featured || [];

    const results = await Promise.all(
      projects.map(async (p) => {
        try {
          const [owner, name] = p.repo.split('/');
          const stats = await fetchJSON(`/github/repo/${owner}/${name}`);
          return renderProjectCard(p, stats);
        } catch {
          const li = document.createElement('li');
          li.className = 'card';
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
    const data = await fetchJSON('/stackoverflow/answers?pagesize=5');
    const items = data.items || [];
    if (items.length === 0) {
      container.innerHTML = '<p class="widget-loading">No answers found.</p>';
      return;
    }
    const list = document.createElement('ul');
    list.className = 'card-grid';
    list.replaceChildren(...items.map(renderAnswerCard));
    container.replaceChildren(list);
  } catch (err) {
    container.innerHTML = `<p class="widget-error">Could not load Stack Overflow answers: ${err.message}</p>`;
  }
}

document.addEventListener('DOMContentLoaded', () => {
  loadProjects();
  loadStackOverflow();
});
