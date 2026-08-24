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
