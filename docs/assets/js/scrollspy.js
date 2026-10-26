(() => {
  function initialize() {
    const toc = document.querySelector('.sidebar__right .toc');
    const content = document.querySelector('.page__content');
    if (!toc || !content) return;
    const items = Array.from(toc.querySelectorAll('a[href*="#"]')).map(link => {
      const url = new URL(link.href, location.href);
      if (url.pathname !== location.pathname || !url.hash) return null;
      const heading = document.getElementById(decodeURIComponent(url.hash.slice(1)));
      return heading && content.contains(heading) ? {link, heading} : null;
    }).filter(Boolean);
    if (!items.length) return;
    const scrollbox = toc.querySelector('.toc__menu') || toc;
    let active = null;
    let queued = false;
    const topOffset = () => (document.querySelector('.masthead')?.getBoundingClientRect().height || 0) + 16;
    function refresh() {
      queued = false;
      const threshold = topOffset() + 2;
      let selected = items[0];
      for (const item of items) {
        if (item.heading.getBoundingClientRect().top <= threshold) selected = item;
        else break;
      }
      if (selected === active) return;
      active = selected;
      for (const item of items) {
        item.link.classList.toggle('is-active', item === active);
        if (item === active) item.link.setAttribute('aria-current','location');
        else item.link.removeAttribute('aria-current');
      }
      const a = active.link.getBoundingClientRect(), b = scrollbox.getBoundingClientRect();
      if (a.top < b.top || a.bottom > b.bottom) {
        scrollbox.scrollTo({top:scrollbox.scrollTop+a.top-b.top-b.height/2+a.height/2,behavior:'smooth'});
      }
    }
    function schedule() { if (!queued) { queued = true; requestAnimationFrame(refresh); } }
    toc.addEventListener('click', event => {
      const link = event.target.closest('a[href*="#"]');
      const item = items.find(entry => entry.link === link);
      if (!item) return;
      event.preventDefault();
      history.pushState(null, '', item.link.getAttribute('href'));
      const desiredTop = item.heading.getBoundingClientRect().top + window.scrollY - topOffset();
      window.scrollTo({top:Math.max(0,desiredTop),behavior:'smooth'});
      schedule();
    });
    addEventListener('scroll',schedule,{passive:true});
    addEventListener('resize',schedule,{passive:true});
    addEventListener('hashchange',schedule);
    document.querySelectorAll('.sidebar .nav__list a[href]').forEach(a => {
      const u = new URL(a.href, location.href);
      if (u.pathname.replace(/\/$/,'') === location.pathname.replace(/\/$/,'')) {
        a.classList.add('is-active-page');
        a.setAttribute('aria-current','page');
      }
    });
    refresh();
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded',initialize);
  else initialize();
})();
