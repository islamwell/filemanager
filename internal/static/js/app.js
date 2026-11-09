// Client-side interaction helpers (Vanilla JS, Zero Dependencies)

// Audio state persistence across navigations and page reloads
function saveAudioState() {
  const bar = document.getElementById('globalAudioBar');
  const player = document.getElementById('globalAudioPlayer');
  const trackName = document.getElementById('audioTrackName');
  if (!bar || !player) return;

  try {
    if (bar.classList.contains('active') && player.src) {
      const state = {
        src: player.src,
        filename: trackName ? trackName.textContent : 'Audio Track',
        currentTime: player.currentTime,
        paused: player.paused,
        active: true
      };
      sessionStorage.setItem('nrq_audio_player_state', JSON.stringify(state));
    }
  } catch (e) {
    console.warn('Unable to save audio state:', e);
  }
}

function initAudioPlayer() {
  const bar = document.getElementById('globalAudioBar');
  const player = document.getElementById('globalAudioPlayer');
  const trackName = document.getElementById('audioTrackName');
  if (!bar || !player) return;

  if (!player.hasAttribute('data-persisted')) {
    player.setAttribute('data-persisted', 'true');
    player.addEventListener('timeupdate', () => {
      if (!window._audioSaveTimeout) {
        window._audioSaveTimeout = setTimeout(() => {
          saveAudioState();
          window._audioSaveTimeout = null;
        }, 1000);
      }
    });
    player.addEventListener('play', saveAudioState);
    player.addEventListener('pause', saveAudioState);
    player.addEventListener('ended', saveAudioState);
  }

  try {
    const raw = sessionStorage.getItem('nrq_audio_player_state');
    if (!raw) return;
    const state = JSON.parse(raw);
    if (state && state.active && state.src) {
      if (trackName) trackName.textContent = state.filename || 'Audio Track';
      if (player.src !== state.src) {
        player.src = state.src;
        if (state.currentTime) {
          player.currentTime = state.currentTime;
        }
      }
      bar.classList.add('active');
      if (!state.paused) {
        player.play().catch(err => {
          console.log('Autoplay restriction caught, bottom player kept open ready to play:', err);
        });
      }
    }
  } catch (e) {
    console.warn('Unable to restore audio state:', e);
  }
}

// Single-Click MP3 Playback Controller
function playAudio(streamUrl, filename) {
  const bar = document.getElementById('globalAudioBar');
  const player = document.getElementById('globalAudioPlayer');
  const trackName = document.getElementById('audioTrackName');

  if (!bar || !player) return;

  trackName.textContent = filename || 'Audio Track';
  player.src = streamUrl;
  bar.classList.add('active');
  player.play().catch(err => {
    console.warn('Autoplay prevented by browser:', err);
  });
  saveAudioState();
}

function closeAudioPlayer() {
  const bar = document.getElementById('globalAudioBar');
  const player = document.getElementById('globalAudioPlayer');
  if (player) {
    player.pause();
    player.src = '';
  }
  if (bar) {
    bar.classList.remove('active');
  }
  try {
    sessionStorage.removeItem('nrq_audio_player_state');
  } catch (e) {}
}

// Modal Dialog Helpers
function openDialog(id) {
  const dlg = document.getElementById(id);
  if (dlg && typeof dlg.showModal === 'function') {
    dlg.showModal();
  }
}

function closeDialog(id) {
  const dlg = document.getElementById(id);
  if (dlg && typeof dlg.close === 'function') {
    dlg.close();
  }
}

// Share Modal Setup
function openShareModal(itemId, itemType, shareToken) {
  const dlg = document.getElementById('shareModal');
  const inputItemId = document.getElementById('shareItemId');
  const inputItemType = document.getElementById('shareItemType');
  const shareLinkSection = document.getElementById('existingShareSection');
  const shareLinkInput = document.getElementById('shareLinkInput');

  if (inputItemId) inputItemId.value = itemId;
  if (inputItemType) inputItemType.value = itemType;

  if (shareToken && shareToken !== "") {
    const fullUrl = window.location.origin + '/share/' + shareToken;
    if (shareLinkInput) shareLinkInput.value = fullUrl;
    if (shareLinkSection) shareLinkSection.style.display = 'block';
  } else {
    if (shareLinkSection) shareLinkSection.style.display = 'none';
  }

  openDialog('shareModal');
}

