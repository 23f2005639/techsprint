(function () {
  let elapsed = 0;
  const clock = seconds => `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
  window.PRESENTER_NOTES.forEach((note, index) => {
    const article = document.createElement('article');
    article.id = `notes-${index + 1}`;
    const title = document.createElement('h2'); title.textContent = `${index + 1}. ${note.title}`;
    const timing = document.createElement('div'); timing.className = 'timing'; timing.textContent = `${clock(elapsed)}–${clock(elapsed + note.duration)} · ${note.duration} seconds`;
    elapsed += note.duration;
    const cue = document.createElement('p'); cue.className = 'cue'; cue.textContent = note.cue;
    article.append(title, timing, cue);
    for (const text of note.paragraphs) { const p = document.createElement('p'); p.textContent = text; article.append(p); }
    const research = document.createElement('p'); research.className = 'research'; research.textContent = note.research; article.append(research);
    document.getElementById('script').append(article);
  });
  document.getElementById('print-notes').addEventListener('click', () => window.print());
})();
