// Client-side interaction helpers (Vanilla JS, Zero Dependencies)

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