// Copy to Clipboard Utility
function copyToClipboard(text, btnElement) {
  if (!navigator.clipboard) {
    // Fallback
    const textArea = document.createElement("textarea");
    textArea.value = text;
    document.body.appendChild(textArea);
    textArea.select();
    try {
      document.execCommand('copy');
      indicateCopySuccess(btnElement);
    } catch (err) {
      console.error('Fallback copy failed', err);
    }
    document.body.removeChild(textArea);
    return;
  }

  navigator.clipboard.writeText(text).then(() => {
    indicateCopySuccess(btnElement);
  }).catch(err => {
    console.error('Copy failed:', err);
  });
}

function indicateCopySuccess(btn) {
  if (!btn) return;
  const originalText = btn.textContent;
  btn.textContent = 'Copied!';
  btn.classList.add('btn-primary');
  setTimeout(() => {
    btn.textContent = originalText;
    btn.classList.remove('btn-primary');
  }, 2000);
}

// Global Clipboard Paste Handler (Screenshots, Images, and Text)
window.addEventListener('paste', function(e) {
  const activeTag = document.activeElement ? document.activeElement.tagName.toLowerCase() : '';
  const isInputActive = activeTag === 'input' || activeTag === 'textarea' || activeTag === 'select';

  // Check for image blob in clipboard data
  if (e.clipboardData && e.clipboardData.items) {
    for (let i = 0; i < e.clipboardData.items.length; i++) {
      const item = e.clipboardData.items[i];
      if (item.type.indexOf('image') !== -1) {
        e.preventDefault();
        const file = item.getAsFile();
        if (!file) return;

        const pasteModal = document.getElementById('pasteImageModal');
        const preview = document.getElementById('pasteImagePreview');
        const meta = document.getElementById('pasteImageMeta');
        const filenameInput = document.getElementById('pastedFileName');
        const fileInput = document.getElementById('pastedFileInput');

        if (pasteModal && preview && fileInput) {
          const now = new Date();
          const timestamp = now.getFullYear() +
            String(now.getMonth() + 1).padStart(2, '0') +
            String(now.getDate()).padStart(2, '0') + '_' +
            String(now.getHours()).padStart(2, '0') +
            String(now.getMinutes()).padStart(2, '0') +
            String(now.getSeconds()).padStart(2, '0');

          const ext = file.type === 'image/jpeg' ? '.jpg' : file.type === 'image/webp' ? '.webp' : '.png';
          const defaultName = 'screenshot_' + timestamp + ext;

          if (filenameInput) filenameInput.value = defaultName;
          preview.src = URL.createObjectURL(file);
          if (meta) {
            const kbSize = (file.size / 1024).toFixed(1);
            meta.textContent = `Image Type: ${file.type} | Size: ${kbSize} KB`;
          }

          // Transfer blob to file input using DataTransfer
          try {
            const dataTransfer = new DataTransfer();
            const namedFile = new File([file], defaultName, { type: file.type });
            dataTransfer.items.add(namedFile);
            fileInput.files = dataTransfer.files;
          } catch (err) {
            console.warn('DataTransfer not supported in browser:', err);
          }

          openDialog('pasteImageModal');
          return;
        }
      }
    }
  }

  // Handle plain text paste when NOT actively focused in an input
  if (!isInputActive && e.clipboardData) {
    const text = e.clipboardData.getData('text');
    if (text && text.trim().length > 0 && window.location.pathname === '/') {
      const snippetEditor = document.getElementById('snippetContent');
      if (snippetEditor) {
        snippetEditor.value = text;
      }
    }
  }
});

