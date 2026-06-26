document.addEventListener('DOMContentLoaded', () => {
  document.querySelectorAll('pre').forEach((pre) => {
    const btn = document.createElement('button');
    btn.className = 'copy-btn';
    btn.type = 'button';
    btn.textContent = 'Copy';
    btn.addEventListener('click', async () => {
      const code = pre.querySelector('code');
      const text = code ? code.innerText : pre.innerText;
      try {
        await navigator.clipboard.writeText(text);
        btn.textContent = 'Copied!';
        setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
      } catch {
        btn.textContent = 'Failed';
      }
    });
    pre.style.position = 'relative';
    pre.appendChild(btn);
  });

  initThemeToggle();
  initScrollHeader();
});

const THEME_STORAGE_KEY = 'theme';

function getStoredTheme() {
  return localStorage.getItem(THEME_STORAGE_KEY);
}

function getSystemTheme() {
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  updateThemeToggle(theme);
}

function updateThemeToggle(theme) {
  const toggle = document.querySelector('.theme-toggle');
  if (!toggle) return;

  const isDark = theme === 'dark';
  toggle.setAttribute('aria-pressed', String(isDark));
  toggle.setAttribute(
    'aria-label',
    isDark ? 'Switch to light mode' : 'Switch to dark mode',
  );
}

function initThemeToggle() {
  const toggle = document.querySelector('.theme-toggle');
  if (!toggle) return;

  const currentTheme = document.documentElement.dataset.theme || getSystemTheme();
  updateThemeToggle(currentTheme);

  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      document.documentElement.classList.add('theme-icons-ready');
    });
  });

  toggle.addEventListener('click', () => {
    const nextTheme = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
    applyTheme(nextTheme);
    localStorage.setItem(THEME_STORAGE_KEY, nextTheme);
  });

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (event) => {
    if (getStoredTheme()) return;
    applyTheme(event.matches ? 'dark' : 'light');
  });
}

function initScrollHeader() {
  const header = document.querySelector('.site-header');
  if (!header || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    return;
  }

  let lastScrollY = window.scrollY;
  let ticking = false;
  const deltaThreshold = 8;

  const update = () => {
    const currentScrollY = window.scrollY;
    const delta = currentScrollY - lastScrollY;
    const minScroll = header.offsetHeight;

    if (currentScrollY <= minScroll) {
      header.classList.remove('is-hidden');
    } else if (delta > deltaThreshold) {
      header.classList.add('is-hidden');
    } else if (delta < -deltaThreshold) {
      header.classList.remove('is-hidden');
    }

    lastScrollY = currentScrollY;
    ticking = false;
  };

  window.addEventListener('scroll', () => {
    if (!ticking) {
      window.requestAnimationFrame(update);
      ticking = true;
    }
  }, { passive: true });
}
