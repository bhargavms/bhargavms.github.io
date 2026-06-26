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

  initScrollHeader();
});

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