// In-Browser Media Lightbox Preview (Images, PDFs, Videos)
function openMediaPreview(url, filename, mimeType) {
  const modal = document.getElementById('mediaPreviewModal');
  const titleEl = document.getElementById('mediaPreviewTitle');
  const bodyEl = document.getElementById('mediaPreviewBody');
  const dlBtn = document.getElementById('mediaDownloadBtn');

  if (!modal || !bodyEl) return;

  if (titleEl) titleEl.textContent = filename || 'Media Preview';
  if (dlBtn) {
    dlBtn.href = url.replace('/view', '/download');
    dlBtn.setAttribute('download', filename);
  }

  bodyEl.innerHTML = '';
  const lowerMime = (mimeType || '').toLowerCase();
  const lowerUrl = (url || '').toLowerCase();

  if (lowerMime.startsWith('image/') || lowerUrl.match(/\.(png|jpe?g|gif|webp|svg|ico|bmp)$/)) {
    const img = document.createElement('img');
    img.src = url;
    img.alt = filename;
    img.style.maxWidth = '100%';
    img.style.maxHeight = '70vh';
    img.style.borderRadius = '6px';
    img.style.boxShadow = '0 4px 16px rgba(0,0,0,0.15)';
    bodyEl.appendChild(img);
  } else if (lowerMime.startsWith('video/') || lowerUrl.match(/\.(mp4|webm|ogg|mov|m4v)$/)) {
    const video = document.createElement('video');
    video.src = url;
    video.controls = true;
    video.autoplay = true;
    video.style.width = '100%';
    video.style.maxHeight = '70vh';
    video.style.borderRadius = '6px';
    video.style.background = '#000';
    bodyEl.appendChild(video);
  } else if (lowerMime === 'application/pdf' || lowerUrl.endsWith('.pdf')) {
    const iframe = document.createElement('iframe');
    iframe.src = url;
    iframe.style.width = '100%';
    iframe.style.height = '70vh';
    iframe.style.border = 'none';
    iframe.style.borderRadius = '6px';
    bodyEl.appendChild(iframe);
  } else {
    const iframe = document.createElement('iframe');
    iframe.src = url;
    iframe.style.width = '100%';
    iframe.style.height = '70vh';
    iframe.style.border = 'none';
    bodyEl.appendChild(iframe);
  }

  openDialog('mediaPreviewModal');
}

function closeMediaPreview() {
  const bodyEl = document.getElementById('mediaPreviewBody');
  if (bodyEl) {
    const video = bodyEl.querySelector('video');
    if (video) video.pause();
    bodyEl.innerHTML = '';
  }
  closeDialog('mediaPreviewModal');
}

// Move Item Modal (Files and Snippets)
function openMoveModal(itemId, itemType, currentCatId, title) {
  const form = document.getElementById('moveItemForm');
  const titleEl = document.getElementById('moveModalTitle');
  const descEl = document.getElementById('moveItemDesc');
  const catSelect = document.getElementById('moveTargetCat');

  if (form) {
    form.action = itemType === 'file' ? ('/files/' + itemId + '/move') : ('/snippets/' + itemId + '/move');
  }
  if (titleEl) {
    titleEl.textContent = '📁 Move ' + (itemType === 'file' ? 'File' : 'Snippet');
  }
  if (descEl) {
    descEl.textContent = 'Moving: ' + (title || itemId);
  }
  if (catSelect) {
    catSelect.value = currentCatId || '';
  }

  openDialog('moveModal');
}

// Drag & Drop File Upload Handlers
let dragCounter = 0;
window.addEventListener('dragenter', function(e) {
  if (e.dataTransfer && e.dataTransfer.types && Array.from(e.dataTransfer.types).includes('Files')) {
    e.preventDefault();
    dragCounter++;
    const overlay = document.getElementById('dropOverlay');
    if (overlay) overlay.style.display = 'flex';
  }
});

window.addEventListener('dragleave', function(e) {
  dragCounter--;
  if (dragCounter <= 0) {
    dragCounter = 0;
    const overlay = document.getElementById('dropOverlay');
    if (overlay) overlay.style.display = 'none';
  }
});

window.addEventListener('dragover', function(e) {
  e.preventDefault();
});

