/* Progressive enhancement: all instructions, images, and links work without JS. */
(() => {
  document.querySelectorAll('.help-toggle').forEach(button => {
    button.addEventListener('click', () => {
      const figure = button.closest('.help-figure');
      const shown = button.getAttribute('aria-pressed') !== 'true';
      figure.querySelector('.help-highlights').hidden = !shown;
      button.setAttribute('aria-pressed', String(shown));
      button.textContent = shown ? 'Hide highlights' : 'Show highlights';
    });
  });
  const input = document.getElementById('help-search');
  if (!input) return;
  const status = document.getElementById('help-search-status');
  const results = document.getElementById('help-search-results');
  let indexPromise;
  let generation = 0;
  input.addEventListener('input', async () => {
    const current = ++generation;
    const words = input.value.toLocaleLowerCase().trim().split(/\s+/).filter(Boolean);
    results.replaceChildren();
    results.hidden = true;
    if (!words.length) { status.textContent = 'Search the manual and the visual control reference.'; return; }
    status.textContent = 'Searching…';
    try {
      indexPromise ||= fetch('search-index.json').then(response => {
        if (!response.ok) throw new Error('Search unavailable');
        return response.json();
      }).catch(error => { indexPromise = undefined; throw error; });
      const index = await indexPromise;
      if (generation !== current) return;
      const matches = index.map(entry => {
        const title = entry.title.toLocaleLowerCase();
        const text = `${title} ${entry.text.toLocaleLowerCase()}`;
        return {entry, score: words.every(word => text.includes(word)) ? 1 + words.filter(word => title.includes(word)).length * 3 : 0};
      }).filter(result => result.score).sort((a,b) => b.score-a.score);
      status.textContent = matches.length ? `${matches.length} matching topics${matches.length > 12 ? '; showing the first 12' : ''}.` : 'No matching topics. Try fewer words, a control name, or browse the sections below.';
      for (const {entry} of matches.slice(0, 12)) {
        const item = document.createElement('li');
        const link = document.createElement('a');
        link.href = entry.url;
        link.textContent = entry.title;
        const excerpt = document.createElement('small');
        const lower = entry.text.toLocaleLowerCase();
        const first = Math.max(0, lower.indexOf(words[0]) - 45);
        excerpt.textContent = (first ? '…' : '') + entry.text.slice(first, first + 190) + (entry.text.length > first + 190 ? '…' : '');
        item.append(link, excerpt);
        results.append(item);
      }
      results.hidden = matches.length === 0;
    } catch {
      if (generation === current) status.textContent = 'Search could not load. Use your browser’s Find command, section links, or the visual reference.';
    }
  });
})();