window.addEventListener('drop', function(e) {
  dragCounter = 0;
  const overlay = document.getElementById('dropOverlay');
  if (overlay) overlay.style.display = 'none';

  if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files.length > 0) {
    e.preventDefault();
    const fileInput = document.getElementById('uploadFile');
    if (fileInput) {
      fileInput.files = e.dataTransfer.files;
      openDialog('uploadModal');
    }
  }
});

// Markdown Rendering & Tab Switching for Snippet View
function switchSnippetTab(mode) {
  const rawContainer = document.getElementById('snippetRawContainer');
  const mdContainer = document.getElementById('snippetMdContainer');
  const rawBtn = document.getElementById('tabRawBtn');
  const mdBtn = document.getElementById('tabMdBtn');

  if (!rawContainer || !mdContainer) return;

  if (mode === 'markdown') {
    rawContainer.style.display = 'none';
    mdContainer.style.display = 'block';
    if (rawBtn) { rawBtn.classList.remove('btn-primary'); rawBtn.classList.add('btn-secondary'); }
    if (mdBtn) { mdBtn.classList.remove('btn-secondary'); mdBtn.classList.add('btn-primary'); }

    const rawEl = document.getElementById('snippetRawCode');
    if (rawEl && !mdContainer.dataset.rendered) {
      mdContainer.innerHTML = renderMarkdown(rawEl.textContent);
      mdContainer.dataset.rendered = 'true';
    }
  } else {
    rawContainer.style.display = 'block';
    mdContainer.style.display = 'none';
    if (rawBtn) { rawBtn.classList.remove('btn-secondary'); rawBtn.classList.add('btn-primary'); }
    if (mdBtn) { mdBtn.classList.remove('btn-primary'); mdBtn.classList.add('btn-secondary'); }
  }
}

// Simple and safe client-side Markdown parser
function renderMarkdown(text) {
  if (!text) return '';

  let html = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');

  // Fenced code blocks ```lang\ncode\n```
  html = html.replace(/```([a-zA-Z0-9_-]*)\n([\s\S]*?)```/g, function(_, lang, code) {
    return `<pre class="md-code-block" data-lang="${lang}"><code>${code}</code></pre>`;
  });

  // Inline code `code`
  html = html.replace(/`([^`]+)`/g, '<code class="md-inline-code">$1</code>');

  // Headings
  html = html.replace(/^### (.*$)/gim, '<h3 class="md-h3">$1</h3>');
  html = html.replace(/^## (.*$)/gim, '<h2 class="md-h2">$1</h2>');
  html = html.replace(/^# (.*$)/gim, '<h1 class="md-h1">$1</h1>');

  // Blockquotes
  html = html.replace(/^\> (.*$)/gim, '<blockquote class="md-quote">$1</blockquote>');

  // Bold & Italic
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  html = html.replace(/\*([^*]+)\*/g, '<em>$1</em>');
  html = html.replace(/~~([^~]+)~~/g, '<del>$1</del>');

  // Unordered list items
  html = html.replace(/^\s*[\-\*] (.*$)/gim, '<li class="md-li">$1</li>');
  html = html.replace(/((?:<li class="md-li">.*<\/li>\s*)+)/g, '<ul class="md-ul">$1</ul>');

  // Paragraphs & Line Breaks
  html = html.replace(/\n\n/g, '<p style="margin: 0.75rem 0;"></p>');
  html = html.replace(/\n/g, '<br>');

  return html;
}

// -------------------------------------------------------------
// Dark / Light Theme Switching
// -------------------------------------------------------------
function toggleTheme() {
  const current = document.documentElement.getAttribute('data-theme') || 'light';
  const next = current === 'dark' ? 'light' : 'dark';
  document.documentElement.setAttribute('data-theme', next);
  localStorage.setItem('fm_theme', next);
}

// -------------------------------------------------------------
// Multi-Select & Batch Actions
// -------------------------------------------------------------
function toggleSelectAllFiles(master) {
  const checkboxes = document.querySelectorAll('.file-select-checkbox');
  checkboxes.forEach(cb => cb.checked = master.checked);
  updateBatchBar();
}

function onFileSelectionChange() {
  const checkboxes = document.querySelectorAll('.file-select-checkbox');
  const master = document.getElementById('selectAllFiles');
  const checked = document.querySelectorAll('.file-select-checkbox:checked');
  if (master) {
    master.checked = checkboxes.length > 0 && checked.length === checkboxes.length;
    master.indeterminate = checked.length > 0 && checked.length < checkboxes.length;
  }
  updateBatchBar();
}

function toggleSelectAllSnippets(master) {
  const checkboxes = document.querySelectorAll('.snippet-select-checkbox');
  checkboxes.forEach(cb => cb.checked = master.checked);
}

function onSnippetSelectionChange() {
  const checkboxes = document.querySelectorAll('.snippet-select-checkbox');
  const master = document.getElementById('selectAllSnippets');
  const checked = document.querySelectorAll('.snippet-select-checkbox:checked');
  if (master) {
    master.checked = checkboxes.length > 0 && checked.length === checkboxes.length;
    master.indeterminate = checked.length > 0 && checked.length < checkboxes.length;
  }
}

function getSelectedFileIDs() {
  const checked = document.querySelectorAll('.file-select-checkbox:checked');
  return Array.from(checked).map(cb => cb.value);
}

function updateBatchBar() {
  const bar = document.getElementById('batchActionBar');
  const countBadge = document.getElementById('batchCountBadge');
  const countText = document.getElementById('batchCountText');
  if (!bar) return;

  const selected = getSelectedFileIDs();
  if (selected.length > 0) {
    bar.style.display = 'flex';
    if (countBadge) countBadge.textContent = selected.length;
    if (countText) countText.textContent = selected.length === 1 ? 'file selected' : 'files selected';
  } else {
    bar.style.display = 'none';
  }
}

function clearBatchSelection() {
  document.querySelectorAll('.file-select-checkbox, .snippet-select-checkbox').forEach(cb => cb.checked = false);
  const masterFiles = document.getElementById('selectAllFiles');
  const masterSnippets = document.getElementById('selectAllSnippets');
  if (masterFiles) { masterFiles.checked = false; masterFiles.indeterminate = false; }
  if (masterSnippets) { masterSnippets.checked = false; masterSnippets.indeterminate = false; }
  const bar = document.getElementById('batchActionBar');
  if (bar) bar.style.display = 'none';
}

function submitBatchZip() {
  const ids = getSelectedFileIDs();
  if (ids.length === 0) return;

  const form = document.createElement('form');
  form.method = 'POST';
  form.action = '/files/batch/zip';

  const csrfInput = document.querySelector('input[name="_csrf"]');
  if (csrfInput) {
    const hiddenCSRF = document.createElement('input');
    hiddenCSRF.type = 'hidden';
    hiddenCSRF.name = '_csrf';
    hiddenCSRF.value = csrfInput.value;
    form.appendChild(hiddenCSRF);
  }

  ids.forEach(id => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = 'file_ids';
    input.value = id;
    form.appendChild(input);
  });

  document.body.appendChild(form);
  form.submit();
  document.body.removeChild(form);
}

function submitBatchDelete() {
  const ids = getSelectedFileIDs();
  if (ids.length === 0) return;

  if (!confirm(`Are you sure you want to delete ${ids.length} selected files?`)) {
    return;
  }

  const form = document.createElement('form');
  form.method = 'POST';
  form.action = '/files/batch/delete';

  const csrfInput = document.querySelector('input[name="_csrf"]');
  if (csrfInput) {
    const hiddenCSRF = document.createElement('input');
    hiddenCSRF.type = 'hidden';
    hiddenCSRF.name = '_csrf';
    hiddenCSRF.value = csrfInput.value;
    form.appendChild(hiddenCSRF);
  }

  ids.forEach(id => {
    const input = document.createElement('input');
    input.type = 'hidden';
    input.name = 'file_ids';
    input.value = id;
    form.appendChild(input);
  });

  document.body.appendChild(form);
  form.submit();
}

function openBatchMoveModal() {
  const ids = getSelectedFileIDs();
  if (ids.length === 0) return;

  const hiddenIDs = document.getElementById('batchMoveFileIDs');
  if (hiddenIDs) {
    hiddenIDs.value = ids.join(',');
  }
  openDialog('batchMoveModal');
}

// -------------------------------------------------------------
// Inline Rename Modal
// -------------------------------------------------------------
function openRenameModal(id, itemType, currentName) {
  const dlg = document.getElementById('renameModal');
  const form = document.getElementById('renameForm');
  const input = document.getElementById('renameInput');
  const title = document.getElementById('renameModalTitle');
  const label = document.getElementById('renameInputLabel');

  if (!dlg || !form || !input) return;

  if (itemType === 'file') {
    form.action = '/files/' + id + '/rename';
    input.name = 'filename';
    input.value = currentName;
    if (title) title.textContent = '✏️ Rename File';
    if (label) label.textContent = 'New Filename';
  } else {
    form.action = '/snippets/' + id + '/rename';
    input.name = 'title';
    input.value = currentName;
    if (title) title.textContent = '✏️ Rename Snippet';
    if (label) label.textContent = 'New Title';
  }

  openDialog('renameModal');
}

// -------------------------------------------------------------
// Offline Mode & PWA Helpers
// -------------------------------------------------------------
function updateNetworkStatus() {
  const banner = document.getElementById('offlineNotice');
  if (!banner) return;
  if (!navigator.onLine) {
    banner.style.display = 'flex';
  } else {
    banner.style.display = 'none';
  }
}

window.addEventListener('online', updateNetworkStatus);
window.addEventListener('offline', updateNetworkStatus);
document.addEventListener('DOMContentLoaded', updateNetworkStatus);

// Automatically cache viewed snippets in localStorage for offline access
function cacheCurrentSnippetIfPresent() {
  const codeEl = document.getElementById('snippetRawCode');
  if (!codeEl) return;

  const titleEl = document.querySelector('.card-section h2');
  const langEl = document.querySelector('.card-section span strong');

  const title = titleEl ? titleEl.textContent.trim() : 'Untitled Snippet';
  const language = langEl ? langEl.textContent.trim() : 'text';
  const content = codeEl.textContent;

  try {
    let snippets = [];
    const raw = localStorage.getItem('fm_cached_snippets');
    if (raw) {
      snippets = JSON.parse(raw);
    }
    snippets = snippets.filter(s => s.title !== title);
    snippets.unshift({
      title: title,
      language: language,
      content: content,
      cachedAt: new Date().toISOString()
    });
    if (snippets.length > 50) {
      snippets = snippets.slice(0, 50);
    }
    localStorage.setItem('fm_cached_snippets', JSON.stringify(snippets));
  } catch(e) {
    console.warn('Could not cache snippet for offline access:', e);
  }
}

document.addEventListener('DOMContentLoaded', cacheCurrentSnippetIfPresent);

// -------------------------------------------------------------
// Seamless Folder Navigation & Upload Progress Handlers
// -------------------------------------------------------------
function formatBytes(bytes, decimals = 1) {
  if (!bytes || bytes === 0) return '0 B';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

// Seamless Folder Navigation (Keeps audio player playing continuously across folders)
function seamlessNavigate(url, push = true) {
  const dashboardGrid = document.getElementById('dashboardGrid');
  if (!dashboardGrid) {
    window.location.href = url;
    return;
  }

  dashboardGrid.style.transition = 'opacity 0.15s ease';
  dashboardGrid.style.opacity = '0.5';

  fetch(url, { headers: { 'X-Requested-With': 'XMLHttpRequest' } })
    .then(res => {
      if (!res.ok) throw new Error('HTTP ' + res.status);
      return res.text();
    })
    .then(html => {
      const parser = new DOMParser();
      const doc = parser.parseFromString(html, 'text/html');
      const newGrid = doc.getElementById('dashboardGrid');

      if (newGrid) {
        dashboardGrid.innerHTML = newGrid.innerHTML;
        dashboardGrid.className = newGrid.className;
        document.title = doc.title;
        if (push) {
          history.pushState(null, '', url);
        }
        initUploadForms();
        if (typeof onFileSelectionChange === 'function') {
          onFileSelectionChange();
        }
      } else {
        window.location.href = url;
      }
    })
    .catch(err => {
      console.warn('Seamless navigation failed, falling back:', err);
      window.location.href = url;
    })
    .finally(() => {
      if (dashboardGrid) {
        dashboardGrid.style.opacity = '1';
      }
    });
}

function setupSeamlessFolderNavigation() {
  document.addEventListener('click', function(e) {
    const link = e.target.closest('a');
    if (!link || !link.href) return;

    // Ignore middle-click, modifier keys, target="_blank", or download links
    if (e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey) return;
    if (link.target === '_blank' || link.hasAttribute('download')) return;

    try {
      const url = new URL(link.href, window.location.origin);
      // Intercept navigation to root dashboard or folder/tag queries
      if (url.origin === window.location.origin && url.pathname === '/') {
        if (document.getElementById('dashboardGrid')) {
          e.preventDefault();
          seamlessNavigate(url.href, true);
        }
      }
    } catch (err) {}
  });

  window.addEventListener('popstate', function() {
    if (document.getElementById('dashboardGrid')) {
      seamlessNavigate(window.location.href, false);
    }
  });
}

// Upload with Live Progress Bar (MWG-compliant)
function uploadFormWithProgress(form, options) {
  const fileInput = form.querySelector('input[type="file"]');
  if (!fileInput || !fileInput.files || fileInput.files.length === 0) {
    alert('Please select a file to upload.');
    return;
  }

  const file = fileInput.files[0];
  const progressSection = options.progressSection;
  const progressBar = options.progressBar;
  const percentEl = options.percentEl;
  const detailEl = options.detailEl;
  const submitBtn = form.querySelector('button[type="submit"]');
  const cancelBtn = form.querySelector('button[type="button"]');

  const globalToast = document.getElementById('globalUploadToast');
  const globalBar = document.getElementById('globalUploadProgressBar');
  const globalFilename = document.getElementById('globalUploadFilename');
  const globalPercent = document.getElementById('globalUploadPercent');
  const globalDetail = document.getElementById('globalUploadDetail');

  if (globalToast) {
    if (globalFilename) globalFilename.textContent = file.name;
    if (globalBar) globalBar.value = 0;
    if (globalPercent) globalPercent.textContent = '0%';
    if (globalDetail) globalDetail.textContent = '0 B / ' + formatBytes(file.size);
    globalToast.style.display = 'flex';
  }

  if (progressSection) progressSection.style.display = 'flex';
  if (progressBar) progressBar.value = 0;
  if (percentEl) percentEl.textContent = '0%';
  if (detailEl) detailEl.textContent = '0 B / ' + formatBytes(file.size);

  if (submitBtn) {
    submitBtn.disabled = true;
    submitBtn.dataset.originalText = submitBtn.textContent;
    submitBtn.textContent = 'Uploading...';
  }
  if (cancelBtn) cancelBtn.disabled = true;

  const xhr = new XMLHttpRequest();
  xhr.open('POST', form.action, true);
  xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');

  xhr.upload.onprogress = function(e) {
    if (e.lengthComputable) {
      const percent = Math.min(99, Math.round((e.loaded / e.total) * 100));
      if (progressBar) progressBar.value = percent;
      if (percentEl) percentEl.textContent = percent + '%';
      if (detailEl) {
        detailEl.textContent = formatBytes(e.loaded) + ' of ' + formatBytes(e.total) + ' (' + percent + '%)';
      }

      if (globalBar) globalBar.value = percent;
      if (globalPercent) globalPercent.textContent = percent + '%';
      if (globalDetail) {
        globalDetail.textContent = formatBytes(e.loaded) + ' of ' + formatBytes(e.total);
      }
    }
  };

  xhr.onload = function() {
    if (progressBar) progressBar.value = 100;
    if (globalBar) globalBar.value = 100;
    if (percentEl) percentEl.textContent = '100%';
    if (globalPercent) globalPercent.textContent = '100%';

    let isSuccess = xhr.status >= 200 && xhr.status < 400;
    let msg = 'File uploaded successfully!';

    try {
      const data = JSON.parse(xhr.responseText);
      if (data && data.success === false) {
        isSuccess = false;
        msg = data.error || 'Upload failed.';
      } else if (data && data.message) {
        msg = data.message;
      }
    } catch (e) {
      if (!isSuccess) {
        msg = 'Upload failed with status ' + xhr.status;
      }
    }

    if (isSuccess) {
      if (detailEl) detailEl.textContent = '✓ Upload complete!';
      if (globalDetail) globalDetail.textContent = '✓ Complete!';
      if (submitBtn) submitBtn.textContent = 'Uploaded!';

      setTimeout(() => {
        if (options.modalId) {
          closeDialog(options.modalId);
        }
        if (progressSection) progressSection.style.display = 'none';
        if (submitBtn) {
          submitBtn.disabled = false;
          submitBtn.textContent = submitBtn.dataset.originalText || 'Upload File';
        }
        if (cancelBtn) cancelBtn.disabled = false;
        form.reset();

        // Refresh dashboard content without interrupting audio playback
        if (document.getElementById('dashboardGrid')) {
          seamlessNavigate(window.location.href, false);
        } else {
          window.location.reload();
        }

        setTimeout(() => {
          if (globalToast) globalToast.style.display = 'none';
        }, 3000);
      }, 600);
    } else {
      alert('Upload failed: ' + msg);
      if (detailEl) detailEl.textContent = 'Error: ' + msg;
      if (globalToast) globalToast.style.display = 'none';
      if (submitBtn) {
        submitBtn.disabled = false;
        submitBtn.textContent = submitBtn.dataset.originalText || 'Upload File';
      }
      if (cancelBtn) cancelBtn.disabled = false;
    }
  };

  xhr.onerror = function() {
    alert('Network error occurred during upload. Please check your connection.');
    if (globalToast) globalToast.style.display = 'none';
    if (submitBtn) {
      submitBtn.disabled = false;
      submitBtn.textContent = submitBtn.dataset.originalText || 'Upload File';
    }
    if (cancelBtn) cancelBtn.disabled = false;
  };

  const formData = new FormData(form);
  xhr.send(formData);
}

function initUploadForms() {
  const uploadForm = document.getElementById('uploadFileForm');
  if (uploadForm && !uploadForm.hasAttribute('data-ajax-init')) {
    uploadForm.setAttribute('data-ajax-init', 'true');
    uploadForm.addEventListener('submit', function(e) {
      e.preventDefault();
      uploadFormWithProgress(uploadForm, {
        modalId: 'uploadModal',
        progressSection: document.getElementById('uploadProgressSection'),
        progressBar: document.getElementById('uploadProgressBar'),
        percentEl: document.getElementById('uploadProgressPercent'),
        detailEl: document.getElementById('uploadProgressDetail')
      });
    });
  }

  const pasteForm = document.getElementById('pasteUploadForm');
  if (pasteForm && !pasteForm.hasAttribute('data-ajax-init')) {
    pasteForm.setAttribute('data-ajax-init', 'true');
    pasteForm.addEventListener('submit', function(e) {
      e.preventDefault();
      uploadFormWithProgress(pasteForm, {
        modalId: 'pasteImageModal',
        progressSection: document.getElementById('pasteProgressSection'),
        progressBar: document.getElementById('pasteProgressBar'),
        percentEl: document.getElementById('pasteProgressPercent'),
        detailEl: document.getElementById('pasteProgressDetail')
      });
    });
  }
}

// Lifecycle boot
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', () => {
    initAudioPlayer();
    setupSeamlessFolderNavigation();
    initUploadForms();
  });
} else {
  initAudioPlayer();
  setupSeamlessFolderNavigation();
  initUploadForms();
}



